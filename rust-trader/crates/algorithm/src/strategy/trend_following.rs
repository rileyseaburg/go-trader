//! Trend Following strategy — rides sustained directional moves.
//!
//! Uses SMA crossover confirmation, ADX trend strength, and directional
//! MACD to identify and ride trends. Avoids entering during low-trend
//! or high-volatility regimes.

use go_trader_indicators::TrendDirection;

use super::{Strategy, StrategyContext, StrategyVote};

/// Detects and rides sustained trends using moving average crossovers,
/// ADX strength filtering, and MACD direction confirmation.
pub struct TrendFollowingStrategy {
    /// Minimum ADX to consider a trend tradeable.
    adx_threshold: f64,
    /// SMA crossover weight in the direction score.
    sma_weight: f64,
    /// MACD direction weight in the direction score.
    macd_weight: f64,
}

impl Default for TrendFollowingStrategy {
    fn default() -> Self {
        Self {
            adx_threshold: 25.0,
            sma_weight: 0.6,
            macd_weight: 0.4,
        }
    }
}

impl TrendFollowingStrategy {
    pub fn new(adx_threshold: f64) -> Self {
        Self {
            adx_threshold,
            ..Self::default()
        }
    }
}

impl Strategy for TrendFollowingStrategy {
    fn name(&self) -> &str {
        "TrendFollowing"
    }

    fn weight(&self) -> f64 {
        0.40
    }

    fn evaluate(&self, ctx: &StrategyContext) -> StrategyVote {
        let ind = ctx.indicators;

        // Trend strength gate
        let adx = ind.adx();
        if adx < self.adx_threshold {
            return StrategyVote::neutral(
                self.name(),
                format!("ADX {:.1} below threshold {:.1}; no actionable trend", adx, self.adx_threshold),
            );
        }

        // Compute directional score from SMA crossover + MACD
        let sma_dir = match ind.sma_trend() {
            TrendDirection::Bullish => 1.0,
            TrendDirection::Bearish => -1.0,
            TrendDirection::Neutral => 0.0,
        };

        let macd_dir = match ind.macd_signal() {
            go_trader_indicators::MACDSignal::BullishCross => 1.0,
            go_trader_indicators::MACDSignal::BearishCross => -1.0,
            go_trader_indicators::MACDSignal::BullishAbove => 0.5,
            go_trader_indicators::MACDSignal::BearishBelow => -0.5,
            go_trader_indicators::MACDSignal::Neutral => 0.0,
        };

        let raw_score = sma_dir * self.sma_weight + macd_dir * self.macd_weight;

        // Scale confidence by ADX strength (25–50 maps to 0.5–1.0)
        let strength_factor = ((adx - self.adx_threshold) / 25.0).clamp(0.0, 1.0);
        let base_confidence = 0.5 + 0.5 * strength_factor;

        // Regime dampening
        let regime_factor = ctx.regime_multiplier;

        if raw_score > 0.3 {
            let confidence = base_confidence * regime_factor;
            StrategyVote::buy(
                self.name(),
                confidence,
                format!(
                    "uptrend: SMA={}, MACD={:.2}, ADX={:.1}, score={:+.2}",
                    match ind.sma_trend() {
                        TrendDirection::Bullish => "bullish",
                        TrendDirection::Bearish => "bearish",
                        TrendDirection::Neutral => "neutral",
                    },
                    ind.macd_value(),
                    adx,
                    raw_score,
                ),
            )
        } else if raw_score < -0.3 {
            let confidence = base_confidence * regime_factor;
            StrategyVote::sell(
                self.name(),
                confidence,
                format!(
                    "downtrend: SMA={}, MACD={:.2}, ADX={:.1}, score={:+.02}",
                    match ind.sma_trend() {
                        TrendDirection::Bullish => "bullish",
                        TrendDirection::Bearish => "bearish",
                        TrendDirection::Neutral => "neutral",
                    },
                    ind.macd_value(),
                    adx,
                    raw_score,
                ),
            )
        } else {
            StrategyVote::neutral(
                self.name(),
                format!("mixed signals: score={:+.02}, ADX={:.1}", raw_score, adx),
            )
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use go_trader_indicators::{IndicatorSet, IndicatorSetBuilder};

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
    fn low_adx_produces_neutral() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(15.0)
            .build();
        let strategy = TrendFollowingStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert_eq!(vote.direction, 0.0);
        assert!(vote.reason.contains("below threshold"));
    }

    #[test]
    fn bullish_trend_with_high_adx() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(35.0)
            .with_sma_trend(TrendDirection::Bullish)
            .with_macd_signal(go_trader_indicators::MACDSignal::BullishCross)
            .with_macd_value(2.5)
            .build();
        let strategy = TrendFollowingStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert!(vote.direction > 0.0, "expected positive direction, got {}", vote.direction);
        assert!(vote.confidence > 0.5);
    }

    #[test]
    fn bearish_trend_with_high_adx() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(40.0)
            .with_sma_trend(TrendDirection::Bearish)
            .with_macd_signal(go_trader_indicators::MACDSignal::BearishCross)
            .with_macd_value(-3.0)
            .build();
        let strategy = TrendFollowingStrategy::default();
        let vote = strategy.evaluate(&make_ctx(&ind));
        assert!(vote.direction < 0.0);
    }

    #[test]
    fn regime_multiplier_dampens_confidence() {
        let ind = IndicatorSetBuilder::new()
            .with_adx(35.0)
            .with_sma_trend(TrendDirection::Bullish)
            .with_macd_signal(go_trader_indicators::MACDSignal::BullishCross)
            .with_macd_value(1.0)
            .build();
        let strategy = TrendFollowingStrategy::default();
        let mut ctx = make_ctx(&ind);
        ctx.regime_multiplier = 0.5;
        let vote = strategy.evaluate(&ctx);
        assert!(vote.direction > 0.0);
        assert!(vote.confidence < 0.6, "confidence should be dampened: got {}", vote.confidence);
    }
}
