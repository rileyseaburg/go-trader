//! Alpaca trade updates WebSocket consumer.
//!
//! This stream is the primary source for order state transitions. REST polling
//! remains useful for reconciliation, but every pushed event is appended to the
//! SQLite audit log as it arrives.

use std::time::Duration;

use futures_util::{SinkExt, StreamExt};
use go_trader_notification::NotificationManager;
use serde_json::Value;
use tokio_tungstenite::{connect_async, tungstenite::Message};
use tracing::{error, info, warn};

use crate::audit_store::{json_f64, json_str, AuditStore, FillRecord, OrderRecord};
use std::{collections::HashMap, sync::Arc};

const PAPER_STREAM_URL: &str = "wss://paper-api.alpaca.markets/stream";
const LIVE_STREAM_URL: &str = "wss://api.alpaca.markets/stream";
const WS_ALERT_AFTER_SECS: u64 = 30;

pub fn spawn_trade_updates_stream(
    api_key: String,
    api_secret: String,
    paper: bool,
    audit: AuditStore,
    notifications: Arc<NotificationManager>,
) {
    tokio::spawn(async move {
        let url = if paper {
            PAPER_STREAM_URL
        } else {
            LIVE_STREAM_URL
        };
        let mut disconnected_since: Option<std::time::Instant> = None;
        loop {
            let started_connected = std::time::Instant::now();
            if let Err(e) = run_once(url, &api_key, &api_secret, &audit).await {
                error!("Alpaca trade updates stream disconnected: {}", e);
                let since = disconnected_since.get_or_insert(started_connected);
                let disconnected_for = since.elapsed().as_secs();
                if disconnected_for >= WS_ALERT_AFTER_SECS {
                    push_ws_alert(&notifications, disconnected_for);
                }
            } else {
                disconnected_since = None;
            }
            tokio::time::sleep(Duration::from_secs(5)).await;
        }
    });
}

async fn run_once(
    url: &str,
    api_key: &str,
    api_secret: &str,
    audit: &AuditStore,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    info!("Connecting Alpaca trade updates stream: {}", url);
    let (mut ws, _) = connect_async(url).await?;

    ws.send(Message::Text(
        serde_json::json!({
            "action": "authenticate",
            "data": {"key_id": api_key, "secret_key": api_secret}
        })
        .to_string()
        .into(),
    ))
    .await?;
    ws.send(Message::Text(
        serde_json::json!({
            "action": "listen",
            "data": {"streams": ["trade_updates"]}
        })
        .to_string()
        .into(),
    ))
    .await?;

    while let Some(msg) = ws.next().await {
        match msg? {
            Message::Text(text) => handle_message(&text, audit),
            Message::Binary(bytes) => {
                if let Ok(text) = String::from_utf8(bytes.to_vec()) {
                    handle_message(&text, audit);
                }
            }
            Message::Ping(payload) => ws.send(Message::Pong(payload)).await?,
            Message::Close(frame) => {
                warn!("Alpaca trade updates stream closed: {:?}", frame);
                break;
            }
            _ => {}
        }
    }
    Ok(())
}

fn handle_message(text: &str, audit: &AuditStore) {
    let Ok(value) = serde_json::from_str::<Value>(text) else {
        warn!("Invalid Alpaca stream JSON: {}", text);
        return;
    };

    let stream = json_str(&value, "stream").unwrap_or_default();
    if stream != "trade_updates" {
        info!("Alpaca stream control message: {}", value);
        return;
    }

    let data = value.get("data").unwrap_or(&value);
    let event = json_str(data, "event").unwrap_or("unknown");
    let order = data.get("order").unwrap_or(data);

    let alpaca_order_id = json_str(order, "id");
    let symbol = json_str(order, "symbol");
    let side = json_str(order, "side");
    let qty = json_f64(order, "qty");
    let order_type = json_str(order, "type");
    let time_in_force = json_str(order, "time_in_force");
    let limit_price = json_f64(order, "limit_price");
    let status = json_str(order, "status").or(Some(event));

    if let Err(e) = audit.record_order(OrderRecord {
        alpaca_order_id,
        signal_id: None,
        symbol,
        side,
        qty,
        order_type,
        time_in_force,
        limit_price,
        status,
        source: "alpaca_ws",
        raw: value.clone(),
    }) {
        error!(
            "AUDIT_DB_WRITE_FAILED: failed to persist Alpaca trade update order event: {}",
            e
        );
    }

    if matches!(event, "fill" | "partial_fill") {
        if let Err(e) = audit.record_fill(FillRecord {
            alpaca_order_id,
            trade_id: json_str(data, "trade_id"),
            symbol,
            side,
            qty: json_f64(data, "qty").or_else(|| json_f64(order, "filled_qty")),
            price: json_f64(data, "price").or_else(|| json_f64(order, "filled_avg_price")),
            raw: value.clone(),
        }) {
            error!(
                "AUDIT_DB_WRITE_FAILED: failed to persist Alpaca fill event: {}",
                e
            );
        }
    }
}

fn push_ws_alert(notifications: &NotificationManager, disconnected_for: u64) {
    let mut metadata = HashMap::new();
    metadata.insert(
        "disconnected_for_seconds".into(),
        serde_json::json!(disconnected_for),
    );
    metadata.insert(
        "recommended_action".into(),
        serde_json::json!(
            "Run GET /v2/orders reconciliation and verify no state transitions were missed."
        ),
    );
    notifications.add_notification(go_trader_notification::create_system_alert_notification(
        "Alpaca WebSocket reconnect delay",
        &format!(
            "Trade updates stream has been disconnected for at least {} seconds; reconcile orders after reconnect.",
            disconnected_for
        ),
        Some(metadata),
    ));
}
