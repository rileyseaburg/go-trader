mod algorithm;
mod local_signal;
mod model_signal;
mod strategy;
mod types;
pub use algorithm::TradingAlgorithm;
pub use local_signal::RuleBasedSignalEngine;
pub use model_signal::ModelSignalClient;
pub use strategy::{
    Arbitrator, MeanReversionStrategy, MicroScalperStrategy, MomentumBreakoutStrategy, Strategy,
    StrategyContext, StrategyVote, TapeVelocityStrategy, TrendFollowingStrategy,
};
pub use types::*;
