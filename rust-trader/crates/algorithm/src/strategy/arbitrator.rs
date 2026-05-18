//! Arbitrator — combines votes from multiple strategies into a final trading signal.
//!
//! Uses weighted confidence voting to aggregate strategy opinions. Each strategy
//! contributes a direction × confidence × weight score. The net score determines
//! the final signal direction and confidence.

use chrono::Utc;
use go_trader_types::{MarketData, TradeSignal, SIGNAL_BUY, SIGNAL_HOLD, SIGNAL_SELL};

use super::{Strategy, StrategyContext, StrategyVote};
use crate::types::{PortfolioData, RiskParameters};

const BUY_THRESHOLD: f64 = 0.15;
const SELL_THRESHOLD: f64 = -0.15;
const MIN_CONFIDENCE_FOR_ACTION: f64 = 0.55;

/// Combines weighted votes from multiple strategies into a final `TradeSignal`.
pub struct Arbitrator {
    strategies: Vec<Box<dyn Strategy>>,
}

impl Default for Arbitrator {
    fn default() -> Self {
        Self::new_default()
    }
}

impl Arbitrator {
    pub fn new_default() -> Self {
        use super::{MeanReversionStrategy, MomentumBreakoutStrategy, TrendFollowingStrategy};
        Self {
            strategies: vec![
                Box::new(TrendFollowingStrategy::default()),
                Box::new(MeanReversionStrategy::default()),
                Box::new(MomentumBreakoutStrategy::default()),
            ],
        }
    }

    pub fn with_strategies(strategies: Vec<Box<dyn Strategy>>) -> Self {
        Self { strategies }
    }

    pub fn collect_votes(&self, ctx: &StrategyContext) -> Vec<StrategyVote> {
        self.strategies.iter().map(|s| s.evaluate(ctx)).collect()
    }

    /// Returns (net_direction, weighted_confidence, votes).
    pub fn aggregate(&self, ctx: &StrategyContext) -> (f64, f64, Vec<StrategyVote>) {
        let votes = self.collect_votes(ctx);
        let mut weighted_buy = 0.0_f64;
        let mut weighted_sell = 0.0_f64;
        let mut total_weight = 0.0_f64;

        for vote in &votes {
            let weight = self
                .strategies
                .iter()
                .find(|s| s.name() == vote.strategy_name)
                .map(|s| s.weight())
                .unwrap_or(1.0);

            let contribution = vote.direction * vote.confidence * weight;
            if contribution > 0.0 {
                weighted_buy += contribution;
            } else if contribution < 0.0 {
                weighted_sell += contribution.abs();
            }
            total_weight += weight;
        }

        if total_weight < f64::EPSILON {
            return (0.0, 0.0, votes);
        }

        let net = (weighted_buy - weighted_sell) / total_weight;
        let confidence = if net > 0.0 {
            weighted_buy / total_weight
        } else {
            weighted_sell / total_weight
        };

        (net, confidence, votes)
    }

