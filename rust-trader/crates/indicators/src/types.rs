//! Core types for the indicators crate.

use serde::{Deserialize, Serialize};

/// A single OHLCV bar — the atomic input for all indicators.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Bar {
    /// Unix epoch milliseconds or RFC3339 timestamp.
    pub timestamp: i64,
    pub open: f64,
    pub high: f64,
    pub low: f64,
    pub close: f64,
    pub volume: f64,
}

/// Snapshot of every indicator for a symbol, computed at evaluation time.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct IndicatorSet {
    // Moving averages
    pub sma_10: Option<f64>,
    pub sma_20: Option<f64>,
    pub sma_50: Option<f64>,
    pub sma_200: Option<f64>,
    pub ema_9: Option<f64>,
    pub ema_12: Option<f64>,
    pub ema_21: Option<f64>,
    pub ema_26: Option<f64>,
    pub ema_50: Option<f64>,
    pub ema_200: Option<f64>,

    // RSI
    pub rsi_14: Option<f64>,

    // MACD
    pub macd_line: Option<f64>,
    pub macd_signal: Option<f64>,
    pub macd_histogram: Option<f64>,

    // Bollinger Bands (20, 2σ)
    pub bb_upper: Option<f64>,
    pub bb_middle: Option<f64>,
    pub bb_lower: Option<f64>,
    pub bb_bandwidth: Option<f64>,
    pub bb_percent_b: Option<f64>,

    // ATR
    pub atr_14: Option<f64>,

    // VWAP
    pub vwap: Option<f64>,

    // ADX
    pub adx_14: Option<f64>,
    pub plus_di_14: Option<f64>,
    pub minus_di_14: Option<f64>,

    // Stochastic
    pub stoch_k: Option<f64>,
    pub stoch_d: Option<f64>,

    // Derived signals
    pub trend: TrendDirection,
    pub volatility: VolatilityLevel,
    pub momentum: MomentumState,

    /// Volume ratio: current bar volume / average volume (≥1.0 = above average).
    pub volume_ratio: Option<f64>,
}

impl IndicatorSet {
    pub fn adx(&self) -> f64 { self.adx_14.unwrap_or(0.0) }
    pub fn rsi(&self) -> f64 { self.rsi_14.unwrap_or(50.0) }
    pub fn sma_trend(&self) -> TrendDirection { self.trend }
    pub fn macd_value(&self) -> f64 { self.macd_line.unwrap_or(0.0) }
    pub fn bb_upper(&self) -> f64 { self.bb_upper.unwrap_or(f64::MAX) }
    pub fn bb_lower(&self) -> f64 { self.bb_lower.unwrap_or(0.0) }
    pub fn bb_middle(&self) -> f64 { self.bb_middle.unwrap_or(0.0) }
    pub fn volatility_level(&self) -> VolatilityLevel { self.volatility }
    pub fn momentum_state(&self) -> MomentumState { self.momentum }
    pub fn volume_ratio(&self) -> f64 { self.volume_ratio.unwrap_or(1.0) }

    pub fn macd_signal(&self) -> MACDSignal {
        match (self.macd_line, self.macd_signal, self.macd_histogram) {
            (Some(line), Some(sig), Some(h)) if line > sig && h > 0.0 => MACDSignal::BullishAbove,
            (Some(line), Some(sig), _) if line > sig => MACDSignal::BullishCross,
            (Some(line), Some(sig), Some(h)) if line < sig && h < 0.0 => MACDSignal::BearishBelow,
            (Some(line), Some(sig), _) if line < sig => MACDSignal::BearishCross,
            _ => MACDSignal::Neutral,
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, Default, PartialEq)]
pub enum TrendDirection {
    #[default]
    Neutral,
    Bullish,
    Bearish,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, Default, PartialEq)]
pub enum VolatilityLevel {
    #[default]
    Normal,
    Low,
    Elevated,
    High,
    Extreme,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, Default, PartialEq)]
pub enum MomentumState {
    #[default]
    Neutral,
    Overbought,
    Oversold,
    Rising,
    Falling,
    AcceleratingBullish,
    AcceleratingBearish,
    Decelerating,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, Default, PartialEq)]
pub enum MACDSignal {
    #[default]
    Neutral,
    BullishCross,
    BullishAbove,
    BearishCross,
    BearishBelow,
}

pub fn closes(bars: &[Bar]) -> Vec<f64> { bars.iter().map(|b| b.close).collect() }
pub fn highs(bars: &[Bar]) -> Vec<f64> { bars.iter().map(|b| b.high).collect() }
pub fn lows(bars: &[Bar]) -> Vec<f64> { bars.iter().map(|b| b.low).collect() }
pub fn volumes(bars: &[Bar]) -> Vec<f64> { bars.iter().map(|b| b.volume).collect() }
