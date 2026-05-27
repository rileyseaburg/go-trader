//! Tape velocity strategy — fast intrabar momentum from price vs EMA/VWAP/BB location.
//!
//! This strategy is designed for live stream evaluation. It does not replace
//! slower confirmation strategies; it adds a high-weight vote when the current
//! tick price is moving away from fair value with volume confirmation.

use go_trader_indicators::{MACDSignal, MomentumState, TrendDirection};

use super::{Strategy, StrategyContext, StrategyVote};

/// Fast price-location/velocity strategy for live paper trading.
pub struct TapeVelocityStrategy;

impl Default for TapeVelocityStrategy {
    fn default() -> Self {
        Self
    }
}

impl Strategy for TapeVelocityStrategy {
    fn name(&self) -> &str {
        "TapeVelocity"
    }

    fn weight(&self) -> f64 {
        1.60
    }

    fn evaluate(&self, ctx: &StrategyContext) -> StrategyVote {
        let ind = ctx.indicators;
        let price = ctx.price;
        if price <= f64::EPSILON {
            return StrategyVote::neutral(self.name(), "missing live price".to_string());
        }

        let mut score = 0.0_f64;
        let mut reasons = Vec::new();

        if let Some(ema_9) = ind.ema_9.filter(|v| *v > f64::EPSILON) {
            let pct = (price - ema_9) / ema_9 * 100.0;
            if pct >= 0.08 {
                score += (pct / 0.35).clamp(0.10, 0.38);
                reasons.push(format!("price {pct:+.2}% above EMA9"));
            } else if pct <= -0.08 {
                score -= ((-pct) / 0.35).clamp(0.10, 0.38);
                reasons.push(format!("price {pct:+.2}% below EMA9"));
            }
        }

        if let Some(vwap) = ind.vwap.filter(|v| *v > f64::EPSILON) {
            let pct = (price - vwap) / vwap * 100.0;
            if pct >= 0.10 {
                score += (pct / 0.45).clamp(0.10, 0.32);
                reasons.push(format!("price {pct:+.2}% above VWAP"));
            } else if pct <= -0.10 {
                score -= ((-pct) / 0.45).clamp(0.10, 0.32);
                reasons.push(format!("price {pct:+.2}% below VWAP"));
            }
        }

        if let Some(percent_b) = ind.bb_percent_b {
            if percent_b >= 0.82 {
                score += ((percent_b - 0.80) * 0.45).clamp(0.05, 0.22);
                reasons.push(format!("BB %B high {:.2}", percent_b));
            } else if percent_b <= 0.18 {
                score -= ((0.20 - percent_b) * 0.45).clamp(0.05, 0.22);
                reasons.push(format!("BB %B low {:.2}", percent_b));
            }
        }

        match ind.momentum_state() {
            MomentumState::AcceleratingBullish => {
                score += 0.24;
                reasons.push("accelerating bullish tape".to_string());
            }
            MomentumState::Rising => {
                score += 0.14;
                reasons.push("rising tape".to_string());
            }
            MomentumState::AcceleratingBearish => {
                score -= 0.24;
                reasons.push("accelerating bearish tape".to_string());
            }
            MomentumState::Falling => {
                score -= 0.14;
                reasons.push("falling tape".to_string());
            }
            _ => {}
        }

        match ind.macd_signal() {
            MACDSignal::BullishAbove | MACDSignal::BullishCross => score += 0.08,
            MACDSignal::BearishBelow | MACDSignal::BearishCross => score -= 0.08,
            MACDSignal::Neutral => {}
        }

        match ind.sma_trend() {
            TrendDirection::Bullish if score > 0.0 => score *= 1.10,
            TrendDirection::Bearish if score < 0.0 => score *= 1.10,
            TrendDirection::Bullish if score < 0.0 => score *= 0.82,
            TrendDirection::Bearish if score > 0.0 => score *= 0.82,
            _ => {}
        }

        let vol_ratio = ind.volume_ratio();
        if vol_ratio >= 1.15 {
            score *= 1.12;
            reasons.push(format!("volume {:.2}x" , vol_ratio));
        }
        if vol_ratio >= 1.75 {
            score *= 1.10;
            reasons.push("volume expansion".to_string());
        }

        // Do not pyramid aggressively into existing long exposure; exits remain
        // fully available because negative scores are not dampened here.
        if ctx.has_position && score > 0.0 {
            score *= 0.50;
            reasons.push("existing position; buy velocity dampened".to_string());
        }

        let confidence = (0.58 + score.abs() * 0.62).clamp(0.0, 0.97)
            * ctx.regime_multiplier.max(0.50);
        let reason = format!(
            "tape velocity score={score:+.2}, price={price:.2}; {}",
            if reasons.is_empty() {
                "no fast tape edge".to_string()
            } else {
                reasons.join(", ")
            }
        );

        if score >= 0.22 {
            StrategyVote::buy(self.name(), confidence, reason)
        } else if score <= -0.22 {
            StrategyVote::sell(self.name(), confidence, reason)
        } else {
            StrategyVote::neutral(self.name(), reason)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use go_trader_indicators::{IndicatorSet, IndicatorSetBuilder, VolatilityLevel};

    fn base_ctx(indicators: &IndicatorSet, price: f64, has_position: bool) -> StrategyContext<'_> {
        StrategyContext {
            symbol: "MSFT",
            price,
            indicators,
            regime_multiplier: 1.0,
            has_position,
            portfolio_equity: 100_000.0,
        }
    }

    #[test]
    fn bullish_tape_velocity_is_actionable() {
        let mut ind = IndicatorSetBuilder::new()
            .with_sma_trend(TrendDirection::Bullish)
            .with_macd_signal(MACDSignal::BullishAbove)
            .with_momentum_state(MomentumState::AcceleratingBullish)
            .with_volume_ratio(1.6)
            .with_volatility_level(VolatilityLevel::Elevated)
            .build();
        ind.ema_9 = Some(100.0);
        ind.vwap = Some(99.9);
        ind.bb_percent_b = Some(0.88);

        let vote = TapeVelocityStrategy.evaluate(&base_ctx(&ind, 100.35, false));
        assert!(vote.direction > 0.0, "vote={vote:?}");
        assert!(vote.confidence >= 0.70, "confidence={}", vote.confidence);
    }

    #[test]
    fn bearish_tape_velocity_is_actionable_for_exits() {
        let mut ind = IndicatorSetBuilder::new()
            .with_sma_trend(TrendDirection::Bearish)
            .with_macd_signal(MACDSignal::BearishBelow)
            .with_momentum_state(MomentumState::AcceleratingBearish)
            .with_volume_ratio(1.6)
            .build();
        ind.ema_9 = Some(100.0);
        ind.vwap = Some(100.1);
        ind.bb_percent_b = Some(0.12);

        let vote = TapeVelocityStrategy.evaluate(&base_ctx(&ind, 99.65, true));
        assert!(vote.direction < 0.0, "vote={vote:?}");
        assert!(vote.confidence >= 0.70, "confidence={}", vote.confidence);
    }
}
