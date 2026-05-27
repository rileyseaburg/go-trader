//! go-trader-indicators: Technical analysis indicators for algorithmic trading.
//!
//! Computes standard technical indicators from OHLCV bar data:
//! - SMA / EMA (Simple & Exponential Moving Averages)
//! - RSI (Relative Strength Index)
//! - MACD (Moving Average Convergence Divergence)
//! - Bollinger Bands
//! - ATR (Average True Range)
//! - VWAP (Volume-Weighted Average Price)
//! - ADX (Average Directional Index)
//! - Stochastic Oscillator (%K / %D)

mod adx;
mod atr;
mod bollinger;
mod builder;
mod composite;
mod macd;
mod moving_average;
mod rsi;
mod stochastic;
mod types;
mod vwap;

pub use adx::{adx, AdxResult};
pub use atr::atr;
pub use bollinger::{bollinger_bands, BollingerResult};
pub use builder::IndicatorSetBuilder;
pub use composite::compute_all;
pub use macd::{macd, MacdResult};
pub use moving_average::{ema, sma};
pub use rsi::rsi;
pub use stochastic::{stochastic, StochasticResult};
pub use types::*;
pub use vwap::vwap;
