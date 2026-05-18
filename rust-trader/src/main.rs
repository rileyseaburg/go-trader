//! go-trader: Algorithmic trading platform in Rust with Alpaca Markets API.

mod alpaca_rest;
mod alpaca_stream;
mod audit_store;
mod bar_buffer;
mod http_handlers;

use std::sync::{Arc, RwLock};

use chrono::{Datelike, TimeZone, Timelike};
use chrono_tz::America::New_York;
use clap::Parser;
use tracing::{error, info, warn};

use go_trader_algorithm::{PositionData, TradingAlgorithm};
use go_trader_cartography::{self as cartography, Reading};
use go_trader_notification::NotificationManager;
use go_trader_ticker::{change_from_snapshot, DataHandler, TickerData, TickerServer};
use go_trader_types::TradeSignal;
use go_trader_backtest::{BacktestEngine, BacktestConfig};

use crate::{
    alpaca_rest::AlpacaRestClient,
    audit_store::{AuditStore, DecisionRecord},
    bar_buffer::BarBuffer,
};

const DEFAULT_PORT: &str = "8080";
const DEFAULT_SYMBOLS: &str = "AAPL,MSFT,TSLA";
const PAPER_URL: &str = "https://paper-api.alpaca.markets";
const LIVE_URL: &str = "https://api.alpaca.markets";
const MAX_NOTIFICATIONS: usize = 100;
const REGIME_SNAPSHOT_CHECK_SECS: u64 = 60;
// Four daily regime samples anchored to the US equities market clock.  These
// local New York times remain 09:30/10:30/14:30/16:55 across EST/EDT changes;
// chrono-tz handles the corresponding UTC offset.
const REGIME_SNAPSHOT_MARKET_TIMES_ET: [(u32, u32); 4] = [(9, 30), (10, 30), (14, 30), (16, 55)];

#[derive(Parser, Debug)]
#[command(
    name = "go-trader",
    about = "Algorithmic trading platform powered by Alpaca"
)]
struct Args {
    #[arg(long, default_value = DEFAULT_PORT)]
    port: String,
    #[arg(long, default_value = DEFAULT_SYMBOLS)]
    symbols: String,
    #[arg(long, default_value_t = true)]
    paper: bool,
    #[arg(long)]
    mock: bool,
    #[arg(long, env = "ALPACA_KEY")]
    alpaca_key: Option<String>,
    #[arg(long, env = "ALPACA_SECRET")]
    alpaca_secret: Option<String>,
    #[arg(
        long,
        env = "GO_TRADER_DB",
        default_value = "/app/data/go-trader.sqlite3"
    )]
    db_path: String,
    /// Validate live Alpaca reads but short-circuit order submission.
    #[arg(long, env = "GO_TRADER_DRY_RUN", default_value_t = false)]
    dry_run: bool,
    /// Run the algorithmic trading loop. Defaults off so paper/live order flow is explicit.
    #[arg(long, env = "GO_TRADER_AUTO_TRADE", default_value_t = false)]
    auto_trade: bool,
    /// Seconds between automatic signal scans.
    #[arg(
        long,
        env = "GO_TRADER_AUTO_TRADE_INTERVAL_SECS",
        default_value_t = 300
    )]
    auto_trade_interval_secs: u64,
    /// Run historical backtest instead of live trading.
    #[arg(long, default_value_t = false)]
    backtest: bool,
    /// Equity to start backtest with.
    #[arg(long, default_value_t = 5000.0)]
    backtest_equity: f64,
    /// Number of historical bars for backtest.
    #[arg(long, default_value_t = 1000)]
    backtest_bars: u32,
}

