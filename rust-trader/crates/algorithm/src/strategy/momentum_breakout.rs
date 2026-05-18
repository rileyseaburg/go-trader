//! Momentum Breakout strategy — detects explosive moves from consolidation.
//!
//! Uses volume surge detection, price breaking above/below Bollinger Bands
//! or recent highs/lows, and momentum state to trade breakouts.

use go_trader_indicators::{MomentumState, VolatilityLevel};

use super::{Strategy, StrategyContext, StrategyVote};

const VOLUME_SURGE_RATIO: f64 = 1.8; // Current vol > 1.8x average

/// Trades breakouts from consolidation ranges using volume confirmation
/// and momentum acceleration.
pub struct MomentumBreakoutStrategy {
    /// Minimum volume surge ratio to confirm a breakout.
    volume_ratio_threshold: f64,
    /// Minimum momentum to confirm direction.
    min_momentum_score: f64,
}

impl Default for MomentumBreakoutStrategy {
    fn default() -> Self {
        Self {
            volume_ratio_threshold: VOLUME_SURGE_RATIO,
            min_momentum_score: 0.3,
        }
    }
}

impl MomentumBreakoutStrategy {
    pub fn new(volume_ratio_threshold: f64) -> Self {
        Self {
            volume_ratio_threshold,
            ..Self::default()
        }
    }
}

impl Strategy for MomentumBreakoutStrategy {
    fn name(&self) -> &str {
        "MomentumBreakout"
    }

    fn weight(&self) -> f64 {
        0.25
    }

    fn evaluate(&self, ctx: &StrategyContext) -> StrategyVote {
        let ind = ctx.indicators;

        let momentum: f64 = match ind.momentum_state() {
            MomentumState::AcceleratingBullish => 1.0,
            MomentumState::AcceleratingBearish => -1.0,
            MomentumState::Rising => 0.5,
            MomentumState::Falling => -0.5,
            MomentumState::Decelerating => 0.0,
            MomentumState::Overbought => 0.3,
            MomentumState::Oversold => -0.3,
            MomentumState::Neutral => 0.0,
        };

        // Volume confirmation
        let vol_ratio = ind.volume_ratio();
        let volume_confirmed = vol_ratio >= self.volume_ratio_threshold;

        // BB breakout detection
        let bb_breakout_up = ctx.price > ind.bb_upper();
        let bb_breakout_down = ctx.price < ind.bb_lower();

        let vol_level = ind.volatility_level();

        // Construct score
        let mut score = 0.0_f64;
        let mut reasons = Vec::new();

        // Momentum direction
        if momentum.abs() >= self.min_momentum_score {
            score += momentum * 0.5;
            reasons.push(format!("momentum={:+.1}", momentum));
        }

        // BB breakout bonus
        if bb_breakout_up && momentum > 0.0 {
            score += 0.4;
            reasons.push("BB upper breakout".to_string());
        } else if bb_breakout_down && momentum < 0.0 {
            score -= 0.4;
            reasons.push("BB lower breakout".to_string());
        }

        // Volume confirmation amplifies the signal
        if volume_confirmed && score.abs() > 0.1 {
            score *= 1.3;
            reasons.push(format!("volume surge {:.1}x", vol_ratio));
        }

        // In high volatility, breakouts are more likely to be real
        let vol_factor = match vol_level {
            VolatilityLevel::High => 1.1,
            VolatilityLevel::Elevated => 1.0,
            VolatilityLevel::Normal => 0.9,
            VolatilityLevel::Low => 0.7,
            VolatilityLevel::Extreme => 1.2,
        };
        score *= vol_factor;

        let regime_factor = ctx.regime_multiplier;

        if score > 0.3 {
            let confidence = (0.5 + score * 0.2).min(0.95) * regime_factor;
            StrategyVote::buy(
                self.name(),
                confidence,
                format!("bullish breakout: {}", reasons.join(", ")),
            )
        } else if score < -0.3 {
            let confidence = (0.5 + score.abs() * 0.2).min(0.95) * regime_factor;
            StrategyVote::sell(
                self.name(),
                confidence,
                format!("bearish breakout: {}", reasons.join(", ")),
            )
        } else {
            StrategyVote::neutral(
                self.name(),
                format!(
                    "no breakout signal: momentum={:+.1}, vol_ratio={:.1}x, score={:+.02}",
                    momentum, vol_ratio, score
                ),
            )
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use go_trader_indicators::{IndicatorSet, IndicatorSetBuilder, MomentumState, VolatilityLevel};

    fn make_ctx(indicators: &IndicatorSet) -> StrategyContext<'_> {
        StrategyContext {
            symbol: "TSLA",
            price: 250.0,
            indicators,
            regime_multiplier: 1.0,
            has_position: false,
            portfolio_equity: 100_000.0,
        }
    }

    #[test]
    fn accelerating_bullish_with_volume_triggers_buy() {
        let ind = IndicatorSetBuilder::new()
            .with_momentum_state(MomentumState::AcceleratingBullish)
            .with_volume_ratio(2.5)
            .with_bb(240.0, 260.0) // price 250 at mid, no breakout
            .with_volatility_level(VolatilityLevel::Elevated)
            .build();
        let strategy = MomentumBreakoutStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert!(vote.direction > 0.0, "expected buy, got direction={}", vote.direction);
        assert!(vote.reason.contains("volume surge"));
    }

    #[test]
    fn bb_breakout_up_with_momentum_triggers_buy() {
        let ind = IndicatorSetBuilder::new()
            .with_momentum_state(MomentumState::AcceleratingBullish)
            .with_volume_ratio(1.5)
            .with_bb(240.0, 258.0) // price 250 is between, but not breakout
            .with_volatility_level(VolatilityLevel::High)
            .build();
        let mut ctx = make_ctx(&ind);
        ctx.price = 260.0; // Above upper BB
        let strategy = MomentumBreakoutStrategy::default();
        let vote = strategy.evaluate(&ctx);
        assert!(vote.direction > 0.0);
        assert!(vote.reason.contains("BB upper breakout"));
    }

    #[test]
    fn neutral_momentum_is_neutral() {
        let ind = IndicatorSetBuilder::new()
            .with_momentum_state(MomentumState::Neutral)
            .with_volume_ratio(1.0)
            .with_bb(240.0, 260.0)
            .with_volatility_level(VolatilityLevel::Normal)
            .build();
        let strategy = MomentumBreakoutStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert_eq!(vote.direction, 0.0);
    }

    #[test]
    fn bearish_breakout_triggers_sell() {
        let ind = IndicatorSetBuilder::new()
            .with_momentum_state(MomentumState::AcceleratingBearish)
            .with_volume_ratio(2.0)
            .with_bb(240.0, 260.0)
            .with_volatility_level(VolatilityLevel::Elevated)
            .build();
        let mut ctx = make_ctx(&ind);
        ctx.price = 238.0; // Below lower BB
        let strategy = MomentumBreakoutStrategy::default();
        let vote = strategy.evaluate(&ctx);
        assert!(vote.direction < 0.0);
        assert!(vote.reason.contains("BB lower breakout"));
    }
}
