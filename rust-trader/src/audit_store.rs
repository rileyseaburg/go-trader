//! Append-only Postgres audit log for trading decisions and executions.
//!
//! The audit store now writes to Postgres instead of a local SQLite file. The legacy
//! `--db-path` argument is accepted for CLI compatibility, but the connection target is
//! selected from `GO_TRADER_DATABASE_URL`, `DATABASE_URL`, `POSTGRES_URL`, or the project
//! default Postgres instance.

use std::sync::{Arc, Mutex};

use chrono::{DateTime, Utc};
use postgres::{Client, NoTls, Row};
use serde::Serialize;
use uuid::Uuid;

use go_trader_types::TradeSignal;

const DEFAULT_DATABASE_URL: &str = "postgres://postgres:postgres@192.168.50.70/postgres";

type AuditResult<T> = Result<T, Box<dyn std::error::Error + Send + Sync>>;

#[derive(Clone)]
pub struct AuditStore {
    client: Arc<Mutex<Client>>,
}

#[derive(Debug, Clone, Copy)]
pub enum AuditTable {
    Signals,
    Decisions,
    Orders,
    Fills,
    RegimeStates,
}

impl AuditTable {
    pub fn parse(value: &str) -> Option<Self> {
        match value {
            "signals" => Some(Self::Signals),
            "decisions" => Some(Self::Decisions),
            "orders" => Some(Self::Orders),
            "fills" => Some(Self::Fills),
            "regime_states" | "regime-states" | "regimes" => Some(Self::RegimeStates),
            _ => None,
        }
    }

    fn table_name(self) -> &'static str {
        match self {
            Self::Signals => "signals",
            Self::Decisions => "decisions",
            Self::Orders => "orders",
            Self::Fills => "fills",
            Self::RegimeStates => "regime_states",
        }
    }

    fn columns(self) -> &'static [&'static str] {
        match self {
            Self::Signals => &[
                "id",
                "created_at",
                "symbol",
                "signal",
                "order_type",
                "limit_price",
                "reasoning",
                "confidence",
                "market_data_json",
                "portfolio_json",
                "claude_input_json",
                "claude_output_json",
                "raw_json",
            ],
            Self::Decisions => &[
                "id",
                "created_at",
                "signal_id",
                "action",
                "reason",
                "governance_json",
            ],
            Self::Orders => &[
                "id",
                "created_at",
                "alpaca_order_id",
                "signal_id",
                "symbol",
                "side",
                "qty",
                "order_type",
                "time_in_force",
                "limit_price",
                "status",
                "source",
                "raw_json",
            ],
            Self::Fills => &[
                "id",
                "created_at",
                "alpaca_order_id",
                "trade_id",
                "symbol",
                "side",
                "qty",
                "price",
                "raw_json",
            ],
            Self::RegimeStates => &[
                "id",
                "created_at",
                "regime_name",
                "multiplier",
                "reading_json",
            ],
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct AuditSummary {
    pub signals: i64,
    pub decisions: i64,
    pub orders: i64,
    pub fills: i64,
    pub regime_states: i64,
    pub latest_signal_at: Option<String>,
    pub latest_order_at: Option<String>,
    pub latest_fill_at: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
pub struct DecisionRecord<'a> {
    pub signal_id: &'a str,
    pub action: &'a str,
    pub reason: &'a str,
    pub governance: serde_json::Value,
}

#[derive(Debug, Clone, Serialize)]
pub struct OrderRecord<'a> {
    pub alpaca_order_id: Option<&'a str>,
    pub signal_id: Option<&'a str>,
    pub symbol: Option<&'a str>,
    pub side: Option<&'a str>,
    pub qty: Option<f64>,
    pub order_type: Option<&'a str>,
    pub time_in_force: Option<&'a str>,
    pub limit_price: Option<f64>,
    pub status: Option<&'a str>,
    pub source: &'a str,
    pub raw: serde_json::Value,
}

#[derive(Debug, Clone, Serialize)]
pub struct FillRecord<'a> {
    pub alpaca_order_id: Option<&'a str>,
    pub trade_id: Option<&'a str>,
    pub symbol: Option<&'a str>,
    pub side: Option<&'a str>,
    pub qty: Option<f64>,
    pub price: Option<f64>,
    pub raw: serde_json::Value,
}