#[tokio::main]
async fn main() {
    // Load .env
    let _ = dotenvy::dotenv();

    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive("go_trader=info".parse().unwrap()),
        )
        .init();

    let args = Args::parse();

    // Backtest mode: fetch historical bars and simulate
    if args.backtest {
        run_backtest(args).await;
        return;
    }

    // Resolve API keys
    let mock_mode = args.mock || std::env::var("GO_TRADER_MOCK").unwrap_or_default() == "true";
    let mut api_key = args
        .alpaca_key
        .clone()
        .or_else(|| std::env::var("PAPER_ALPACA_API_KEY").ok())
        .unwrap_or_default();
    let mut api_secret = args
        .alpaca_secret
        .clone()
        .or_else(|| std::env::var("PAPER_ALPACA_SECRET_KEY").ok())
        .or_else(|| std::env::var("PAPAER_ALPACA_SECRET_KEY").ok()) // Handle Go typo
        .unwrap_or_default();

    if mock_mode {
        info!("Running in mock mode — no live trades");
        if api_key.is_empty() {
            api_key = "MOCK_ALPACA_API_KEY".into();
        }
        if api_secret.is_empty() {
            api_secret = "MOCK_ALPACA_SECRET_KEY".into();
        }
    }

    if api_key.is_empty() || api_secret.is_empty() {
        error!("API keys required. Set PAPER_ALPACA_API_KEY/SECRET or use --mock");
        std::process::exit(1);
    }

    let base_url = if args.paper { PAPER_URL } else { LIVE_URL }.to_string();
    info!(
        "Using {} trading environment",
        if args.paper { "PAPER" } else { "LIVE" }
    );

    let audit = AuditStore::open(&args.db_path).unwrap_or_else(|e| {
        error!(
            "Failed to initialize SQLite audit log at {}: {}",
            args.db_path, e
        );
        std::process::exit(1);
    });
    info!("SQLite audit log initialized at {}", args.db_path);

    // Parse symbols
    let symbols: Vec<String> = args
        .symbols
        .split(',')
        .map(|s| s.trim().to_string())
        .collect();

    // Initialize components
    if args.dry_run {
        info!("Dry-run trading enabled — Alpaca read APIs are live but order submission is short-circuited");
    }
    let alpaca_client =
        AlpacaRestClient::new(api_key.clone(), api_secret.clone(), base_url.clone())
            .with_audit(audit.clone())
            .with_dry_run(args.dry_run);
    let mut algorithm = TradingAlgorithm::new().with_alpaca(Box::new(alpaca_client.clone()));
    if let Some(model_client) = go_trader_algorithm::ModelSignalClient::from_env() {
        info!(
            model = model_client.model_name(),
            "External model suggestions enabled for trading signals"
        );
        algorithm = algorithm.with_claude(Box::new(model_client));
    } else {
        info!("External model suggestions disabled; using local deterministic signal engine");
    }
    let trading_algo = Arc::new(algorithm);
    sync_portfolio_from_alpaca(&trading_algo, &alpaca_client).await;
    spawn_portfolio_sync_scheduler(trading_algo.clone(), alpaca_client.clone());
    let notification_mgr = Arc::new(NotificationManager::new(MAX_NOTIFICATIONS));
    let price_tracker: Arc<RwLock<std::collections::HashMap<String, f64>>> =
        Arc::new(RwLock::new(std::collections::HashMap::new()));
    let bar_buffer = Arc::new(BarBuffer::with_default_capacity());

    // Seed bar buffer with historical bars so indicators are available immediately.
    if !mock_mode {
        seed_bar_buffer(&alpaca_client, &bar_buffer, &symbols).await;
    } else {
        info!("Mock mode — skipping historical bar fetch");
    }

    // Cartography setup
    // TODO: wire async Vault loading here; for now, use FRED_API_KEY when present.
    let fred_key = std::env::var("FRED_API_KEY").unwrap_or_default();

    let feed_cache: Arc<Option<cartography::feed::FeedCache>> = if !fred_key.is_empty() {
        info!("Cartography FRED overlay enabled");
        Arc::new(Some(cartography::feed::FeedCache::new(
            cartography::fred::FREDClient::new(fred_key),
            chrono::Duration::hours(6),
        )))
    } else {
        info!("Cartography running formula-only — no FRED key");
        Arc::new(None)
    };

    // Apply cartography reading
    let reading = snapshot_regime_state(
        &trading_algo,
        &feed_cache,
        &audit,
        &notification_mgr,
        "initial",
    )
    .await;
    spawn_regime_snapshot_scheduler(
        trading_algo.clone(),
        feed_cache.clone(),
        audit.clone(),
        notification_mgr.clone(),
    );
    info!(
        "Cartography: {} ×{:.2}",
        reading.regime.name, reading.regime.multiplier
    );

    // Ticker server
    let ticker_server = Arc::new(TickerServer::new(
        args.paper,
        api_key.clone(),
        api_secret.clone(),
    ));

    // Set up data handler to forward ticker -> algorithm
    let algo_clone = trading_algo.clone();
    let notif_clone = notification_mgr.clone();
    let pt_clone = price_tracker.clone();
    let buf_clone = bar_buffer.clone();
    struct Handler {
        algo: Arc<TradingAlgorithm>,
        notif: Arc<NotificationManager>,
        pt: Arc<RwLock<std::collections::HashMap<String, f64>>>,
        buf: Arc<BarBuffer>,
    }
    impl DataHandler for Handler {
        fn handle(&self, symbol: &str, data: &TickerData) {
            let price = data.trade.as_ref().map(|t| t.price).unwrap_or(0.0);
            let high = data.bar.as_ref().map(|b| b.high).unwrap_or(price * 1.05);
            let low = data.bar.as_ref().map(|b| b.low).unwrap_or(price * 0.95);
            let volume = data.bar.as_ref().map(|b| b.volume as f64).unwrap_or(1000.0);
            let change = change_from_snapshot(data);
            self.algo
                .update_market_data(symbol, price, high, low, volume, change);

            // Push bar into buffer for indicator computation.
            if data.bar.is_some() {
                let now_ms = chrono::Utc::now().timestamp_millis();
                let bar = go_trader_indicators::Bar {
                    timestamp: now_ms,
                    open: price, // approximate from trade price
                    high,
                    low,
                    close: price,
                    volume,
                };
                self.buf.push(symbol, bar);
            }

            let prev = self.pt.read().unwrap().get(symbol).copied().unwrap_or(0.0);
            if prev > 0.0 {
                let change = (price - prev) / prev;
                if change.abs() > 0.02 {
                    let (event, dir) = if change > 0.0 {
                        ("Significant Price Increase", "+")
                    } else {
                        ("Significant Price Decrease", "")
                    };
                    let notif = go_trader_notification::create_market_event_notification(
                        symbol,
                        event,
                        &format!(
                            "{} price changed {}{:.2}% to ${:.2}",
                            symbol,
                            dir,
                            change * 100.0,
                            price
                        ),
                    );
                    self.notif.add_notification(notif);
                }
            }
            self.pt.write().unwrap().insert(symbol.to_string(), price);
        }
    }
    ticker_server.set_data_handler(Box::new(Handler {
        algo: algo_clone,
        notif: notif_clone,
        pt: pt_clone,
        buf: buf_clone,
    }));

    // Start ticker
    ticker_server.update_symbols(symbols.clone());
    ticker_server.start().await;

    if !mock_mode {
        alpaca_stream::spawn_trade_updates_stream(
            api_key.clone(),
            api_secret.clone(),
            args.paper,
            audit.clone(),
            notification_mgr.clone(),
        );
    }

    // Start algorithm
    trading_algo.start(&symbols);
    info!("Trading algorithm initialized — waiting for UI trigger");

    // Register signal callback
    let notif_for_cb = notification_mgr.clone();
    trading_algo.register_signal_callback(Box::new(move |signal| {
        let priority = if signal.signal == "buy" || signal.signal == "sell" {
            go_trader_notification::NotificationPriority::High
        } else {
            go_trader_notification::NotificationPriority::Medium
        };
        let notif = go_trader_notification::create_signal_generated_notification(
            &signal.symbol,
            &signal.signal,
            &signal.reasoning,
            priority,
            None,
        );
        notif_for_cb.add_notification(notif);
    }));

    if args.auto_trade {
        spawn_auto_trade_loop(
            trading_algo.clone(),
            alpaca_client.clone(),
            audit.clone(),
            symbols.clone(),
            args.auto_trade_interval_secs,
            args.dry_run,
            bar_buffer.clone(),
        );
    } else {
        info!("Auto-trade loop disabled; use --auto-trade to enable automated paper/live order evaluation");
    }

    // Build and start HTTP server
    let app = http_handlers::build_router(
        api_key,
        api_secret,
        base_url,
        trading_algo.clone(),
        ticker_server.clone(),
        notification_mgr.clone(),
        feed_cache.clone(),
        mock_mode,
        args.dry_run,
        audit.clone(),
    );

    let addr = format!("0.0.0.0:{}", args.port);
    info!("Starting HTTP server on {}", addr);
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

