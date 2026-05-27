//! go-trader-backtest: Historical backtesting engine for strategy validation.

pub mod engine;
pub mod report;

pub use engine::{BacktestConfig, BacktestEngine};
pub use report::{BacktestReport, TradeRecord, TradeSide};
