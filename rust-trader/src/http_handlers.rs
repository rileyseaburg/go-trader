use crate::{
    alpaca_rest::AlpacaRestClient,
    audit_store::{AuditStore, AuditTable, DecisionRecord},
};
use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::IntoResponse,
    routing::{any, get, post},
    Json, Router,
};
use go_trader_algorithm::TradingAlgorithm;
use go_trader_cartography as carto;
use go_trader_notification::NotificationManager;
use go_trader_ticker::TickerServer;
use serde::Deserialize;
use std::sync::Arc;
use tower_http::cors::CorsLayer;
use tower_http::services::{ServeDir, ServeFile};
pub type FeedCacheArc = Arc<Option<carto::feed::FeedCache>>;
pub struct AppState {
    pub api_key: String,
    pub api_secret: String,
    pub base_url: String,
    pub algo: Arc<TradingAlgorithm>,
    pub ticker: Arc<TickerServer>,
    pub notifications: Arc<NotificationManager>,
    // The cartography endpoint and scheduler use this when the optional FRED
    // overlay is configured. Keep the field explicit so a formula-only build
    // still documents the optional dependency in AppState.
    #[allow(dead_code)]
    pub feed_cache: FeedCacheArc,
    pub mock_mode: bool,
    pub dry_run: bool,
    pub audit: AuditStore,
}
fn alpaca(s: &AppState) -> AlpacaRestClient {
    AlpacaRestClient::new(s.api_key.clone(), s.api_secret.clone(), s.base_url.clone())
        .with_audit(s.audit.clone())
        .with_dry_run(s.dry_run)
}

#[allow(clippy::too_many_arguments)]
pub fn build_router(
    api_key: String,
    api_secret: String,
    base_url: String,
    algo: Arc<TradingAlgorithm>,
    ticker: Arc<TickerServer>,
    notif: Arc<NotificationManager>,
    fc: FeedCacheArc,
    mock: bool,
    dry_run: bool,
    audit: AuditStore,
) -> Router {
    let st = Arc::new(AppState {
        api_key,
        api_secret,
        base_url,
        algo,
        ticker,
        notifications: notif,
        feed_cache: fc,
        mock_mode: mock,
        dry_run,
        audit,
    });
    let api = Router::new()
        .route("/api/account", get(get_account))
        .route("/api/positions", get(get_positions))
        .route("/api/orders", get(get_orders))
        .route("/api/runtime-status", get(get_runtime_status))
        .route("/api/tickers", get(get_tickers).post(update_tickers))
        .route("/api/signals", get(get_signals))
        .route(
            "/api/risk-parameters",
            get(get_risk_params).post(update_risk_params),
        )
        .route("/api/algorithm/status", get(algo_status))
        .route("/api/algorithm/start", post(start_algo))
        .route("/api/algorithm/stop", post(stop_algo))
        .route("/api/algorithm/process/{symbol}", post(process_symbol))
        .route("/api/recommendations", get(get_recommendations))
        .route("/api/cartography", get(get_cartography))
        .route("/api/notifications", get(get_notifs))
        .route("/api/notifications/{id}", get(get_notif).delete(del_notif))
        .route("/api/notifications/read-all", post(read_all))
        .route("/api/audit/summary", get(get_audit_summary))
        .route("/api/audit/{table}", get(get_audit_table))
        .route("/api/indicators/{symbol}", get(get_indicators))
        .route("/api/execute/buy/{symbol}", post(exec_buy))
        .route("/api/execute/sell/{symbol}", post(exec_sell))
        .route("/api/{*path}", any(api_not_found))
        .with_state(st);

    api.fallback_service(ServeDir::new(".").not_found_service(ServeFile::new("index.html")))
        .layer(CorsLayer::permissive())
}

async fn api_not_found(Path(path): Path<String>) -> impl IntoResponse {
    (
        StatusCode::NOT_FOUND,
        Json(serde_json::json!({"error":"api route not found","path":format!("/api/{path}")})),
    )
}