async fn run_backtest(args: Args) {
    let api_key = args.alpaca_key.clone()
        .or_else(|| std::env::var("PAPER_ALPACA_API_KEY").ok())
        .unwrap_or_default();
    let api_secret = args.alpaca_secret.clone()
        .or_else(|| std::env::var("PAPER_ALPACA_SECRET_KEY").ok())
        .or_else(|| std::env::var("PAPAER_ALPACA_SECRET_KEY").ok())
        .unwrap_or_default();
    if api_key.is_empty() || api_secret.is_empty() {
        error!("API keys required for backtest. Set PAPER_ALPACA_API_KEY/SECRET or use --alpaca-key/--alpaca-secret");
        std::process::exit(1);
    }
    let base_url = if args.paper { PAPER_URL } else { LIVE_URL }.to_string();
    let client = AlpacaRestClient::new(api_key, api_secret, base_url);
    let symbols: Vec<String> = args.symbols.split(',').map(|s| s.trim().to_string()).collect();
    let timeframe = "5Min";
    let config = BacktestConfig {
        starting_equity: args.backtest_equity,
        warmup_bars: 50,
        ..Default::default()
    };
    for symbol in &symbols {
        info!(symbol, bars = args.backtest_bars, "Fetching historical bars for backtest");
        match client.get_bars(symbol, timeframe, args.backtest_bars).await {
            Ok(bars) => {
                info!(symbol, count = bars.len(), "Running backtest");
                let mut engine = BacktestEngine::new(config.clone());
                let report = engine.run(symbol, bars);
                report.print_summary();
                let json_path = format!("/tmp/backtest_{}.json", symbol.to_lowercase());
                match serde_json::to_string_pretty(&report) {
                    Ok(json) => {
                        std::fs::write(&json_path, json).ok();
                        info!(symbol, path = %json_path, "Report saved");
                    }
                    Err(e) => warn!(symbol, "Failed to serialize report: {}", e),
                }
            }
            Err(e) => error!(symbol, "Failed to fetch bars: {}", e),
        }
    }
}

