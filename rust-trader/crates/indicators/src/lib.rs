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

mod types;
mod moving_average;
mod rsi;
mod macd;
mod bollinger;
mod atr;
mod vwap;
mod adx;
mod stochastic;
mod composite;
mod builder;

pub use types::*;
pub use moving_average::{sma, ema};
pub use rsi::rsi;
pub use macd::{macd, MacdResult};
pub use bollinger::{bollinger_bands, BollingerResult};
pub use atr::atr;
pub use vwap::vwap;
pub use adx::{adx, AdxResult};
pub use stochastic::{stochastic, StochasticResult};
pub use composite::compute_all;
pub use builder::IndicatorSetBuilder;
