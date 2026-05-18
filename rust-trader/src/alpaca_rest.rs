//! Alpaca REST API client — direct HTTP calls per Alpaca documentation.
//!
//! Trading API: https://paper-api.alpaca.markets (paper) / https://api.alpaca.markets (live)
//! Market Data: https://data.alpaca.markets
//! Auth: APCA-API-KEY-ID + APCA-API-SECRET-KEY headers

use go_trader_algorithm::{
    AlpacaAccount, AlpacaClient, AlpacaOrder, AlpacaOrderRequest, AlpacaPosition,
};

use crate::audit_store::{json_f64, json_str, AuditStore, OrderRecord};
use go_trader_indicators::Bar;

/// Alpaca market data base URL (separate from trading API).
const DATA_BASE_URL: &str = "https://data.alpaca.markets";

pub struct AlpacaRestClient {
    api_key: String,
    api_secret: String,
    base_url: String,
    data_url: String,
    http: reqwest::Client,
    audit: Option<AuditStore>,
    dry_run: bool,
}

impl Clone for AlpacaRestClient {
    fn clone(&self) -> Self {
        Self {
            api_key: self.api_key.clone(),
            api_secret: self.api_secret.clone(),
            base_url: self.base_url.clone(),
            data_url: self.data_url.clone(),
            http: self.http.clone(),
            audit: self.audit.clone(),
            dry_run: self.dry_run,
        }
    }
}

impl AlpacaRestClient {
    pub fn new(api_key: String, api_secret: String, base_url: String) -> Self {
        Self {
            api_key,
            api_secret,
            base_url,
            data_url: DATA_BASE_URL.to_string(),
            http: reqwest::Client::new(),
            audit: None,
            dry_run: false,
        }
    }

    #[allow(dead_code)]
    pub fn with_data_url(mut self, url: String) -> Self {
        self.data_url = url;
        self
    }

    pub fn with_audit(mut self, audit: AuditStore) -> Self {
        self.audit = Some(audit);
        self
    }

    pub fn with_dry_run(mut self, dry_run: bool) -> Self {
        self.dry_run = dry_run;
        self
    }

    fn auth(&self) -> reqwest::header::HeaderMap {
        let mut h = reqwest::header::HeaderMap::new();
        h.insert("APCA-API-KEY-ID", self.api_key.parse().unwrap());
        h.insert("APCA-API-SECRET-KEY", self.api_secret.parse().unwrap());
        h
    }

    pub async fn get_account_raw(
        &self,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let resp = self
            .http
            .get(format!("{}/v2/account", self.base_url))
            .headers(self.auth())
            .send()
            .await?;
        Ok(resp.json().await?)
    }

    pub async fn get_positions_raw(
        &self,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let resp = self
            .http
            .get(format!("{}/v2/positions", self.base_url))
            .headers(self.auth())
            .send()
            .await?;
        Ok(resp.json().await?)
    }

    pub async fn get_position_raw(
        &self,
        symbol: &str,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let resp = self
            .http
            .get(format!("{}/v2/positions/{}", self.base_url, symbol))
            .headers(self.auth())
            .send()
            .await?;
        Ok(resp.json().await?)
    }

