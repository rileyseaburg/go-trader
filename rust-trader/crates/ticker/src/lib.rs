//! go-trader-ticker: Market data polling and basket management.
pub mod basket;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use tracing::{info, warn};

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
        }
    }

    pub async fn start(&self) {
        *self.running.write().unwrap() = true;
        info!("TickerServer started (mock={})", self.mock_mode);
        self.update_market_data().await;
        self.spawn_poller();
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
        tokio::spawn(async move {
            let mut interval = tokio::time::interval(std::time::Duration::from_secs(5));
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
    for sym in symbols {
        let (trade, quote, bar) = fetch_snapshot(http, api_key, api_secret, sym).await?;
        let data = TickerData {
            symbol: sym.clone(),
            trade: Some(trade),
            quote: Some(quote),
            bar,
            last_updated: Utc::now(),
        };
        last_data.write().unwrap().insert(sym.clone(), data.clone());
        if let Some(ref h) = *handler.read().unwrap() {
            h.handle(sym, &data);
        }
    }
    Ok(())
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
