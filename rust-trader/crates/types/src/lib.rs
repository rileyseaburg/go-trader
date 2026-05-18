//! go-trader-types: Shared types for the go-trader algorithmic trading platform.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Trade signal types
// ---------------------------------------------------------------------------

pub const SIGNAL_BUY: &str = "buy";
pub const SIGNAL_SELL: &str = "sell";
pub const SIGNAL_HOLD: &str = "hold";
pub const SIGNAL_CLOSE: &str = "close";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct TradeSignal {
    pub symbol: String,
    pub signal: String,
    pub order_type: String, // "market" | "limit"
    #[serde(skip_serializing_if = "Option::is_none")]
    pub limit_price: Option<f64>,
    pub timestamp: DateTime<Utc>,
    pub reasoning: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub confidence: Option<f64>,
}

// ---------------------------------------------------------------------------
// Market data
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MarketData {
    pub symbol: String,
    pub price: f64,
    pub high_24h: f64,
    pub low_24h: f64,
    pub volume_24h: f64,
    pub change_24h: f64,
}

// ---------------------------------------------------------------------------
// Historical data
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HistoricalDataPoint {
    pub timestamp: DateTime<Utc>,
    pub symbol: String,
    pub open: f64,
    pub high: f64,
    pub low: f64,
    pub close: f64,
    pub volume: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum TimeFrame {
    #[serde(rename = "1Min")]
    OneMinute,
    #[serde(rename = "5Min")]
    FiveMinutes,
    #[serde(rename = "15Min")]
    FifteenMinutes,
    #[serde(rename = "1H")]
    OneHour,
    #[serde(rename = "1D")]
    OneDay,
    #[serde(rename = "1W")]
    OneWeek,
    #[serde(rename = "1M")]
    OneMonth,
}

impl TimeFrame {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::OneMinute => "1Min",
            Self::FiveMinutes => "5Min",
            Self::FifteenMinutes => "15Min",
            Self::OneHour => "1H",
            Self::OneDay => "1D",
            Self::OneWeek => "1W",
            Self::OneMonth => "1M",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HistoricalDataRequest {
    pub symbol: String,
    pub start_date: DateTime<Utc>,
    pub end_date: DateTime<Utc>,
    pub timeframe: TimeFrame,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HistoricalData {
    pub symbol: String,
    pub timeframe: String,
    pub start_date: DateTime<Utc>,
    pub end_date: DateTime<Utc>,
    pub data: Vec<HistoricalDataPoint>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HistoricalDataAnalysis {
    pub symbol: String,
    pub timeframe: String,
    pub start_date: DateTime<Utc>,
    pub end_date: DateTime<Utc>,
    pub indicators: serde_json::Value,
    pub stats: std::collections::HashMap<String, f64>,
    pub recommendations: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RecommendedTicker {
    pub symbol: String,
    pub confidence: f64,
    pub reasoning: String,
}
