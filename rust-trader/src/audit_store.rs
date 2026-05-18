//! Append-only SQLite audit log for trading decisions and executions.
//!
//! The database lives on durable storage (default `/app/data/go-trader.sqlite3`).
//! Tables intentionally avoid UPDATE/DELETE paths: corrections and status changes are
//! represented as additional rows so the database remains forensic evidence.

use std::{
    path::Path,
    sync::{Arc, Mutex},
};

use chrono::{DateTime, Utc};
use rusqlite::{params, types::ValueRef, Connection, OptionalExtension};
use serde::Serialize;
use uuid::Uuid;

use go_trader_types::TradeSignal;

#[derive(Debug, Clone)]
pub struct AuditStore {
    conn: Arc<Mutex<Connection>>,
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
    pub fn open(path: impl AsRef<Path>) -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        if let Some(parent) = path.as_ref().parent() {
            std::fs::create_dir_all(parent)?;
        }
        let conn = Connection::open(path)?;
        conn.pragma_update(None, "journal_mode", "WAL")?;
        conn.pragma_update(None, "synchronous", "FULL")?;
        conn.pragma_update(None, "foreign_keys", "ON")?;
        let store = Self {
            conn: Arc::new(Mutex::new(conn)),
        };
        store.migrate()?;
        Ok(store)
    }

    fn migrate(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let conn = self.conn.lock().unwrap();
        conn.execute_batch(
            r#"
            CREATE TABLE IF NOT EXISTS signals (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                symbol TEXT NOT NULL,
                signal TEXT NOT NULL,
                order_type TEXT NOT NULL,
                limit_price REAL,
                reasoning TEXT NOT NULL,
                confidence REAL,
                market_data_json TEXT,
                portfolio_json TEXT,
                claude_input_json TEXT,
                claude_output_json TEXT NOT NULL,
                raw_json TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_signals_symbol_created_at ON signals(symbol, created_at);

            CREATE TABLE IF NOT EXISTS decisions (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                signal_id TEXT NOT NULL,
                action TEXT NOT NULL,
                reason TEXT NOT NULL,
                governance_json TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_decisions_signal_id ON decisions(signal_id);
            CREATE INDEX IF NOT EXISTS idx_decisions_created_at ON decisions(created_at);

            CREATE TABLE IF NOT EXISTS orders (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                alpaca_order_id TEXT,
                signal_id TEXT,
                symbol TEXT,
                side TEXT,
                qty REAL,
                order_type TEXT,
                time_in_force TEXT,
                limit_price REAL,
                status TEXT,
                source TEXT NOT NULL,
                raw_json TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_orders_alpaca_order_id ON orders(alpaca_order_id);
            CREATE INDEX IF NOT EXISTS idx_orders_signal_id ON orders(signal_id);
            CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);

            CREATE TABLE IF NOT EXISTS fills (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                alpaca_order_id TEXT,
                trade_id TEXT,
                symbol TEXT,
                side TEXT,
                qty REAL,
                price REAL,
                raw_json TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_fills_alpaca_order_id ON fills(alpaca_order_id);
            CREATE INDEX IF NOT EXISTS idx_fills_created_at ON fills(created_at);

            CREATE TABLE IF NOT EXISTS regime_states (
                id TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                regime_name TEXT NOT NULL,
                multiplier REAL NOT NULL,
                reading_json TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_regime_states_created_at ON regime_states(created_at);
            "#,
        )?;
        Ok(())
    }

    pub fn record_signal(
        &self,
        signal: &TradeSignal,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let id = Uuid::new_v4().to_string();
        let raw = serde_json::to_string(signal)?;
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "INSERT INTO signals (id, created_at, symbol, signal, order_type, limit_price, reasoning, confidence, claude_output_json, raw_json) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)",
            params![id, signal.timestamp.to_rfc3339(), signal.symbol, signal.signal, signal.order_type, signal.limit_price, signal.reasoning, signal.confidence, raw, raw],
        )?;
        Ok(id)
    }

    pub fn latest_signal_id(
        &self,
        symbol: &str,
    ) -> Result<Option<String>, Box<dyn std::error::Error + Send + Sync>> {
        let conn = self.conn.lock().unwrap();
        Ok(conn
            .query_row(
                "SELECT id FROM signals WHERE symbol = ?1 ORDER BY created_at DESC LIMIT 1",
                params![symbol],
                |row| row.get(0),
            )
            .optional()?)
    }

    pub fn record_decision(
        &self,
        decision: DecisionRecord<'_>,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let id = Uuid::new_v4().to_string();
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "INSERT INTO decisions (id, created_at, signal_id, action, reason, governance_json) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            params![id, Utc::now().to_rfc3339(), decision.signal_id, decision.action, decision.reason, decision.governance.to_string()],
        )?;
        Ok(id)
    }

    pub fn record_order(
        &self,
        order: OrderRecord<'_>,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let id = Uuid::new_v4().to_string();
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "INSERT INTO orders (id, created_at, alpaca_order_id, signal_id, symbol, side, qty, order_type, time_in_force, limit_price, status, source, raw_json) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)",
            params![id, Utc::now().to_rfc3339(), order.alpaca_order_id, order.signal_id, order.symbol, order.side, order.qty, order.order_type, order.time_in_force, order.limit_price, order.status, order.source, order.raw.to_string()],
        )?;
        Ok(id)
    }

    pub fn record_fill(
        &self,
        fill: FillRecord<'_>,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let id = Uuid::new_v4().to_string();
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "INSERT INTO fills (id, created_at, alpaca_order_id, trade_id, symbol, side, qty, price, raw_json) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)",
            params![id, Utc::now().to_rfc3339(), fill.alpaca_order_id, fill.trade_id, fill.symbol, fill.side, fill.qty, fill.price, fill.raw.to_string()],
        )?;
        Ok(id)
    }

    pub fn record_regime_state<T: Serialize>(
        &self,
        created_at: DateTime<Utc>,
        name: &str,
        multiplier: f64,
        reading: &T,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let id = Uuid::new_v4().to_string();
        let conn = self.conn.lock().unwrap();
        conn.execute(
            "INSERT INTO regime_states (id, created_at, regime_name, multiplier, reading_json) VALUES (?1, ?2, ?3, ?4, ?5)",
            params![id, created_at.to_rfc3339(), name, multiplier, serde_json::to_string(reading)?],
        )?;
        Ok(id)
    }

    pub fn recent_rows(
        &self,
        table: AuditTable,
        since: Option<&str>,
        limit: usize,
    ) -> Result<Vec<serde_json::Value>, Box<dyn std::error::Error + Send + Sync>> {
        let limit = limit.clamp(1, 500);
        let columns = table.columns();
        let sql = if since.is_some() {
            format!(
                "SELECT {} FROM {} WHERE created_at >= ?1 ORDER BY created_at DESC LIMIT ?2",
                columns.join(", "),
                table.table_name()
            )
        } else {
            format!(
                "SELECT {} FROM {} ORDER BY created_at DESC LIMIT ?1",
                columns.join(", "),
                table.table_name()
            )
        };

        let conn = self.conn.lock().unwrap();
        let mut stmt = conn.prepare(&sql)?;

        let mut out = Vec::new();
        if let Some(since) = since {
            let mut rows = stmt.query(params![since, limit as i64])?;
            while let Some(row) = rows.next()? {
                out.push(row_to_json(row, columns)?);
            }
        } else {
            let mut rows = stmt.query(params![limit as i64])?;
            while let Some(row) = rows.next()? {
                out.push(row_to_json(row, columns)?);
            }
        }

        Ok(out)
    }

    pub fn summary(&self) -> Result<AuditSummary, Box<dyn std::error::Error + Send + Sync>> {
        let conn = self.conn.lock().unwrap();
        Ok(AuditSummary {
            signals: count_table(&conn, "signals")?,
            decisions: count_table(&conn, "decisions")?,
            orders: count_table(&conn, "orders")?,
            fills: count_table(&conn, "fills")?,
            regime_states: count_table(&conn, "regime_states")?,
            latest_signal_at: latest_created_at(&conn, "signals")?,
            latest_order_at: latest_created_at(&conn, "orders")?,
            latest_fill_at: latest_created_at(&conn, "fills")?,
        })
    }
}

