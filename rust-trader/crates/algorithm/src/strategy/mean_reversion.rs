//! Mean Reversion strategy — fades overextended moves.
//!
//! Uses RSI extremes, Bollinger Band proximity, and divergence from VWAP
//! to identify when price has moved too far from fair value. Trades the snap-back.

use go_trader_indicators::VolatilityLevel;

use super::{Strategy, StrategyContext, StrategyVote};

const RSI_OVERBOUGHT: f64 = 70.0;
const RSI_OVERSOLD: f64 = 30.0;
const BB_UPPER_PCT: f64 = 0.95; // % of band width from mid to trigger
const BB_LOWER_PCT: f64 = 0.95;

/// Identifies overextended moves and trades mean reversion via
/// RSI extremes and Bollinger Band touches.
pub struct MeanReversionStrategy {
    rsi_upper: f64,
    rsi_lower: f64,
    /// Minimum volatility to trade (BB width as % of price).
    _min_volatility_pct: f64,
}

impl Default for MeanReversionStrategy {
    fn default() -> Self {
        Self {
            rsi_upper: RSI_OVERBOUGHT,
            rsi_lower: RSI_OVERSOLD,
            _min_volatility_pct: 0.01, // 1% BB width minimum
        }
    }
}

impl MeanReversionStrategy {
    pub fn new(rsi_upper: f64, rsi_lower: f64) -> Self {
        Self {
            rsi_upper,
            rsi_lower,
            ..Self::default()
        }
    }

    /// How far price is from the BB midpoint as a fraction of half-width.
    /// Returns +1.0 when at upper band, -1.0 at lower band, 0.0 at mid.
    fn bb_position(&self, ctx: &StrategyContext) -> f64 {
        let ind = ctx.indicators;
        let bb_width = ind.bb_upper() - ind.bb_lower();
        if bb_width.abs() < f64::EPSILON {
            return 0.0;
        }
        let bb_mid = (ind.bb_upper() + ind.bb_lower()) / 2.0;
        let half_width = bb_width / 2.0;
        (ctx.price - bb_mid) / half_width
    }
}

impl Strategy for MeanReversionStrategy {
    fn name(&self) -> &str {
        "MeanReversion"
    }

    fn weight(&self) -> f64 {
        0.35
    }

    fn evaluate(&self, ctx: &StrategyContext) -> StrategyVote {
        let ind = ctx.indicators;
        let rsi = ind.rsi();
        let vol_level = ind.volatility_level();

        // In low-volatility regimes, mean reversion signals are unreliable
        if matches!(vol_level, VolatilityLevel::Low) {
            return StrategyVote::neutral(
                self.name(),
                "low volatility regime — mean reversion signals unreliable".to_string(),
            );
        }

        let bb_pos = self.bb_position(ctx);

        // Check for overbought: RSI above threshold AND price near upper BB
        let rsi_overbought = rsi >= self.rsi_upper;
        let rsi_oversold = rsi <= self.rsi_lower;
        let bb_at_upper = bb_pos >= BB_UPPER_PCT;
        let bb_at_lower = bb_pos <= -BB_LOWER_PCT;

        // Compute a composite score
        let mut score: f64 = 0.0;
        let mut reasons = Vec::new();

        // RSI component (ranges from -1 to +1)
        if rsi_overbought {
            score -= (rsi - self.rsi_upper) / 30.0; // 70→100 maps to 0→1.0 sell
            reasons.push(format!("RSI overbought at {:.1}", rsi));
        } else if rsi_oversold {
            score += (self.rsi_lower - rsi) / 30.0; // 30→0 maps to 0→1.0 buy
            reasons.push(format!("RSI oversold at {:.1}", rsi));
        }

        // BB component
        if bb_at_upper {
            score -= (bb_pos - BB_UPPER_PCT).max(0.0) * 2.0;
            reasons.push(format!("BB upper touch at {:.0}%", bb_pos * 100.0));
        } else if bb_at_lower {
            score += (BB_LOWER_PCT - bb_pos.min(-BB_LOWER_PCT)).max(0.0) * 2.0;
            reasons.push(format!("BB lower touch at {:.0}%", bb_pos * 100.0));
        }

        // Volatility bonus — higher vol = stronger reversion potential
        let vol_bonus = match vol_level {
            VolatilityLevel::High => 0.1,
            VolatilityLevel::Extreme => 0.15,
            VolatilityLevel::Elevated => 0.05,
            VolatilityLevel::Normal => 0.0,
            VolatilityLevel::Low => -0.1,
        };
        score += score.signum() * vol_bonus;

        let regime_factor = ctx.regime_multiplier;

        if score > 0.2 {
            let confidence = (0.5 + score * 0.3).min(1.0) * regime_factor;
            StrategyVote::buy(
                self.name(),
                confidence,
                format!("oversold reversion: {}", reasons.join(", ")),
            )
        } else if score < -0.2 {
            let confidence = (0.5 + score.abs() * 0.3).min(1.0) * regime_factor;
            StrategyVote::sell(
                self.name(),
                confidence,
                format!("overbought reversion: {}", reasons.join(", ")),
            )
        } else {
            StrategyVote::neutral(
                self.name(),
                format!("no extreme levels: RSI={:.1}, BB pos={:+.0}%", rsi, bb_pos * 100.0),
            )
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use go_trader_indicators::{IndicatorSet, IndicatorSetBuilder, VolatilityLevel};

    fn make_ctx(indicators: &IndicatorSet) -> StrategyContext<'_> {
        StrategyContext {
            symbol: "AAPL",
            price: 150.0,
            indicators,
            regime_multiplier: 1.0,
            has_position: false,
            portfolio_equity: 100_000.0,
        }
    }

    #[test]
    fn rsi_overbought_with_bb_upper_triggers_sell() {
        let ind = IndicatorSetBuilder::new()
            .with_rsi(78.0)
            .with_bb(145.0, 155.0) // price 150 is at upper band
            .with_volatility_level(VolatilityLevel::Elevated)
            .build();
        let strategy = MeanReversionStrategy::default();
        // Set price at the upper BB
        let mut ctx = make_ctx(&ind);
        ctx.price = 154.5; // near upper band
        let vote = strategy.evaluate(&ctx);
        assert!(vote.direction < 0.0, "expected sell, got direction={}", vote.direction);
    }

    #[test]
    fn rsi_oversold_with_bb_lower_triggers_buy() {
        let ind = IndicatorSetBuilder::new()
            .with_rsi(22.0)
            .with_bb(145.0, 155.0)
            .with_volatility_level(VolatilityLevel::Elevated)
            .build();
        let strategy = MeanReversionStrategy::default();
        let mut ctx = make_ctx(&ind);
        ctx.price = 145.5; // near lower band
        let vote = strategy.evaluate(&ctx);
        assert!(vote.direction > 0.0, "expected buy, got direction={}", vote.direction);
    }

    #[test]
    fn low_volatility_is_neutral() {
        let ind = IndicatorSetBuilder::new()
            .with_rsi(75.0)
            .with_volatility_level(VolatilityLevel::Low)
            .build();
        let strategy = MeanReversionStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert_eq!(vote.direction, 0.0);
        assert!(vote.reason.contains("low volatility"));
    }

    #[test]
    fn neutral_rsi_and_mid_band_is_neutral() {
        let ind = IndicatorSetBuilder::new()
            .with_rsi(50.0)
            .with_bb(145.0, 155.0)
            .with_volatility_level(VolatilityLevel::Normal)
            .build();
        let strategy = MeanReversionStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert_eq!(vote.direction, 0.0);
    }
}