impl AuditStore {
    pub fn open(_legacy_path: impl AsRef<std::path::Path>) -> AuditResult<Self> {
        let database_url = std::env::var("GO_TRADER_DATABASE_URL")
            .or_else(|_| std::env::var("DATABASE_URL"))
            .or_else(|_| std::env::var("POSTGRES_URL"))
            .unwrap_or_else(|_| DEFAULT_DATABASE_URL.to_string());

        let client = Client::connect(&database_url, NoTls)?;
        let store = Self {
            client: Arc::new(Mutex::new(client)),
        };
        store.migrate()?;
        tracing::info!("AuditStore connected to Postgres audit database");
        Ok(store)
    }

    fn migrate(&self) -> AuditResult<()> {
        let mut client = self.client.lock().unwrap();
        client.batch_execute(
            r#"
            CREATE TABLE IF NOT EXISTS signals (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                symbol TEXT NOT NULL,
                signal TEXT NOT NULL,
                order_type TEXT NOT NULL,
                limit_price DOUBLE PRECISION,
                reasoning TEXT NOT NULL,
                confidence DOUBLE PRECISION,
                market_data_json JSONB,
                portfolio_json JSONB,
                claude_input_json JSONB,
                claude_output_json JSONB NOT NULL,
                raw_json JSONB NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_signals_symbol_created_at ON signals(symbol, created_at DESC);
            CREATE INDEX IF NOT EXISTS idx_signals_created_at ON signals(created_at DESC);

            CREATE TABLE IF NOT EXISTS decisions (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                signal_id TEXT NOT NULL,
                action TEXT NOT NULL,
                reason TEXT NOT NULL,
                governance_json JSONB NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_decisions_signal_id ON decisions(signal_id);
            CREATE INDEX IF NOT EXISTS idx_decisions_created_at ON decisions(created_at DESC);

            CREATE TABLE IF NOT EXISTS orders (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                alpaca_order_id TEXT,
                signal_id TEXT,
                symbol TEXT,
                side TEXT,
                qty DOUBLE PRECISION,
                order_type TEXT,
                time_in_force TEXT,
                limit_price DOUBLE PRECISION,
                status TEXT,
                source TEXT NOT NULL,
                raw_json JSONB NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_orders_alpaca_order_id ON orders(alpaca_order_id);
            CREATE INDEX IF NOT EXISTS idx_orders_signal_id ON orders(signal_id);
            CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);

            CREATE TABLE IF NOT EXISTS fills (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                alpaca_order_id TEXT,
                trade_id TEXT,
                symbol TEXT,
                side TEXT,
                qty DOUBLE PRECISION,
                price DOUBLE PRECISION,
                raw_json JSONB NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_fills_alpaca_order_id ON fills(alpaca_order_id);
            CREATE INDEX IF NOT EXISTS idx_fills_created_at ON fills(created_at DESC);

            CREATE TABLE IF NOT EXISTS regime_states (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                regime_name TEXT NOT NULL,
                multiplier DOUBLE PRECISION NOT NULL,
                reading_json JSONB NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_regime_states_created_at ON regime_states(created_at DESC);
            "#,
        )?;
        Ok(())
    }

    /// Delete hold-only signals and decisions older than `days` to prevent unbounded DB growth.
    /// Buy/sell signals are preserved for forensic audit. Orders, fills, and regime states
    /// are small tables and are never pruned.
    /// Returns (deleted_signals, deleted_decisions).
    pub fn prune_old_records(&self, days: u32) -> AuditResult<(usize, usize)> {
        let cutoff = (Utc::now() - chrono::Duration::days(days as i64)).to_rfc3339();
        let mut client = self.client.lock().unwrap();
        let deleted_signals = client.execute(
            "DELETE FROM signals WHERE created_at < $1 AND signal = 'hold'",
            &[&cutoff],
        )? as usize;
        let deleted_decisions = client.execute(
            "DELETE FROM decisions WHERE created_at < $1 AND action = 'hold'",
            &[&cutoff],
        )? as usize;
        tracing::info!(
            "pruned {deleted_signals} old hold signals and {deleted_decisions} old hold decisions older than {days} days"
        );
        Ok((deleted_signals, deleted_decisions))
    }

    pub fn record_signal(&self, signal: &TradeSignal) -> AuditResult<String> {
        let id = Uuid::new_v4().to_string();
        let raw = serde_json::to_value(signal)?;
        let created_at = signal.timestamp.to_rfc3339();
        let mut client = self.client.lock().unwrap();
        client.execute(
            "INSERT INTO signals (id, created_at, symbol, signal, order_type, limit_price, reasoning, confidence, claude_output_json, raw_json) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
            &[&id, &created_at, &signal.symbol, &signal.signal, &signal.order_type, &signal.limit_price, &signal.reasoning, &signal.confidence, &raw, &raw],
        )?;
        Ok(id)
    }

    pub fn latest_signal_id(&self, symbol: &str) -> AuditResult<Option<String>> {
        let mut client = self.client.lock().unwrap();
        Ok(client
            .query_opt(
                "SELECT id FROM signals WHERE symbol = $1 ORDER BY created_at DESC LIMIT 1",
                &[&symbol],
            )?
            .map(|row| row.get(0)))
    }

    pub fn record_decision(&self, decision: DecisionRecord<'_>) -> AuditResult<String> {
        let id = Uuid::new_v4().to_string();
        let created_at = Utc::now().to_rfc3339();
        let mut client = self.client.lock().unwrap();
        client.execute(
            "INSERT INTO decisions (id, created_at, signal_id, action, reason, governance_json) VALUES ($1,$2,$3,$4,$5,$6)",
            &[&id, &created_at, &decision.signal_id, &decision.action, &decision.reason, &decision.governance],
        )?;
        Ok(id)
    }

    pub fn record_order(&self, order: OrderRecord<'_>) -> AuditResult<String> {
        let id = Uuid::new_v4().to_string();
        let created_at = Utc::now().to_rfc3339();
        let mut client = self.client.lock().unwrap();
        client.execute(
            "INSERT INTO orders (id, created_at, alpaca_order_id, signal_id, symbol, side, qty, order_type, time_in_force, limit_price, status, source, raw_json) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)",
            &[&id, &created_at, &order.alpaca_order_id, &order.signal_id, &order.symbol, &order.side, &order.qty, &order.order_type, &order.time_in_force, &order.limit_price, &order.status, &order.source, &order.raw],
        )?;
        Ok(id)
    }

    pub fn record_fill(&self, fill: FillRecord<'_>) -> AuditResult<String> {
        let id = Uuid::new_v4().to_string();
        let created_at = Utc::now().to_rfc3339();
        let mut client = self.client.lock().unwrap();
        client.execute(
            "INSERT INTO fills (id, created_at, alpaca_order_id, trade_id, symbol, side, qty, price, raw_json) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
            &[&id, &created_at, &fill.alpaca_order_id, &fill.trade_id, &fill.symbol, &fill.side, &fill.qty, &fill.price, &fill.raw],
        )?;
        Ok(id)
    }

    pub fn record_regime_state<T: Serialize>(
        &self,
        created_at: DateTime<Utc>,
        name: &str,
        multiplier: f64,
        reading: &T,
    ) -> AuditResult<String> {
        let id = Uuid::new_v4().to_string();
        let created_at = created_at.to_rfc3339();
        let reading = serde_json::to_value(reading)?;
        let mut client = self.client.lock().unwrap();
        client.execute(
            "INSERT INTO regime_states (id, created_at, regime_name, multiplier, reading_json) VALUES ($1,$2,$3,$4,$5)",
            &[&id, &created_at, &name, &multiplier, &reading],
        )?;
        Ok(id)
    }

    pub fn recent_rows(
        &self,
        table: AuditTable,
        since: Option<&str>,
        limit: usize,
    ) -> AuditResult<Vec<serde_json::Value>> {
        let limit = i64::try_from(limit.clamp(1, 500)).unwrap_or(500);
        let columns = table.columns();
        let sql = if since.is_some() {
            format!(
                "SELECT {} FROM {} WHERE created_at >= $1 ORDER BY created_at DESC LIMIT $2",
                columns.join(", "),
                table.table_name()
            )
        } else {
            format!(
                "SELECT {} FROM {} ORDER BY created_at DESC LIMIT $1",
                columns.join(", "),
                table.table_name()
            )
        };

        let mut client = self.client.lock().unwrap();
        let rows = if let Some(since) = since {
            client.query(&sql, &[&since, &limit])?
        } else {
            client.query(&sql, &[&limit])?
        };

        rows.iter()
            .map(|row| row_to_json(row, columns))
            .collect::<AuditResult<Vec<_>>>()
    }

    pub fn summary(&self) -> AuditResult<AuditSummary> {
        let mut client = self.client.lock().unwrap();
        Ok(AuditSummary {
            signals: count_table(&mut client, "signals")?,
            decisions: count_table(&mut client, "decisions")?,
            orders: count_table(&mut client, "orders")?,
            fills: count_table(&mut client, "fills")?,
            regime_states: count_table(&mut client, "regime_states")?,
            latest_signal_at: latest_created_at(&mut client, "signals")?,
            latest_order_at: latest_created_at(&mut client, "orders")?,
            latest_fill_at: latest_created_at(&mut client, "fills")?,
        })
    }
}

fn row_to_json(row: &Row, columns: &[&str]) -> AuditResult<serde_json::Value> {
    let mut map = serde_json::Map::new();
    for (idx, column) in columns.iter().enumerate() {
        let value = if column.ends_with("_json") {
            row.get::<_, Option<serde_json::Value>>(idx)
                .unwrap_or(serde_json::Value::Null)
        } else if matches!(
            *column,
            "limit_price" | "confidence" | "qty" | "price" | "multiplier"
        ) {
            row.get::<_, Option<f64>>(idx)
                .map(serde_json::Value::from)
                .unwrap_or(serde_json::Value::Null)
        } else {
            row.get::<_, Option<String>>(idx)
                .map(serde_json::Value::String)
                .unwrap_or(serde_json::Value::Null)
        };
        map.insert((*column).to_string(), value);
    }
    Ok(serde_json::Value::Object(map))
}

fn count_table(client: &mut Client, table: &str) -> Result<i64, postgres::Error> {
    Ok(client
        .query_one(&format!("SELECT COUNT(*)::BIGINT FROM {table}"), &[])?
        .get(0))
}

fn latest_created_at(client: &mut Client, table: &str) -> Result<Option<String>, postgres::Error> {
    Ok(client
        .query_opt(
            &format!("SELECT created_at FROM {table} ORDER BY created_at DESC LIMIT 1"),
            &[],
        )?
        .map(|row| row.get(0)))
}

pub fn json_str<'a>(value: &'a serde_json::Value, key: &str) -> Option<&'a str> {
    value.get(key).and_then(|v| v.as_str())
}

pub fn json_f64(value: &serde_json::Value, key: &str) -> Option<f64> {
    value.get(key).and_then(|v| v.as_f64()).or_else(|| {
        value
            .get(key)
            .and_then(|v| v.as_str())
            .and_then(|s| s.parse().ok())
    })
}