fn row_to_json(row: &rusqlite::Row<'_>, columns: &[&str]) -> rusqlite::Result<serde_json::Value> {
    let mut map = serde_json::Map::new();
    for (idx, column) in columns.iter().enumerate() {
        let value = match row.get_ref(idx)? {
            ValueRef::Null => serde_json::Value::Null,
            ValueRef::Integer(v) => serde_json::json!(v),
            ValueRef::Real(v) => serde_json::json!(v),
            ValueRef::Text(v) => decode_text_column(column, v),
            ValueRef::Blob(v) => serde_json::json!(format!("<{} bytes>", v.len())),
        };
        map.insert((*column).to_string(), value);
    }
    Ok(serde_json::Value::Object(map))
}

fn count_table(conn: &Connection, table: &str) -> rusqlite::Result<i64> {
    conn.query_row(&format!("SELECT COUNT(*) FROM {table}"), [], |row| {
        row.get(0)
    })
}

fn latest_created_at(conn: &Connection, table: &str) -> rusqlite::Result<Option<String>> {
    conn.query_row(
        &format!("SELECT created_at FROM {table} ORDER BY created_at DESC LIMIT 1"),
        [],
        |row| row.get(0),
    )
    .optional()
}

fn decode_text_column(column: &str, bytes: &[u8]) -> serde_json::Value {
    let text = String::from_utf8_lossy(bytes).to_string();
    if column.ends_with("_json") {
        serde_json::from_str(&text).unwrap_or(serde_json::Value::String(text))
    } else {
        serde_json::Value::String(text)
    }
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
