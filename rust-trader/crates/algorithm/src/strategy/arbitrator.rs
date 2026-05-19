//! Arbitrator — combines votes from multiple strategies into a final trading signal.
//!
//! Uses weighted confidence voting to aggregate strategy opinions. Each strategy
//! contributes a direction × confidence × weight score. The net score determines
//! the final signal direction and confidence.

use chrono::Utc;
use go_trader_types::{MarketData, TradeSignal, SIGNAL_BUY, SIGNAL_HOLD, SIGNAL_SELL};
use serde_json::json;

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
        risk: &RiskParameters,
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

        let (raw_net, confidence, votes) = self.aggregate(&ctx);
        // Strategy implementations already apply regime-aware confidence dampening.
        // Keep both names explicit so the API exposes the full scoring pipeline.
        let adjusted_net = raw_net;

        let vote_summary: Vec<String> = votes
            .iter()
            .map(|v| format!("{}: {} ({:.0}%)", v.strategy_name, v.reason, v.confidence * 100.0))
            .collect();

        let (pre_risk_signal, final_confidence, reason_prefix) =
            if adjusted_net > BUY_THRESHOLD && confidence >= MIN_CONFIDENCE_FOR_ACTION {
                (
                    SIGNAL_BUY,
                    confidence,
                    format!("MULTI-STRATEGY BUY (net={:+.02}, conf={:.0}%): ", adjusted_net, confidence * 100.0),
                )
            } else if adjusted_net < SELL_THRESHOLD && confidence >= MIN_CONFIDENCE_FOR_ACTION {
                (
                    SIGNAL_SELL,
                    confidence,
                    format!("MULTI-STRATEGY SELL (net={:+.02}, conf={:.0}%): ", adjusted_net, confidence * 100.0),
                )
            } else {
                (
                    SIGNAL_HOLD,
                    confidence,
                    format!("MULTI-STRATEGY HOLD (net={:+.02}, conf={:.0}%): ", adjusted_net, confidence * 100.0),
                )
            };

        let risk_gate = risk_gate_json(
            pre_risk_signal,
            confidence,
            adjusted_net,
            portfolio,
            symbol,
            risk,
            MIN_CONFIDENCE_FOR_ACTION,
        );
        let blocks_order = risk_gate
            .get("blocks_order")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        let signal = if blocks_order { SIGNAL_HOLD } else { pre_risk_signal };

        let regime_label = if regime_name.is_empty() { "UNSET" } else { regime_name };
        let reasoning = format!(
            "{}regime={} ×{:.2}; votes: [{}]",
            reason_prefix, regime_label, regime_multiplier,
            vote_summary.join("; "),
        );

        let vote_audit: Vec<_> = votes.iter().map(|v| {
            let weight = self.strategies.iter().find(|s| s.name() == v.strategy_name).map(|s| s.weight()).unwrap_or(1.0);
            json!({
                "strategy": v.strategy_name,
                "direction": v.direction,
                "confidence": v.confidence,
                "weight": weight,
                "weighted_score": v.direction * v.confidence * weight,
                "reason": v.reason,
            })
        }).collect();

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
                "Arbitrator: {}; source=multi_strategy; strategies={}; action_thresholds=buy>{:.2}/sell<{:.2}/conf>={:.0}%{}",
                reasoning,
                self.strategies.len(),
                BUY_THRESHOLD,
                SELL_THRESHOLD,
                MIN_CONFIDENCE_FOR_ACTION * 100.0,
                if blocks_order { "; risk_gate=blocked" } else { "" }
            ),
            confidence: Some(final_confidence),
            audit: Some(json!({
                "pipeline": "multi_strategy_arbitrator",
                "raw_strategy_scores": vote_audit,
                "normalized_scores": {
                    "raw_net": raw_net,
                    "adjusted_net": adjusted_net,
                    "canonical_confidence": final_confidence,
                    "total_strategy_weight": self.strategies.iter().map(|s| s.weight()).sum::<f64>()
                },
                "regime_adjustment": {
                    "regime_name": regime_label,
                    "regime_multiplier": regime_multiplier,
                    "note": "strategy confidences are regime-adjusted before aggregation"
                },
                "confidence_calculation": {
                    "canonical_confidence": final_confidence,
                    "source": "same weighted aggregate displayed in reasoning text"
                },
                "risk_units": risk_units_json(risk),
                "risk_gate": risk_gate,
                "action_threshold": {
                    "buy_net_gt": BUY_THRESHOLD,
                    "sell_net_lt": SELL_THRESHOLD,
                    "min_confidence_for_action": MIN_CONFIDENCE_FOR_ACTION,
                    "auto_trade_min_confidence": 0.65
                },
                "order_sizing": order_sizing_json(market_data.price, portfolio, risk, regime_multiplier, final_confidence),
                "execution_decision": {
                    "pre_risk_action": pre_risk_signal,
                    "final_action": signal,
                    "limit_price": if signal == SIGNAL_BUY || signal == SIGNAL_SELL { Some(market_data.price) } else { None }
                }
            })),
        }
    }
}

