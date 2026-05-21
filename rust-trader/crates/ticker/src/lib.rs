//! go-trader-ticker: Market data polling and basket management.
pub mod basket;

use chrono::{DateTime, Utc};
use futures_util::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};
use tokio::task::JoinSet;
use tokio_tungstenite::{connect_async, tungstenite::Message};
use tracing::{debug, info, warn};

const DEFAULT_MARKET_DATA_POLL_SECS: u64 = 15;
const MIN_MARKET_DATA_POLL_SECS: u64 = 5;
const ALPACA_MARKET_DATA_STREAM_URL: &str = "wss://stream.data.alpaca.markets/v2/iex";
const STREAM_RECONNECT_SECS: u64 = 5;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Quote {
    pub bid_price: f64,
    pub bid_size: u32,
    pub ask_price: f64,
    pub ask_size: u32,
    pub timestamp: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub price: f64,
    pub size: u32,
    pub timestamp: String,
    pub exchange: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Bar {
    pub close: f64,
    pub high: f64,
    pub low: f64,
    pub open: f64,
    pub volume: u64,
    pub timestamp: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TickerData {
    pub symbol: String,
    pub trade: Option<Trade>,
    pub quote: Option<Quote>,
    pub bar: Option<Bar>,
    pub last_updated: DateTime<Utc>,
}

pub trait DataHandler: Send + Sync {
    fn handle(&self, symbol: &str, data: &TickerData);
}

pub struct TickerServer {
    api_key: String,
    api_secret: String,
    symbols: Arc<RwLock<Vec<String>>>,
    last_data: Arc<RwLock<HashMap<String, TickerData>>>,
    handler: Arc<RwLock<Option<Box<dyn DataHandler>>>>,
    mock_mode: bool,
    running: Arc<RwLock<bool>>,
    http: reqwest::Client,
    poll_interval: Duration,
}

impl TickerServer {
    pub fn new(_is_paper: bool, api_key: impl Into<String>, api_secret: impl Into<String>) -> Self {
        let key = api_key.into();
        let mock = key == "MOCK_ALPACA_API_KEY"
            || std::env::var("GO_TRADER_MOCK").unwrap_or_default() == "true";
        Self {
            api_key: key,
            api_secret: api_secret.into(),
            symbols: Arc::new(RwLock::new(Vec::new())),
            last_data: Arc::new(RwLock::new(HashMap::new())),
            handler: Arc::new(RwLock::new(None)),
            mock_mode: mock,
            running: Arc::new(RwLock::new(false)),
            http: reqwest::Client::new(),
            poll_interval: market_data_poll_interval(),
        }
    }

    pub async fn start(&self) {
        *self.running.write().unwrap() = true;
        info!(
            mock = self.mock_mode,
            poll_interval_secs = self.poll_interval.as_secs(),
            stream_enabled = market_data_stream_enabled(),
            "TickerServer started"
        );
        self.update_market_data().await;
        if !self.mock_mode && market_data_stream_enabled() {
            self.spawn_streamer();
        }
        self.spawn_poller();
    }

    fn spawn_streamer(&self) {
        let symbols = self.symbols.clone();
        let last_data = self.last_data.clone();
        let handler = self.handler.clone();
        let running = self.running.clone();
        let api_key = self.api_key.clone();
        let api_secret = self.api_secret.clone();
        tokio::spawn(async move {
            loop {
                if !*running.read().unwrap() {
                    break;
                }
                let syms = symbols.read().unwrap().clone();
                if syms.is_empty() {
                    tokio::time::sleep(Duration::from_secs(STREAM_RECONNECT_SECS)).await;
                    continue;
                }
                match alpaca_stream_market_data(
                    &api_key,
                    &api_secret,
                    &syms,
                    &last_data,
                    &handler,
                    &running,
                )
                .await
                {
                    Ok(()) => info!("Alpaca market data stream ended"),
                    Err(e) => warn!("ALPACA_MARKET_DATA_STREAM_FAILED: {}", e),
                }
                tokio::time::sleep(Duration::from_secs(STREAM_RECONNECT_SECS)).await;
            }
        });
    }

    fn spawn_poller(&self) {
        let symbols = self.symbols.clone();
        let last_data = self.last_data.clone();
        let handler = self.handler.clone();
        let running = self.running.clone();
        let http = self.http.clone();
        let api_key = self.api_key.clone();
        let api_secret = self.api_secret.clone();
        let mock_mode = self.mock_mode;
        let poll_interval = self.poll_interval;
        tokio::spawn(async move {
            let mut interval = tokio::time::interval(poll_interval);
            interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);
            loop {
                interval.tick().await;
                if !*running.read().unwrap() {
                    break;
                }
                let syms = symbols.read().unwrap().clone();
                if syms.is_empty() {
                    continue;
                }
                if mock_mode {
                    mock_update(&syms, &last_data, &handler);
                } else {
                    if let Err(e) =
                        alpaca_fetch(&http, &api_key, &api_secret, &syms, &last_data, &handler)
                            .await
                    {
                        warn!("ALPACA_MARKET_DATA_FETCH_FAILED: {}", e);
                    }
                }
            }
        });
    }

    pub fn stop(&self) {
        *self.running.write().unwrap() = false;
    }
    pub fn update_symbols(&self, s: Vec<String>) {
        *self.symbols.write().unwrap() = s;
    }
    pub fn get_symbols(&self) -> Vec<String> {
        self.symbols.read().unwrap().clone()
    }
    pub fn set_data_handler(&self, h: Box<dyn DataHandler>) {
        *self.handler.write().unwrap() = Some(h);
    }
    pub fn get_last_data(&self, sym: &str) -> Option<TickerData> {
        self.last_data.read().unwrap().get(sym).cloned()
    }
    pub fn get_all_last_data(&self) -> HashMap<String, TickerData> {
        self.last_data.read().unwrap().clone()
    }

    async fn update_market_data(&self) {
        let syms = self.symbols.read().unwrap().clone();
        if syms.is_empty() {
            return;
        }
        if self.mock_mode {
            mock_update(&syms, &self.last_data, &self.handler);
        } else if let Err(e) = alpaca_fetch(
            &self.http,
            &self.api_key,
            &self.api_secret,
            &syms,
            &self.last_data,
            &self.handler,
        )
        .await
        {
            warn!("ALPACA_MARKET_DATA_FETCH_FAILED: {}", e);
        }
    }
}

fn mock_update(
    symbols: &[String],
    last_data: &Arc<RwLock<HashMap<String, TickerData>>>,
    handler: &Arc<RwLock<Option<Box<dyn DataHandler>>>>,
) {
    let now = Utc::now();
    for (i, sym) in symbols.iter().enumerate() {
        let base = 100.0 + i as f64 * 25.0;
        let wave = ((now.timestamp() % 3600) as f64 / 180.0 + i as f64).sin() * 2.5;
        let price = base + wave;
        let data = TickerData {
            symbol: sym.clone(),
            trade: Some(Trade {
                price,
                size: 100 + i as u32,
                timestamp: now.to_rfc3339(),
                exchange: "MOCK".into(),
            }),
            quote: Some(Quote {
                bid_price: price - 0.01,
                bid_size: 100,
                ask_price: price + 0.01,
                ask_size: 100,
                timestamp: now.to_rfc3339(),
            }),
            bar: None,
            last_updated: now,
        };
        last_data.write().unwrap().insert(sym.clone(), data.clone());
        if let Some(ref h) = *handler.read().unwrap() {
            h.handle(sym, &data);
        }
    }
}

async fn alpaca_fetch(
    http: &reqwest::Client,
    api_key: &str,
    api_secret: &str,
    symbols: &[String],
    last_data: &Arc<RwLock<HashMap<String, TickerData>>>,
    handler: &Arc<RwLock<Option<Box<dyn DataHandler>>>>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let started = Instant::now();
    let mut fetches = JoinSet::new();
    for sym in symbols.iter().cloned() {
        let http = http.clone();
        let api_key = api_key.to_string();
        let api_secret = api_secret.to_string();
        fetches.spawn(async move {
            let result = fetch_snapshot(&http, &api_key, &api_secret, &sym).await;
            (sym, result)
        });
    }

    while let Some(joined) = fetches.join_next().await {
        let (sym, result) = joined?;
        let (trade, quote, bar) = result?;
        let data = TickerData {
            symbol: sym.clone(),
            trade: Some(trade),
            quote: Some(quote),
            bar,
            last_updated: Utc::now(),
        };
        last_data.write().unwrap().insert(sym.clone(), data.clone());
        if let Some(ref h) = *handler.read().unwrap() {
            h.handle(&sym, &data);
        }
    }
    info!(
        symbols = symbols.len(),
        latency_ms = started.elapsed().as_millis(),
        "Alpaca market data snapshots refreshed"
    );
    Ok(())
}

fn market_data_poll_interval() -> Duration {
    let secs = std::env::var("GO_TRADER_MARKET_DATA_INTERVAL_SECS")
        .ok()
        .and_then(|raw| raw.parse::<u64>().ok())
        .unwrap_or(DEFAULT_MARKET_DATA_POLL_SECS)
        .max(MIN_MARKET_DATA_POLL_SECS);
    Duration::from_secs(secs)
}

fn market_data_stream_enabled() -> bool {
    std::env::var("GO_TRADER_MARKET_DATA_STREAM_ENABLED")
        .map(|raw| {
            !matches!(
                raw.to_ascii_lowercase().as_str(),
                "0" | "false" | "no" | "off"
            )
        })
        .unwrap_or(true)
}

async fn alpaca_stream_market_data(
    api_key: &str,
    api_secret: &str,
    symbols: &[String],
    last_data: &Arc<RwLock<HashMap<String, TickerData>>>,
    handler: &Arc<RwLock<Option<Box<dyn DataHandler>>>>,
    running: &Arc<RwLock<bool>>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    info!(
        url = ALPACA_MARKET_DATA_STREAM_URL,
        symbols = symbols.len(),
        "Connecting Alpaca market data stream"
    );
    let (mut ws, _) = connect_async(ALPACA_MARKET_DATA_STREAM_URL).await?;
    ws.send(Message::Text(
        serde_json::json!({"action":"auth","key":api_key,"secret":api_secret})
            .to_string()
            .into(),
    ))
    .await?;
    ws.send(Message::Text(
        serde_json::json!({"action":"subscribe","trades":symbols,"quotes":symbols,"bars":symbols})
            .to_string()
            .into(),
    ))
    .await?;

    while *running.read().unwrap() {
        let Some(msg) = ws.next().await else { break };
        match msg? {
            Message::Text(text) => handle_stream_text(&text, last_data, handler),
            Message::Binary(bytes) => {
                if let Ok(text) = String::from_utf8(bytes.to_vec()) {
                    handle_stream_text(&text, last_data, handler);
                }
            }
            Message::Ping(payload) => ws.send(Message::Pong(payload)).await?,
            Message::Close(frame) => {
                warn!("Alpaca market data stream closed: {:?}", frame);
                break;
            }
            _ => {}
        }
    }
    Ok(())
}

fn handle_stream_text(
    text: &str,
    last_data: &Arc<RwLock<HashMap<String, TickerData>>>,
    handler: &Arc<RwLock<Option<Box<dyn DataHandler>>>>,
) {
    let Ok(value) = serde_json::from_str::<serde_json::Value>(text) else {
        warn!("Invalid Alpaca market stream JSON: {}", text);
        return;
    };
    let events: Vec<serde_json::Value> = match value {
        serde_json::Value::Array(items) => items,
        other => vec![other],
    };
    for event in events {
        let Some(kind) = event.get("T").and_then(|v| v.as_str()) else {
            continue;
        };
        if matches!(kind, "success" | "subscription" | "error") {
            if kind == "error" {
                warn!("Alpaca market data stream control event: {}", event);
            } else {
                debug!("Alpaca market data stream control event: {}", event);
            }
            continue;
        }
        let Some(symbol) = event.get("S").and_then(|v| v.as_str()).map(str::to_string) else {
            continue;
        };
        let mut data = last_data
            .read()
            .unwrap()
            .get(&symbol)
            .cloned()
            .unwrap_or_else(|| TickerData {
                symbol: symbol.clone(),
                trade: None,
                quote: None,
                bar: None,
                last_updated: Utc::now(),
            });
        match kind {
            "t" => data.trade = parse_stream_trade(&event),
            "q" => data.quote = parse_stream_quote(&event),
            "b" => data.bar = parse_stream_bar(&event),
            _ => continue,
        }
        data.last_updated = Utc::now();
        last_data
            .write()
            .unwrap()
            .insert(symbol.clone(), data.clone());
        if let Some(ref h) = *handler.read().unwrap() {
            h.handle(&symbol, &data);
        }
    }
}

async fn fetch_snapshot(
    http: &reqwest::Client,
    api_key: &str,
    api_secret: &str,
    sym: &str,
) -> Result<(Trade, Quote, Option<Bar>), Box<dyn std::error::Error + Send + Sync>> {
    let resp = http
        .get(format!(
            "https://data.alpaca.markets/v2/stocks/{}/snapshot",
            sym
        ))
        .header("APCA-API-KEY-ID", api_key)
        .header("APCA-API-SECRET-KEY", api_secret)
        .send()
        .await?;
    if !resp.status().is_success() {
        let status = resp.status();
        let body = resp.text().await.unwrap_or_default();
        return Err(format!("snapshot {} returned {}: {}", sym, status, body).into());
    }
    let snapshot = resp.json::<serde_json::Value>().await?;
    let trade = parse_snapshot_trade(sym, &snapshot)?;
    let quote = parse_snapshot_quote(sym, &snapshot)?;
    let bar = parse_snapshot_bar(&snapshot, "dailyBar")
        .or_else(|_| parse_snapshot_bar(&snapshot, "minuteBar"))
        .ok();
    Ok((trade, quote, bar))
}

fn parse_stream_trade(event: &serde_json::Value) -> Option<Trade> {
    Some(Trade {
        price: event.get("p")?.as_f64()?,
        size: u32::try_from(event.get("s")?.as_u64()?).ok()?,
        timestamp: event.get("t")?.as_str()?.to_string(),
        exchange: event
            .get("x")
            .and_then(|v| v.as_str())
            .unwrap_or("IEX")
            .to_string(),
    })
}

fn parse_stream_quote(event: &serde_json::Value) -> Option<Quote> {
    Some(Quote {
        bid_price: event.get("bp")?.as_f64()?,
        bid_size: u32::try_from(event.get("bs")?.as_u64()?).ok()?,
        ask_price: event.get("ap")?.as_f64()?,
        ask_size: u32::try_from(event.get("as")?.as_u64()?).ok()?,
        timestamp: event.get("t")?.as_str()?.to_string(),
    })
}

fn parse_stream_bar(event: &serde_json::Value) -> Option<Bar> {
    Some(Bar {
        close: event.get("c")?.as_f64()?,
        high: event.get("h")?.as_f64()?,
        low: event.get("l")?.as_f64()?,
        open: event.get("o")?.as_f64()?,
        volume: event.get("v")?.as_u64()?,
        timestamp: event.get("t")?.as_str()?.to_string(),
    })
}

fn parse_snapshot_trade(
    symbol: &str,
    payload: &serde_json::Value,
) -> Result<Trade, Box<dyn std::error::Error + Send + Sync>> {
    let trade = payload.get("latestTrade").ok_or_else(|| {
        format!(
            "snapshot {} missing latestTrade object: {}",
            symbol, payload
        )
    })?;
    Ok(Trade {
        price: number_field(trade, "p")?,
        size: u32_field(trade, "s")?,
        timestamp: string_field(trade, "t")?,
        exchange: string_field(trade, "x")?,
    })
}

fn parse_snapshot_quote(
    symbol: &str,
    payload: &serde_json::Value,
) -> Result<Quote, Box<dyn std::error::Error + Send + Sync>> {
    let quote = payload.get("latestQuote").ok_or_else(|| {
        format!(
            "snapshot {} missing latestQuote object: {}",
            symbol, payload
        )
    })?;
    Ok(Quote {
        bid_price: number_field(quote, "bp")?,
        bid_size: u32_field(quote, "bs")?,
        ask_price: number_field(quote, "ap")?,
        ask_size: u32_field(quote, "as")?,
        timestamp: string_field(quote, "t")?,
    })
}

fn parse_snapshot_bar(
    payload: &serde_json::Value,
    key: &str,
) -> Result<Bar, Box<dyn std::error::Error + Send + Sync>> {
    let bar = payload
        .get(key)
        .ok_or_else(|| format!("snapshot missing {} object: {}", key, payload))?;
    Ok(Bar {
        close: number_field(bar, "c")?,
        high: number_field(bar, "h")?,
        low: number_field(bar, "l")?,
        open: number_field(bar, "o")?,
        volume: u64_field(bar, "v")?,
        timestamp: string_field(bar, "t")?,
    })
}

pub fn change_from_snapshot(data: &TickerData) -> f64 {
    let Some(trade) = data.trade.as_ref() else {
        return 0.0;
    };
    let Some(bar) = data.bar.as_ref() else {
        return 0.0;
    };
    if bar.open > 0.0 {
        (trade.price - bar.open) / bar.open
    } else {
        0.0
    }
}
fn number_field(
    obj: &serde_json::Value,
    key: &str,
) -> Result<f64, Box<dyn std::error::Error + Send + Sync>> {
    obj.get(key)
        .and_then(|v| v.as_f64())
        .ok_or_else(|| format!("missing numeric Alpaca field `{}` in {}", key, obj).into())
}

fn u32_field(
    obj: &serde_json::Value,
    key: &str,
) -> Result<u32, Box<dyn std::error::Error + Send + Sync>> {
    let value = obj
        .get(key)
        .ok_or_else(|| format!("missing unsigned integer Alpaca field `{}` in {}", key, obj))?;
    let n = value.as_u64().ok_or_else(|| {
        format!(
            "Alpaca field `{}` must be an unsigned integer in {}",
            key, obj
        )
    })?;
    u32::try_from(n).map_err(|_| format!("Alpaca field `{}` overflows u32: {}", key, n).into())
}

fn u64_field(
    obj: &serde_json::Value,
    key: &str,
) -> Result<u64, Box<dyn std::error::Error + Send + Sync>> {
    obj.get(key)
        .and_then(|v| v.as_u64())
        .ok_or_else(|| format!("missing unsigned integer Alpaca field `{}` in {}", key, obj).into())
}

fn string_field(
    obj: &serde_json::Value,
    key: &str,
) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
    obj.get(key)
        .and_then(|v| v.as_str())
        .map(str::to_string)
        .ok_or_else(|| format!("missing string Alpaca field `{}` in {}", key, obj).into())
}