fn spawn_auto_trade_loop(
    trading_algo: Arc<TradingAlgorithm>,
    alpaca_client: AlpacaRestClient,
    audit: AuditStore,
    symbols: Vec<String>,
    interval_secs: u64,
    dry_run: bool,
    bar_buffer: Arc<BarBuffer>,
) {
    let interval_secs = interval_secs.max(15);
    tokio::spawn(async move {
        info!(
            symbols = symbols.join(","),
            interval_secs, dry_run, "Auto-trade loop enabled"
        );
        let mut interval = tokio::time::interval(std::time::Duration::from_secs(interval_secs));
        interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);
        loop {
            interval.tick().await;
            sync_portfolio_from_alpaca(&trading_algo, &alpaca_client).await;

            // Refresh indicators from bar buffer before evaluating.
            refresh_indicators(&trading_algo, &bar_buffer, &symbols);

            for symbol in &symbols {
                if let Err(e) =
                    evaluate_symbol_for_auto_trade(&trading_algo, &audit, symbol, dry_run).await
                {
                    warn!(symbol, "AUTO_TRADE_EVALUATION_FAILED: {}", e);
                }
            }
        }
    });
}

async fn evaluate_symbol_for_auto_trade(
    trading_algo: &Arc<TradingAlgorithm>,
    audit: &AuditStore,
    symbol: &str,
    dry_run: bool,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let signal = trading_algo.process_symbol(symbol).await?;
    let signal_id = audit.record_signal(&signal)?;
    audit.record_decision(DecisionRecord {
        signal_id: &signal_id,
        action: &signal.signal,
        reason: &signal.reasoning,
        governance: serde_json::json!({
            "source": "auto_trade_loop",
            "regime_name": trading_algo.get_regime_name(),
            "regime_multiplier": trading_algo.get_regime_multiplier(),
            "dry_run": dry_run,
        }),
    })?;

    if is_actionable_signal(&signal) {
        match trading_algo.execute_trade(&signal).await {
            Ok(message) => info!(
                symbol,
                action = signal.signal,
                message,
                "AUTO_TRADE_ORDER_SUBMITTED"
            ),
            Err(e) => warn!(
                symbol,
                action = signal.signal,
                "AUTO_TRADE_ORDER_FAILED: {}",
                e
            ),
        }
    } else {
        info!(
            symbol,
            action = signal.signal,
            "Auto-trade held; no order submitted"
        );
    }
    Ok(())
}

/// Compute fresh indicators from the bar buffer and push them to the algorithm.
fn refresh_indicators(algo: &Arc<TradingAlgorithm>, buf: &Arc<BarBuffer>, symbols: &[String]) {
    for symbol in symbols {
        let bars = buf.get_bars(symbol);
        if bars.len() >= 20 {
            let set = go_trader_indicators::compute_all(&bars);
            algo.update_indicators(symbol, set);
        }
    }
}