    pub async fn place_order_raw(
        &self,
        order: &AlpacaOrderRequest,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let body = serde_json::json!({
            "symbol": order.symbol, "side": order.side, "type": order.order_type,
            "time_in_force": order.time_in_force, "qty": order.qty.to_string(),
            "limit_price": order.limit_price.map(|p| format!("{:.2}", p)),
        });
        let value: serde_json::Value = if self.dry_run {
            serde_json::json!({
                "id": format!("dry-run-{}", uuid::Uuid::new_v4()),
                "status": "dry_run_not_submitted",
                "symbol": order.symbol,
                "side": order.side,
                "qty": order.qty.to_string(),
                "type": order.order_type,
                "time_in_force": order.time_in_force,
                "limit_price": order.limit_price.map(|p| format!("{:.2}", p)),
                "request": body,
            })
        } else {
            let resp = self
                .http
                .post(format!("{}/v2/orders", self.base_url))
                .headers(self.auth())
                .json(&body)
                .send()
                .await?;
            let status = resp.status();
            let text = resp.text().await?;
            if !status.is_success() {
                return Err(format!("Alpaca order: {} {}", status, text).into());
            }
            serde_json::from_str(&text)?
        };
        if let Some(audit) = &self.audit {
            let signal_id = audit.latest_signal_id(&order.symbol).ok().flatten();
            if let Err(e) = audit.record_order(OrderRecord {
                alpaca_order_id: json_str(&value, "id"),
                signal_id: signal_id.as_deref(),
                symbol: Some(&order.symbol),
                side: Some(&order.side),
                qty: Some(order.qty),
                order_type: Some(&order.order_type),
                time_in_force: Some(&order.time_in_force),
                limit_price: order.limit_price,
                status: json_str(&value, "status"),
                source: "alpaca_rest_submit",
                raw: value.clone(),
            }) {
                tracing::error!(
                    "AUDIT_DB_WRITE_FAILED: failed to persist order submission: {}",
                    e
                );
            }
        }
        Ok(value)
    }

    pub async fn get_orders_raw(
        &self,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let resp = self
            .http
            .get(format!("{}/v2/orders?status=all&limit=50", self.base_url))
            .headers(self.auth())
            .send()
            .await?;
        Ok(resp.json().await?)
    }

    // -----------------------------------------------------------------------
    // Market Data API (data.alpaca.markets)
    // -----------------------------------------------------------------------

    /// Fetch historical OHLCV bars for a symbol.
    ///
    /// Alpaca endpoint: `GET /v2/stocks/{symbol}/bars`
    /// Returns bars sorted ascending by timestamp.
    #[allow(dead_code)]
    pub async fn get_bars_raw(
        &self,
        symbol: &str,
        timeframe: &str,  // e.g. "1Min", "5Min", "15Min", "1H", "1D"
        limit: u32,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        // Look back 7 calendar days to ensure we capture bars even on
        // weekends / holidays when "now" has no recent data.
        let start = chrono::Utc::now() - chrono::Duration::days(7);
        let start_str = start.format("%Y-%m-%dT%H:%M:%SZ").to_string();
        let url = format!(
            "{}/v2/stocks/{}/bars?timeframe={}&limit={}&adjustment=raw&feed=iex&sort=asc&start={}",
            self.data_url, symbol, timeframe, limit, start_str
        );
        let resp = self
            .http
            .get(&url)
            .headers(self.auth())
            .send()
            .await?;
        let status = resp.status();
        let text = resp.text().await?;
        if !status.is_success() {
            return Err(format!("Alpaca bars: {} {}", status, text).into());
        }
        Ok(serde_json::from_str(&text)?)
    }

    /// Fetch bars and parse into typed `Bar` structs.
    #[allow(dead_code)]
    pub async fn get_bars(
        &self,
        symbol: &str,
        timeframe: &str,
        limit: u32,
    ) -> Result<Vec<Bar>, Box<dyn std::error::Error + Send + Sync>> {
        let raw = self.get_bars_raw(symbol, timeframe, limit).await?;
        let bars_val = raw
            .get("bars")
            .cloned()
            .unwrap_or(serde_json::Value::Array(vec![]));
        let arr = bars_val
            .as_array()
            .cloned()
            .unwrap_or_default();

        let bars: Vec<Bar> = arr
            .iter()
            .filter_map(|b| {
                let ts = b.get("t")?.as_str()?.parse::<i64>().ok()?;
                Some(Bar {
                    timestamp: ts,
                    open: json_f64(b, "o")?,
                    high: json_f64(b, "h")?,
                    low: json_f64(b, "l")?,
                    close: json_f64(b, "c")?,
                    volume: json_f64(b, "v")?,
                })
            })
            .collect();

        tracing::debug!(
            "Fetched {} bars for {} ({})",
            bars.len(),
            symbol,
            timeframe
        );
        Ok(bars)
    }