fn risk_units_json(risk: &RiskParameters) -> serde_json::Value {
    json!({
        "max_position_size_percent": { "value": risk.max_position_size_percent, "unit": "percent_of_portfolio_equity" },
        "max_account_allocation": { "value": risk.max_account_allocation, "unit": "percent_of_portfolio_equity" },
        "stop_loss_percent": { "value": risk.stop_loss_percent, "unit": "percent_move_from_entry" },
        "take_profit_percent": { "value": risk.take_profit_percent, "unit": "percent_move_from_entry" },
        "daily_loss_limit": { "value": risk.daily_loss_limit, "unit": "percent_of_starting_day_equity" },
        "max_open_positions": { "value": risk.max_open_positions, "unit": "count" },
        "max_daily_trades": { "value": risk.max_daily_trades, "unit": "count" }
    })
}

fn risk_gate_json(
    signal: &str,
    confidence: f64,
    adjusted_net: f64,
    portfolio: &PortfolioData,
    symbol: &str,
    risk: &RiskParameters,
    min_confidence: f64,
) -> serde_json::Value {
    let equity = portfolio.total_value;
    let current_position_value = portfolio.positions.get(symbol).map(|p| p.market_val.abs()).unwrap_or(0.0);
    let current_position_pct = if equity > f64::EPSILON { current_position_value / equity * 100.0 } else { 0.0 };
    let mut violations = Vec::new();
    if current_position_pct > risk.max_position_size_percent {
        violations.push(format!(
            "current {} exposure {:.2}% exceeds max_position_size_percent {:.2}% (grandfathered; blocks new buys)",
            symbol, current_position_pct, risk.max_position_size_percent
        ));
    }
    if portfolio.positions.len() >= risk.max_open_positions && signal == SIGNAL_BUY && current_position_value <= f64::EPSILON {
        violations.push(format!("max_open_positions {} reached", risk.max_open_positions));
    }
    if confidence < min_confidence && signal != SIGNAL_HOLD {
        violations.push(format!("confidence {:.0}% below action minimum {:.0}%", confidence * 100.0, min_confidence * 100.0));
    }
    let buy_below_threshold = signal == SIGNAL_BUY && adjusted_net <= BUY_THRESHOLD;
    let sell_below_threshold = signal == SIGNAL_SELL && adjusted_net >= SELL_THRESHOLD;
    if buy_below_threshold || sell_below_threshold {
        violations.push(format!("adjusted net {:+.2} below directional threshold", adjusted_net));
    }
    let blocks_order = signal == SIGNAL_BUY && current_position_pct > risk.max_position_size_percent;
    json!({
        "passed": violations.is_empty() || !blocks_order,
        "blocks_order": blocks_order,
        "violations": violations,
        "current_position_pct": current_position_pct,
        "grandfathered_existing_exposure": current_position_pct > risk.max_position_size_percent,
        "note": "existing exposure violations are displayed and block additional buys, but do not force liquidation"
    })
}

fn order_sizing_json(price: f64, portfolio: &PortfolioData, risk: &RiskParameters, regime_multiplier: f64, confidence: f64) -> serde_json::Value {
    let equity = portfolio.total_value;
    let max_position_value = equity * (risk.max_position_size_percent / 100.0);
    let regime_adjusted_value = max_position_value * regime_multiplier * confidence;
    let estimated_qty = if price > f64::EPSILON { (regime_adjusted_value / price).floor() } else { 0.0 };
    json!({
        "equity": equity,
        "max_position_value": max_position_value,
        "regime_adjusted_value": regime_adjusted_value,
        "estimated_qty": estimated_qty,
        "formula": "floor(equity * max_position_size_percent / 100 * regime_multiplier * confidence / price)"
    })
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
        assert_eq!(signal.confidence, Some(0.0));
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
