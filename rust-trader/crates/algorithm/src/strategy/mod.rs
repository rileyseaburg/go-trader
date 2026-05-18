//! Multi-strategy framework for go-trader.
//!
//! Provides a composable trait-based architecture where each strategy
//! produces a `StrategyVote`, and an `Arbitrator` combines votes into
//! a final trading signal.

mod trend_following;
mod mean_reversion;
mod momentum_breakout;
mod arbitrator;

pub use trend_following::TrendFollowingStrategy;
pub use mean_reversion::MeanReversionStrategy;
pub use momentum_breakout::MomentumBreakoutStrategy;
pub use arbitrator::Arbitrator;

use go_trader_indicators::IndicatorSet;

/// A single strategy's vote on a symbol.
#[derive(Debug, Clone)]
pub struct StrategyVote {
    /// Name of the strategy that produced this vote.
    pub strategy_name: String,
    /// Direction: -1.0 (strong sell) to +1.0 (strong buy), 0.0 = neutral.
    pub direction: f64,
    /// Confidence of this vote: 0.0 to 1.0.
    pub confidence: f64,
    /// Human-readable reason for the vote.
    pub reason: String,
}

impl StrategyVote {
    pub fn neutral(strategy_name: &str, reason: String) -> Self {
        Self {
            strategy_name: strategy_name.to_string(),
            direction: 0.0,
            confidence: 0.0,
            reason,
        }
    }

    pub fn buy(strategy_name: &str, confidence: f64, reason: String) -> Self {
        Self {
            strategy_name: strategy_name.to_string(),
            direction: 1.0,
            confidence: confidence.clamp(0.0, 1.0),
            reason,
        }
    }

    pub fn sell(strategy_name: &str, confidence: f64, reason: String) -> Self {
        Self {
            strategy_name: strategy_name.to_string(),
            direction: -1.0,
            confidence: confidence.clamp(0.0, 1.0),
            reason,
        }
    }
}

/// Context provided to each strategy for evaluation.
pub struct StrategyContext<'a> {
    pub symbol: &'a str,
    pub price: f64,
    pub indicators: &'a IndicatorSet,
    pub regime_multiplier: f64,
    pub has_position: bool,
    pub portfolio_equity: f64,
}

/// Trait for all trading strategies.
pub trait Strategy: Send + Sync {
    /// Name of this strategy.
    fn name(&self) -> &str;

    /// Weight of this strategy's vote (0.0 - 1.0). Higher = more influence.
    fn weight(&self) -> f64;

    /// Evaluate the current market state and produce a vote.
    fn evaluate(&self, ctx: &StrategyContext) -> StrategyVote;
}
