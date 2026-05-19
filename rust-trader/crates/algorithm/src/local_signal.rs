//! Deterministic local signal engine used when no external advisor is configured.

use chrono::Utc;
use go_trader_types::{MarketData, TradeSignal, SIGNAL_BUY, SIGNAL_HOLD, SIGNAL_SELL};

use crate::types::{PortfolioData, RiskParameters};

const DEFENSIVE_REGIME_CUTOFF: f64 = 0.70;
const BUY_CHANGE_THRESHOLD: f64 = 0.015;
const SELL_CHANGE_THRESHOLD: f64 = -0.025;
const NEUTRAL_CONFIDENCE: f64 = 0.55;
const ACTION_CONFIDENCE: f64 = 0.68;

#[derive(Debug, Clone, Default)]
pub struct RuleBasedSignalEngine;

impl RuleBasedSignalEngine {
    pub fn generate(
        &self,
        symbol: &str,
        market_data: &MarketData,
        portfolio: &PortfolioData,
        risk: &RiskParameters,
        regime_name: &str,
        regime_multiplier: f64,
    ) -> TradeSignal {
        let price = market_data.price;
        let change = market_data.change_24h;
        let held_position = portfolio.positions.get(symbol);
        let has_position = held_position
            .map(|p| p.quantity.abs() > f64::EPSILON)
            .unwrap_or(false);

        let (signal, confidence, reason) = if price <= 0.0 {
            (
                SIGNAL_HOLD,
                NEUTRAL_CONFIDENCE,
                "no valid live price available yet".to_string(),
            )
        } else if regime_multiplier <= DEFENSIVE_REGIME_CUTOFF && !has_position {
            (
                SIGNAL_HOLD,
                0.70,
                format!(
                    "defensive regime gate active: {} ×{:.2}; no existing position",
                    printable_regime(regime_name),
                    regime_multiplier
                ),
            )
        } else if has_position && change <= SELL_CHANGE_THRESHOLD {
            (
                SIGNAL_SELL,
                ACTION_CONFIDENCE,
                format!(
                    "local risk rule: existing position and 24h change {:+.2}% <= sell threshold {:+.2}%",
                    change * 100.0,
                    SELL_CHANGE_THRESHOLD * 100.0
                ),
            )
        } else if !has_position
            && regime_multiplier > DEFENSIVE_REGIME_CUTOFF
            && change >= BUY_CHANGE_THRESHOLD
        {
            (
                SIGNAL_BUY,
                ACTION_CONFIDENCE,
                format!(
                    "local momentum rule: 24h change {:+.2}% >= buy threshold {:+.2}% with regime {} ×{:.2}",
                    change * 100.0,
                    BUY_CHANGE_THRESHOLD * 100.0,
                    printable_regime(regime_name),
                    regime_multiplier
                ),
            )
        } else {
            (
                SIGNAL_HOLD,
                NEUTRAL_CONFIDENCE,
                format!(
                    "local rule engine hold: price {:.4}, 24h change {:+.2}%, regime {} ×{:.2}, max position {:.2}%",
                    price,
                    change * 100.0,
                    printable_regime(regime_name),
                    regime_multiplier,
                    risk.max_position_size_percent
                ),
            )
        };

        TradeSignal {
            symbol: symbol.into(),
            signal: signal.into(),
            order_type: "limit".into(),
            limit_price: if signal == SIGNAL_BUY || signal == SIGNAL_SELL {
                Some(price)
            } else {
                None
            },
            timestamp: Utc::now(),
            reasoning: format!(
                "RuleBasedSignalEngine: {}; source=local_deterministic; confidence={:.0}%; external_llm=false",
                reason,
                confidence * 100.0
            ),
            confidence: Some(confidence),
            audit: Some(serde_json::json!({
                "pipeline": "local_rule_engine",
                "canonical_confidence": confidence,
                "raw_inputs": {
                    "price": price,
                    "change_24h": change,
                    "has_position": has_position,
                    "regime_name": printable_regime(regime_name),
                    "regime_multiplier": regime_multiplier
                },
                "risk_units": risk_units_json(risk),
                "decision": { "action": signal, "reason": reason }
            })),
        }
    }
}

fn printable_regime(regime_name: &str) -> &str {
    if regime_name.is_empty() {
        "UNSET"
    } else {
        regime_name
    }
}

fn risk_units_json(risk: &RiskParameters) -> serde_json::Value {
    serde_json::json!({
        "max_position_size_percent": { "value": risk.max_position_size_percent, "unit": "percent_of_portfolio_equity" },
        "max_account_allocation": { "value": risk.max_account_allocation, "unit": "percent_of_portfolio_equity" },
        "stop_loss_percent": { "value": risk.stop_loss_percent, "unit": "percent_move_from_entry" },
        "take_profit_percent": { "value": risk.take_profit_percent, "unit": "percent_move_from_entry" },
        "daily_loss_limit": { "value": risk.daily_loss_limit, "unit": "percent_of_starting_day_equity" },
        "max_open_positions": { "value": risk.max_open_positions, "unit": "count" },
        "max_daily_trades": { "value": risk.max_daily_trades, "unit": "count" }
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn md(change_24h: f64) -> MarketData {
        MarketData {
            symbol: "AAPL".into(),
            price: 100.0,
            high_24h: 105.0,
            low_24h: 95.0,
            volume_24h: 1_000_000.0,
            change_24h,
        }
    }

    #[test]
    fn defensive_regime_blocks_new_buys() {
        let engine = RuleBasedSignalEngine;
        let signal = engine.generate(
            "AAPL",
            &md(0.05),
            &PortfolioData::default(),
            &RiskParameters::default(),
            "EBBING TIDE",
            0.6,
        );
        assert_eq!(signal.signal, SIGNAL_HOLD);
        assert!(signal.reasoning.contains("defensive regime gate"));
    }

    #[test]
    fn constructive_momentum_can_buy() {
        let engine = RuleBasedSignalEngine;
        let signal = engine.generate(
            "AAPL",
            &md(0.03),
            &PortfolioData::default(),
            &RiskParameters::default(),
            "RISING WATERS",
            1.0,
        );
        assert_eq!(signal.signal, SIGNAL_BUY);
        assert_eq!(signal.limit_price, Some(100.0));
    }
}