    /// Fetch the latest quote for a symbol.
    #[allow(dead_code)]
    pub async fn get_latest_quote_raw(
        &self,
        symbol: &str,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let url = format!(
            "{}/v2/stocks/{}/quotes/latest",
            self.data_url, symbol
        );
        let resp = self
            .http
            .get(&url)
            .headers(self.auth())
            .send()
            .await?;
        let status = resp.status();
        let text = resp.text().await?;
        if !status.is_success() {
            return Err(format!("Alpaca quote: {} {}", status, text).into());
        }
        Ok(serde_json::from_str(&text)?)
    }
}

// Helpers to extract f64 from Alpaca's string-valued JSON fields
fn str_f64(v: &serde_json::Value, key: &str) -> f64 {
    json_f64(v, key).unwrap_or(0.0)
}
fn str_string(v: &serde_json::Value, key: &str) -> String {
    json_str(v, key).unwrap_or("").into()
}

impl AlpacaClient for AlpacaRestClient {
    fn get_account(&self) -> Result<AlpacaAccount, Box<dyn std::error::Error + Send + Sync>> {
        let client = self.clone();
        let v = blocking_on_async(async move { client.get_account_raw().await })?;
        Ok(AlpacaAccount {
            cash: str_f64(&v, "cash"),
            equity: str_f64(&v, "equity"),
            portfolio_value: str_f64(&v, "portfolio_value"),
        })
    }
    fn get_position(
        &self,
        symbol: &str,
    ) -> Result<AlpacaPosition, Box<dyn std::error::Error + Send + Sync>> {
        let client = self.clone();
        let symbol = symbol.to_string();
        let v = blocking_on_async(async move { client.get_position_raw(&symbol).await })?;
        Ok(AlpacaPosition {
            symbol: str_string(&v, "symbol"),
            qty: str_f64(&v, "qty"),
            avg_entry_price: str_f64(&v, "avg_entry_price"),
            market_value: str_f64(&v, "market_value"),
            unrealized_pl: str_f64(&v, "unrealized_pl"),
            unrealized_plpc: str_f64(&v, "unrealized_plpc"),
        })
    }
    fn place_order(
        &self,
        req: AlpacaOrderRequest,
    ) -> Result<AlpacaOrder, Box<dyn std::error::Error + Send + Sync>> {
        let client = self.clone();
        let v = blocking_on_async(async move { client.place_order_raw(&req).await })?;
        Ok(AlpacaOrder {
            id: str_string(&v, "id"),
            status: str_string(&v, "status"),
            filled_avg_price: v
                .get("filled_avg_price")
                .and_then(|v| v.as_str())
                .and_then(|s| s.parse().ok()),
        })
    }
    fn get_positions(
        &self,
    ) -> Result<Vec<AlpacaPosition>, Box<dyn std::error::Error + Send + Sync>> {
        let client = self.clone();
        let v = blocking_on_async(async move { client.get_positions_raw().await })?;
        let arr = v.as_array().cloned().unwrap_or_default();
        Ok(arr
            .iter()
            .map(|v| AlpacaPosition {
                symbol: str_string(v, "symbol"),
                qty: str_f64(v, "qty"),
                avg_entry_price: str_f64(v, "avg_entry_price"),
                market_value: str_f64(v, "market_value"),
                unrealized_pl: str_f64(v, "unrealized_pl"),
                unrealized_plpc: str_f64(v, "unrealized_plpc"),
            })
            .collect())
    }
}

fn blocking_on_async<F, T>(future: F) -> Result<T, Box<dyn std::error::Error + Send + Sync>>
where
    F: std::future::Future<Output = Result<T, Box<dyn std::error::Error + Send + Sync>>>
        + Send
        + 'static,
    T: Send + 'static,
{
    std::thread::spawn(move || {
        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .map_err(|e| -> Box<dyn std::error::Error + Send + Sync> { Box::new(e) })?;
        rt.block_on(future)
    })
    .join()
    .map_err(|_| "Alpaca blocking bridge thread panicked")?
}