/// Fetch historical 5-minute bars from Alpaca and seed the bar buffer.
///
/// This ensures indicator computation has enough data immediately on
/// startup rather than waiting for live ticks to accumulate.
async fn seed_bar_buffer(
    alpaca_client: &AlpacaRestClient,
    buf: &Arc<BarBuffer>,
    symbols: &[String],
) {
    // 300 five-minute bars = ~1 trading day of data, enough for all indicators
    // including 200-SMA.
    const BAR_COUNT: u32 = 300;
    const TIMEFRAME: &str = "5Min";

    for symbol in symbols {
        match alpaca_client.get_bars(symbol, TIMEFRAME, BAR_COUNT).await {
            Ok(bars) => {
                for bar in &bars {
                    buf.push(symbol, bar.clone());
                }
                info!(
                    symbol,
                    bars = bars.len(),
                    "Seeded bar buffer with historical bars"
                );
            }
            Err(e) => {
                warn!(
                    symbol,
                    "HISTORICAL_BARS_FETCH_FAILED: {}: {}", TIMEFRAME, e
                );
            }
        }
    }
}

fn is_actionable_signal(signal: &TradeSignal) -> bool {
    matches!(signal.signal.as_str(), "buy" | "sell") && signal.confidence.unwrap_or(0.0) >= 0.65
}

async fn sync_portfolio_from_alpaca(
    trading_algo: &Arc<TradingAlgorithm>,
    alpaca_client: &AlpacaRestClient,
) {
    let account = match alpaca_client.get_account_raw().await {
        Ok(account) => account,
        Err(e) => {
            warn!("PORTFOLIO_SYNC_FAILED: account fetch failed: {}", e);
            return;
        }
    };
    let positions_raw = match alpaca_client.get_positions_raw().await {
        Ok(positions) => positions,
        Err(e) => {
            warn!("PORTFOLIO_SYNC_FAILED: positions fetch failed: {}", e);
            return;
        }
    };

    let cash = json_num_or_str(&account, "cash");
    let equity = json_num_or_str(&account, "equity");
    let portfolio_value = json_num_or_str(&account, "portfolio_value");
    let daily_pnl = json_num_or_str(&account, "equity") - json_num_or_str(&account, "last_equity");
    let last_equity = json_num_or_str(&account, "last_equity");
    let daily_return = if last_equity.abs() > f64::EPSILON {
        daily_pnl / last_equity
    } else {
        0.0
    };

    let mut positions = std::collections::HashMap::new();
    for pos in positions_raw.as_array().into_iter().flatten() {
        let symbol = json_string(pos, "symbol");
        if symbol.is_empty() {
            continue;
        }
        positions.insert(
            symbol.clone(),
            PositionData {
                symbol,
                quantity: json_num_or_str(pos, "qty"),
                avg_price: json_num_or_str(pos, "avg_entry_price"),
                market_val: json_num_or_str(pos, "market_value"),
                profit: json_num_or_str(pos, "unrealized_pl"),
                return_pct: json_num_or_str(pos, "unrealized_plpc"),
            },
        );
    }
    trading_algo.update_portfolio(
        cash,
        portfolio_value.max(equity),
        daily_pnl,
        daily_return,
        positions,
    );
    info!(
        positions = trading_algo.get_portfolio().positions.len(),
        "Portfolio synced from Alpaca"
    );
}

fn spawn_portfolio_sync_scheduler(
    trading_algo: Arc<TradingAlgorithm>,
    alpaca_client: AlpacaRestClient,
) {
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(std::time::Duration::from_secs(60));
        interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);
        loop {
            interval.tick().await;
            sync_portfolio_from_alpaca(&trading_algo, &alpaca_client).await;
        }
    });
}

fn json_num_or_str(v: &serde_json::Value, key: &str) -> f64 {
    v.get(key)
        .and_then(|raw| {
            raw.as_f64()
                .or_else(|| raw.as_str().and_then(|s| s.parse().ok()))
        })
        .unwrap_or(0.0)
}

fn json_string(v: &serde_json::Value, key: &str) -> String {
    v.get(key)
        .and_then(|raw| raw.as_str())
        .unwrap_or_default()
        .to_string()
}

