//! Micro scalper strategy — reacts quickly to short-horizon momentum/mean-reversion edges.
//!
//! This is intended for Alpaca API software trading: it is aggressive enough to
//! generate actionable orders from live stream updates, while still respecting
//! global risk gates and existing position checks in the arbitrator/executor.

use go_trader_indicators::{MACDSignal, MomentumState, TrendDirection};

use super::{Strategy, StrategyContext, StrategyVote};

/// Fast, low-threshold strategy for live stream-driven evaluation.
pub struct MicroScalperStrategy;

impl Default for MicroScalperStrategy {
    fn default() -> Self {
        Self
    }
}

impl Strategy for MicroScalperStrategy {
    fn name(&self) -> &str {
        "MicroScalper"
    }

    fn weight(&self) -> f64 {
        1.25
    }

    fn evaluate(&self, ctx: &StrategyContext) -> StrategyVote {
        let ind = ctx.indicators;
        let mut score = 0.0_f64;
        let mut reasons = Vec::new();

        match ind.momentum_state() {
            MomentumState::AcceleratingBullish => {
                score += 0.45;
                reasons.push("accelerating bullish momentum");
            }
            MomentumState::Rising => {
                score += 0.25;
                reasons.push("rising momentum");
            }
            MomentumState::AcceleratingBearish => {
                score -= 0.45;
                reasons.push("accelerating bearish momentum");
            }
            MomentumState::Falling => {
                score -= 0.25;
                reasons.push("falling momentum");
            }
            MomentumState::Oversold => {
                score += 0.20;
                reasons.push("oversold snapback setup");
            }
            MomentumState::Overbought => {
                score -= 0.20;
                reasons.push("overbought fade setup");
            }
            _ => {}
        }

        match ind.sma_trend() {
            TrendDirection::Bullish => {
                score += 0.22;
                reasons.push("bullish SMA tape");
            }
            TrendDirection::Bearish => {
                score -= 0.22;
                reasons.push("bearish SMA tape");
            }
            TrendDirection::Neutral => {}
        }

        match ind.macd_signal() {
            MACDSignal::BullishAbove | MACDSignal::BullishCross => {
                score += 0.22;
                reasons.push("bullish MACD");
            }
            MACDSignal::BearishBelow | MACDSignal::BearishCross => {
                score -= 0.22;
                reasons.push("bearish MACD");
            }
            MACDSignal::Neutral => {}
        }

        let rsi = ind.rsi();
        if rsi <= 32.0 {
            score += 0.18;
            reasons.push("RSI oversold");
        } else if rsi >= 68.0 {
            score -= 0.18;
            reasons.push("RSI overbought");
        }

        let vol_ratio = ind.volume_ratio();
        if vol_ratio >= 1.25 {
            score *= 1.15;
            reasons.push("above-average volume");
        }
        if vol_ratio >= 1.8 {
            score *= 1.10;
            reasons.push("volume surge");
        }

        // Keep sells available for exits/short-capable configs, but avoid adding
        // to already-held long exposure unless the edge is very strong.
        if ctx.has_position && score > 0.0 {
            score *= 0.55;
            reasons.push("already in position; add dampened");
        }

        let confidence =
            (0.62 + score.abs() * 0.55).clamp(0.0, 0.96) * ctx.regime_multiplier.max(0.5);
        let reason = format!(
            "micro edge score={:+.2}, rsi={:.1}, vol_ratio={:.2}x, price={:.2}; {}",
            score,
            rsi,
            vol_ratio,
            ctx.price,
            if reasons.is_empty() {
                "no fast edge".to_string()
            } else {
                reasons.join(", ")
            }
        );

        if score >= 0.28 {
            StrategyVote::buy(self.name(), confidence, reason)
        } else if score <= -0.28 {
            StrategyVote::sell(self.name(), confidence, reason)
        } else {
            StrategyVote::neutral(self.name(), reason)
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
    fn bullish_fast_edge_is_actionable() {
        let ind = IndicatorSetBuilder::new()
            .with_sma_trend(TrendDirection::Bullish)
            .with_macd_signal(MACDSignal::BullishAbove)
            .with_momentum_state(MomentumState::Rising)
            .with_volume_ratio(1.5)
            .with_rsi(45.0)
            .build();
        let vote = MicroScalperStrategy.evaluate(&make_ctx(&ind));
        assert!(vote.direction > 0.0);
        assert!(vote.confidence >= 0.65, "confidence={}", vote.confidence);
    }

    #[test]
    fn bearish_fast_edge_is_actionable() {
        let ind = IndicatorSetBuilder::new()
            .with_sma_trend(TrendDirection::Bearish)
            .with_macd_signal(MACDSignal::BearishBelow)
            .with_momentum_state(MomentumState::Falling)
            .with_volume_ratio(1.5)
            .with_rsi(55.0)
            .build();
        let vote = MicroScalperStrategy.evaluate(&make_ctx(&ind));
        assert!(vote.direction < 0.0);
        assert!(vote.confidence >= 0.65, "confidence={}", vote.confidence);
    }
}