async fn get_account(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    if s.mock_mode {
        return Json(serde_json::json!({"cash":"100000.00","equity":"100000.00"})).into_response();
    }
    match alpaca(&s).get_account_raw().await {
        Ok(v) => Json(v).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}
async fn get_positions(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    if s.mock_mode {
        return Json(serde_json::json!([])).into_response();
    }
    match alpaca(&s).get_positions_raw().await {
        Ok(v) => Json(v).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}
async fn get_orders(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    if s.mock_mode {
        return Json(serde_json::json!([])).into_response();
    }
    match alpaca(&s).get_orders_raw().await {
        Ok(v) => Json(v).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}
async fn get_runtime_status(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    let trading_mode = if s.mock_mode {
        "MOCK"
    } else if s.dry_run {
        "DRY_RUN"
    } else {
        "LIVE_PAPER"
    };

    Json(serde_json::json!({
        "trading_mode": trading_mode,
        "mock_mode": s.mock_mode,
        "dry_run": s.dry_run,
        "order_submission_enabled": !s.mock_mode && !s.dry_run,
        "broker": if s.mock_mode { "mock" } else { "alpaca" },
        "broker_environment": if s.base_url.contains("paper") { "paper" } else { "live_or_custom" },
        "base_url": s.base_url,
        "algorithm_running": s.algo.is_running(),
        "regime_name": s.algo.get_regime_name(),
        "regime_multiplier": s.algo.get_regime_multiplier(),
    }))
}
async fn get_tickers(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    Json(serde_json::json!({"symbols":s.ticker.get_symbols(),"data":s.ticker.get_all_last_data()}))
}
#[derive(Deserialize)]
struct TickReq {
    symbols: Vec<String>,
}
async fn update_tickers(
    State(s): State<Arc<AppState>>,
    Json(r): Json<TickReq>,
) -> impl IntoResponse {
    s.ticker.update_symbols(r.symbols.clone());
    s.algo.start(&r.symbols);
    Json(serde_json::json!({"status":"ok","symbols":r.symbols}))
}
#[derive(Deserialize)]
struct SigQ {
    symbol: Option<String>,
}
async fn get_signals(State(s): State<Arc<AppState>>, Query(q): Query<SigQ>) -> impl IntoResponse {
    match q.symbol {
        Some(sym) => Json(
            serde_json::json!({"signals":s.algo.get_signal(&sym).map(|s|vec![s]).unwrap_or_default()}),
        ),
        None => Json(serde_json::json!({"signals":s.algo.get_all_signals()})),
    }
}
async fn get_risk_params(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    Json(serde_json::to_value(s.algo.get_risk_parameters()).unwrap_or_default())
}
async fn update_risk_params(
    State(s): State<Arc<AppState>>,
    Json(p): Json<go_trader_algorithm::RiskParameters>,
) -> impl IntoResponse {
    s.algo.update_risk_parameters(p);
    Json(serde_json::json!({"status":"ok"}))
}
async fn algo_status(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    Json(
        serde_json::json!({"running":s.algo.is_running(),"regime":s.algo.get_regime_name(),"multiplier":s.algo.get_regime_multiplier(),"portfolio":s.algo.get_portfolio()}),
    )
}
#[derive(Deserialize)]
struct StartReq {
    symbols: Vec<String>,
}
async fn start_algo(State(s): State<Arc<AppState>>, Json(r): Json<StartReq>) -> impl IntoResponse {
    s.algo.start(&r.symbols);
    Json(serde_json::json!({"status":"started"}))
}
async fn stop_algo(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    s.algo.stop();
    Json(serde_json::json!({"status":"stopped"}))
}
async fn process_symbol(
    State(s): State<Arc<AppState>>,
    Path(sym): Path<String>,
) -> impl IntoResponse {
    match s.algo.process_symbol(&sym).await {
        Ok(sig) => {
            match s.audit.record_signal(&sig) {
                Ok(signal_id) => {
                    let action = sig.signal.clone();
                    if let Err(e) = s.audit.record_decision(DecisionRecord {
                        signal_id: &signal_id,
                        action: &action,
                        reason: &sig.reasoning,
                        governance: serde_json::json!({
                            "source": "process_symbol",
                            "regime_name": s.algo.get_regime_name(),
                            "regime_multiplier": s.algo.get_regime_multiplier(),
                            "mock_mode": s.mock_mode
                        }),
                    }) {
                        alert_audit_failure(&s, "Failed to persist algorithm decision", e);
                    }
                }
                Err(e) => alert_audit_failure(&s, "Failed to persist algorithm signal", e),
            }
            Json(serde_json::to_value(sig).unwrap_or_default()).into_response()
        }
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}
async fn get_recommendations() -> impl IntoResponse {
    Json(serde_json::json!({"recommendations": []}))
}
async fn get_cartography(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    let reading = carto::reading_at(chrono::Utc::now());
    let feed = match s.feed_cache.as_ref() {
        Some(cache) => match cache.get_fresh(chrono::Utc::now()) {
            Some(feed) => Some(feed),
            None => match cache.refresh().await {
                Ok(feed) => Some(feed),
                Err(e) => {
                    s.notifications.add_notification(
                        go_trader_notification::create_system_alert_notification(
                            "FRED feed refresh failed",
                            &format!("Failed to refresh FRED cartography overlay: {}", e),
                            None,
                        ),
                    );
                    cache.get()
                }
            },
        },
        None => None,
    };
    let applied_multiplier =
        carto::feed::applied_multiplier(reading.regime.multiplier, feed.as_ref());
    Json(serde_json::json!({
        "reading": reading,
        "feed": feed,
        "applied_multiplier": applied_multiplier,
        "series":carto::series(2000.0,2030.0,0.25)
    }))
}
async fn get_notifs(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    Json(serde_json::json!({"notifications":s.notifications.get_notifications()}))
}
async fn get_notif(State(s): State<Arc<AppState>>, Path(id): Path<String>) -> impl IntoResponse {
    match s
        .notifications
        .get_notifications()
        .into_iter()
        .find(|n| n.id == id)
    {
        Some(n) => Json(serde_json::to_value(n).unwrap_or_default()).into_response(),
        None => (StatusCode::NOT_FOUND, "not found").into_response(),
    }
}
async fn del_notif(State(s): State<Arc<AppState>>, Path(id): Path<String>) -> impl IntoResponse {
    if s.notifications.delete_notification(&id) {
        Json(serde_json::json!({"status":"deleted"})).into_response()
    } else {
        (StatusCode::NOT_FOUND, "not found").into_response()
    }
}
async fn read_all(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    s.notifications.mark_all_as_read();
    Json(serde_json::json!({"status":"ok"}))
}

#[derive(Deserialize)]
struct AuditQ {
    since: Option<String>,
    limit: Option<usize>,
}

async fn get_audit_summary(State(s): State<Arc<AppState>>) -> impl IntoResponse {
    match s.audit.summary() {
        Ok(summary) => Json(serde_json::json!({"summary": summary})).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}

async fn get_audit_table(
    State(s): State<Arc<AppState>>,
    Path(table): Path<String>,
    Query(q): Query<AuditQ>,
) -> impl IntoResponse {
    let Some(table) = AuditTable::parse(&table) else {
        return (
            StatusCode::BAD_REQUEST,
            "unknown audit table; use signals, decisions, orders, fills, or regime_states",
        )
            .into_response();
    };

    match s
        .audit
        .recent_rows(table, q.since.as_deref(), q.limit.unwrap_or(50))
    {
        Ok(rows) => Json(serde_json::json!({"rows": rows})).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}

async fn get_indicators(
    State(s): State<Arc<AppState>>,
    Path(symbol): Path<String>,
) -> impl IntoResponse {
    match s.algo.get_indicators(&symbol) {
        Some(ind) => Json(serde_json::to_value(ind).unwrap_or_default()).into_response(),
        None => (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({
                "error": "no indicator data available",
                "symbol": symbol,
                "hint": "indicators are computed from the bar buffer during auto-trade evaluation; ensure bars have accumulated (≥20 bars required)"
            })),
        )
            .into_response(),
    }
}

async fn exec_buy(State(s): State<Arc<AppState>>, Path(sym): Path<String>) -> impl IntoResponse {
    let latest_price = s
        .ticker
        .get_last_data(&sym)
        .and_then(|d| {
            d.trade
                .map(|t| t.price)
                .or_else(|| d.quote.map(|q| q.ask_price))
        })
        .filter(|p| *p > 0.0);
    let sig = go_trader_types::TradeSignal {
        symbol: sym,
        signal: "buy".into(),
        order_type: "market".into(),
        limit_price: latest_price,
        timestamp: chrono::Utc::now(),
        reasoning: "Manual".into(),
        confidence: None,
        audit: Some(serde_json::json!({"pipeline":"manual_http_execute","action":"buy"})),
    };
    persist_manual_decision(&s, &sig);
    match s.algo.execute_trade(&sig).await {
        Ok(m) => Json(serde_json::json!({"status":"ok","message":m})).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}
async fn exec_sell(State(s): State<Arc<AppState>>, Path(sym): Path<String>) -> impl IntoResponse {
    let sig = go_trader_types::TradeSignal {
        symbol: sym,
        signal: "sell".into(),
        order_type: "market".into(),
        limit_price: None,
        timestamp: chrono::Utc::now(),
        reasoning: "Manual".into(),
        confidence: None,
        audit: Some(serde_json::json!({"pipeline":"manual_http_execute","action":"sell"})),
    };
    persist_manual_decision(&s, &sig);
    match s.algo.execute_trade(&sig).await {
        Ok(m) => Json(serde_json::json!({"status":"ok","message":m})).into_response(),
        Err(e) => (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()).into_response(),
    }
}

fn persist_manual_decision(s: &Arc<AppState>, sig: &go_trader_types::TradeSignal) {
    match s.audit.record_signal(sig) {
        Ok(signal_id) => {
            if let Err(e) = s.audit.record_decision(DecisionRecord {
                signal_id: &signal_id,
                action: &sig.signal,
                reason: &sig.reasoning,
                governance: serde_json::json!({"source":"manual_http_execute","mock_mode":s.mock_mode,"dry_run":s.dry_run}),
            }) {
                alert_audit_failure(s, "Failed to persist manual decision", e);
            }
        }
        Err(e) => alert_audit_failure(s, "Failed to persist manual signal", e),
    }
}

fn alert_audit_failure(
    s: &Arc<AppState>,
    context: &str,
    err: Box<dyn std::error::Error + Send + Sync>,
) {
    tracing::error!("AUDIT_DB_WRITE_FAILED: {}: {}", context, err);
    s.notifications
        .add_notification(go_trader_notification::create_system_alert_notification(
            "Audit DB write failed",
            &format!("{}: {}", context, err),
            None,
        ));
}