    /// Produce a final `TradeSignal` from the aggregated vote.
    pub fn generate_signal(
        &self,
        symbol: &str,
        market_data: &MarketData,
        portfolio: &PortfolioData,
        _risk: &RiskParameters,
        regime_name: &str,
        regime_multiplier: f64,
        indicators: &go_trader_indicators::IndicatorSet,
    ) -> TradeSignal {
        let has_position = portfolio
            .positions
            .get(symbol)
            .map(|p| p.quantity.abs() > f64::EPSILON)
            .unwrap_or(false);

        let ctx = StrategyContext {
            symbol,
            price: market_data.price,
            indicators,
            regime_multiplier,
            has_position,
            portfolio_equity: portfolio.total_value,
        };

        let (net_direction, confidence, votes) = self.aggregate(&ctx);

        let vote_summary: Vec<String> = votes
            .iter()
            .map(|v| format!("{}: {} ({:.0}%)", v.strategy_name, v.reason, v.confidence * 100.0))
            .collect();

        let (signal, final_confidence, reason_prefix) =
            if net_direction > BUY_THRESHOLD && confidence >= MIN_CONFIDENCE_FOR_ACTION {
                (
                    SIGNAL_BUY,
                    confidence,
                    format!("MULTI-STRATEGY BUY (net={:+.02}, conf={:.0}%): ", net_direction, confidence * 100.0),
                )
            } else if net_direction < SELL_THRESHOLD && confidence >= MIN_CONFIDENCE_FOR_ACTION {
                (
                    SIGNAL_SELL,
                    confidence,
                    format!("MULTI-STRATEGY SELL (net={:+.02}, conf={:.0}%): ", net_direction, confidence * 100.0),
                )
            } else {
                (
                    SIGNAL_HOLD,
                    confidence.max(0.45),
                    format!("MULTI-STRATEGY HOLD (net={:+.02}, conf={:.0}%): ", net_direction, confidence * 100.0),
                )
            };

        let regime_label = if regime_name.is_empty() { "UNSET" } else { regime_name };
        let reasoning = format!(
            "{}regime={} ×{:.2}; votes: [{}]",
            reason_prefix, regime_label, regime_multiplier,
            vote_summary.join("; "),
        );

        TradeSignal {
            symbol: symbol.into(),
            signal: signal.into(),
            order_type: "limit".into(),
            limit_price: if signal == SIGNAL_BUY || signal == SIGNAL_SELL {
                Some(market_data.price)
            } else {
                None
            },
            timestamp: Utc::now(),
            reasoning: format!(
                "Arbitrator: {}; source=multi_strategy; strategies={}",
                reasoning, self.strategies.len()
            ),
            confidence: Some(final_confidence),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use go_trader_indicators::{
        IndicatorSetBuilder, MACDSignal, MomentumState, TrendDirection, VolatilityLevel,
    };

    fn md(price: f64) -> MarketData {
        MarketData {
            symbol: "AAPL".into(),
            price,
            high_24h: price * 1.05,
            low_24h: price * 0.95,
            volume_24h: 1_000_000.0,
            change_24h: 0.02,
        }
    }

    #[test]
    fn strong_bullish_consensus_triggers_buy() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(35.0)
            .with_sma_trend(TrendDirection::Bullish)
            .with_macd_signal(MACDSignal::BullishCross)
            .with_macd_value(3.0)
            .with_rsi(25.0)
            .with_bb(95.0, 110.0)
            .with_volatility_level(VolatilityLevel::Elevated)
            .with_momentum_state(MomentumState::AcceleratingBullish)
            .with_volume_ratio(2.5)
            .build();

        let arb = Arbitrator::new_default();
        let signal = arb.generate_signal(
            "AAPL",
            &md(100.0),
            &PortfolioData::default(),
            &RiskParameters::default(),
            "RISING WATERS",
            1.0,
            &ind,
        );
        assert_eq!(signal.signal, SIGNAL_BUY, "reasoning: {}", signal.reasoning);
        assert!(signal.confidence.unwrap() > 0.5);
    }

    #[test]
    fn mixed_signals_produce_hold() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(20.0)
            .with_sma_trend(TrendDirection::Neutral)
            .with_macd_signal(MACDSignal::Neutral)
            .with_macd_value(0.0)
            .with_rsi(50.0)
            .with_bb(95.0, 105.0)
            .with_volatility_level(VolatilityLevel::Normal)
            .with_momentum_state(MomentumState::Neutral)
            .with_volume_ratio(1.0)
            .build();

        let arb = Arbitrator::new_default();
        let signal = arb.generate_signal(
            "AAPL",
            &md(100.0),
            &PortfolioData::default(),
            &RiskParameters::default(),
            "NEUTRAL",
            0.85,
            &ind,
        );
        assert_eq!(signal.signal, SIGNAL_HOLD, "reasoning: {}", signal.reasoning);
    }

    #[test]
    fn bearish_consensus_triggers_sell() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(38.0)
            .with_sma_trend(TrendDirection::Bearish)
            .with_macd_signal(MACDSignal::BearishCross)
            .with_macd_value(-2.5)
            .with_rsi(78.0)
            .with_bb(140.0, 155.0)
            .with_volatility_level(VolatilityLevel::Elevated)
            .with_momentum_state(MomentumState::AcceleratingBearish)
            .with_volume_ratio(2.0)
            .build();

        let arb = Arbitrator::new_default();
        let signal = arb.generate_signal(
            "AAPL",
            &md(154.0),
            &PortfolioData::default(),
            &RiskParameters::default(),
            "EBBING TIDE",
            0.9,
            &ind,
        );
        assert_eq!(signal.signal, SIGNAL_SELL, "reasoning: {}", signal.reasoning);
    }

    #[test]
    fn defensive_regime_dampens_buy() {
        // Strong bullish indicators but defensive regime
        let ind = IndicatorSetBuilder::new()
            .with_adx(40.0)
            .with_sma_trend(TrendDirection::Bullish)
            .with_macd_signal(MACDSignal::BullishCross)
            .with_macd_value(4.0)
            .with_rsi(28.0)
            .with_bb(90.0, 110.0)
            .with_volatility_level(VolatilityLevel::Elevated)
            .with_momentum_state(MomentumState::AcceleratingBullish)
            .with_volume_ratio(3.0)
            .build();

        let arb = Arbitrator::new_default();
        let signal = arb.generate_signal(
            "AAPL",
            &md(100.0),
            &PortfolioData::default(),
            &RiskParameters::default(),
            "EBBING TIDE",
            0.5, // defensive
            &ind,
        );
        // Even with bullish indicators, defensive regime dampens confidence
        // The signal direction is still based on indicator consensus
        assert!(signal.confidence.unwrap() < 0.8, "confidence should be dampened");
    }
}