async fn snapshot_regime_state(
    trading_algo: &Arc<TradingAlgorithm>,
    feed_cache: &Arc<Option<cartography::feed::FeedCache>>,
    audit: &AuditStore,
    notification_mgr: &Arc<NotificationManager>,
    source: &str,
) -> Reading {
    let reading_at = chrono::Utc::now();
    let mut reading = cartography::reading_at(reading_at);

    if let Some(cache) = feed_cache.as_ref() {
        let feed = match cache.get_fresh(reading_at) {
            Some(feed) => Some(feed),
            None => match cache.refresh().await {
                Ok(feed) => Some(feed),
                Err(e) => {
                    warn!(
                        "FRED_FEED_REFRESH_FAILED: failed to refresh {} regime overlay: {}",
                        source, e
                    );
                    cache.get()
                }
            },
        };
        let applied =
            cartography::feed::applied_multiplier(reading.regime.multiplier, feed.as_ref());
        if applied < reading.regime.multiplier {
            reading.regime.name = format!("{} + FRED RISK OVERLAY", reading.regime.name);
            reading.regime.multiplier = applied;
        }
    }

    trading_algo.set_regime_multiplier(&reading.regime.name, reading.regime.multiplier);
    if let Err(e) = audit.record_regime_state(
        reading_at,
        &reading.regime.name,
        reading.regime.multiplier,
        &reading,
    ) {
        error!(
            "AUDIT_DB_WRITE_FAILED: failed to persist {} regime state: {}",
            source, e
        );
        notification_mgr.add_notification(
            go_trader_notification::create_system_alert_notification(
                "Audit DB write failed",
                &format!("Failed to persist {} regime state: {}", source, e),
                None,
            ),
        );
    }
    reading
}

fn spawn_regime_snapshot_scheduler(
    trading_algo: Arc<TradingAlgorithm>,
    feed_cache: Arc<Option<cartography::feed::FeedCache>>,
    audit: AuditStore,
    notification_mgr: Arc<NotificationManager>,
) {
    tokio::spawn(async move {
        let mut ticker =
            tokio::time::interval(std::time::Duration::from_secs(REGIME_SNAPSHOT_CHECK_SECS));
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);
        ticker.tick().await; // skip immediate tick; boot snapshot was already written
        let mut last_snapshot_slot: Option<String> = None;

        loop {
            ticker.tick().await;
            let now = chrono::Utc::now();
            if let Some(slot_id) = regime_snapshot_market_slot_id(now) {
                if last_snapshot_slot.as_deref() == Some(slot_id.as_str()) {
                    continue;
                }
                last_snapshot_slot = Some(slot_id);
                let reading = snapshot_regime_state(
                    &trading_algo,
                    &feed_cache,
                    &audit,
                    &notification_mgr,
                    "scheduled",
                )
                .await;
                info!(
                    "Scheduled cartography snapshot: {} ×{:.2}",
                    reading.regime.name, reading.regime.multiplier
                );
            }
        }
    });
}

fn regime_snapshot_market_slot_id(now: chrono::DateTime<chrono::Utc>) -> Option<String> {
    let market_now = New_York.from_utc_datetime(&now.naive_utc());
    let weekday = market_now.weekday();
    if matches!(weekday, chrono::Weekday::Sat | chrono::Weekday::Sun) {
        return None;
    }
    let hour = market_now.hour();
    let minute = market_now.minute();
    let is_slot = REGIME_SNAPSHOT_MARKET_TIMES_ET
        .iter()
        .any(|(slot_hour, slot_minute)| hour == *slot_hour && minute == *slot_minute);
    if is_slot {
        Some(format!(
            "{}-{:02}:{:02}-America/New_York",
            market_now.date_naive(),
            hour,
            minute
        ))
    } else {
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn regime_snapshot_slots_follow_new_york_standard_time() {
        let winter_open = chrono::Utc.with_ymd_and_hms(2026, 1, 5, 14, 30, 0).unwrap();
        assert_eq!(
            regime_snapshot_market_slot_id(winter_open).as_deref(),
            Some("2026-01-05-09:30-America/New_York")
        );
    }

    #[test]
    fn regime_snapshot_slots_follow_new_york_daylight_time() {
        let summer_open = chrono::Utc.with_ymd_and_hms(2026, 7, 6, 13, 30, 0).unwrap();
        assert_eq!(
            regime_snapshot_market_slot_id(summer_open).as_deref(),
            Some("2026-07-06-09:30-America/New_York")
        );
        let stale_utc_early_slot = chrono::Utc.with_ymd_and_hms(2026, 7, 6, 15, 30, 0).unwrap();
        assert!(regime_snapshot_market_slot_id(stale_utc_early_slot).is_none());
    }
}
