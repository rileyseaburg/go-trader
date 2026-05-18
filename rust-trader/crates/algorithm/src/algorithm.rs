//! Core trading algorithm.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use chrono::Utc;
use tracing::{error, info};

use go_trader_indicators::IndicatorSet;
use go_trader_types::{MarketData, TradeSignal, SIGNAL_HOLD};

use crate::local_signal::RuleBasedSignalEngine;
use crate::strategy::Arbitrator;
use crate::types::*;

pub struct TradingAlgorithm {
    claude_client: Option<Box<dyn ClaudeClient>>,
    local_signal_engine: RuleBasedSignalEngine,
    arbitrator: Arbitrator,
    alpaca_client: Option<Box<dyn AlpacaClient>>,
    market_data: Arc<RwLock<HashMap<String, MarketData>>>,
    indicators: Arc<RwLock<HashMap<String, IndicatorSet>>>,
    signals: Arc<RwLock<HashMap<String, TradeSignal>>>,
    portfolio: Arc<RwLock<PortfolioData>>,
    risk_params: Arc<RwLock<RiskParameters>>,
    signal_callback: Arc<RwLock<Option<SignalCallback>>>,
    regime_multiplier: Arc<RwLock<f64>>,
    regime_name: Arc<RwLock<String>>,
    running: Arc<RwLock<bool>>,
}

impl Default for TradingAlgorithm {
    fn default() -> Self {
        Self::new()
    }
}

impl TradingAlgorithm {
    pub fn new() -> Self {
        Self {
            claude_client: None,
            local_signal_engine: RuleBasedSignalEngine,
            arbitrator: Arbitrator::new_default(),
            alpaca_client: None,
            market_data: Arc::new(RwLock::new(HashMap::new())),
            indicators: Arc::new(RwLock::new(HashMap::new())),
            signals: Arc::new(RwLock::new(HashMap::new())),
            portfolio: Arc::new(RwLock::new(PortfolioData::default())),
            risk_params: Arc::new(RwLock::new(RiskParameters::default())),
            signal_callback: Arc::new(RwLock::new(None)),
            regime_multiplier: Arc::new(RwLock::new(1.0)),
            regime_name: Arc::new(RwLock::new(String::new())),
            running: Arc::new(RwLock::new(false)),
        }
    }

    pub fn with_claude(mut self, c: Box<dyn ClaudeClient>) -> Self {
        self.claude_client = Some(c);
        self
    }
    pub fn with_alpaca(mut self, c: Box<dyn AlpacaClient>) -> Self {
        self.alpaca_client = Some(c);
        self
    }

    pub fn start(&self, symbols: &[String]) {
        *self.running.write().unwrap() = true;
        let mut md = self.market_data.write().unwrap();
        for s in symbols {
            md.insert(
                s.clone(),
                MarketData {
                    symbol: s.clone(),
                    price: 0.0,
                    high_24h: 0.0,
                    low_24h: 0.0,
                    volume_24h: 0.0,
                    change_24h: 0.0,
                },
            );
        }
        info!("TradingAlgorithm started with {} symbols", symbols.len());
    }

    pub fn stop(&self) {
        *self.running.write().unwrap() = false;
    }
    pub fn is_running(&self) -> bool {
        *self.running.read().unwrap()
    }

    pub fn update_market_data(
        &self,
        symbol: &str,
        price: f64,
        high: f64,
        low: f64,
        volume: f64,
        change: f64,
    ) {
        self.market_data
            .write()
            .unwrap()
            .entry(symbol.into())
            .and_modify(|m| {
                m.price = price;
                m.high_24h = high;
                m.low_24h = low;
                m.volume_24h = volume;
                m.change_24h = change;
            })
            .or_insert_with(|| MarketData {
                symbol: symbol.into(),
                price,
                high_24h: high,
                low_24h: low,
                volume_24h: volume,
                change_24h: change,
            });
    }

    /// Update the computed indicator set for a symbol.
    ///
    /// Typically called after `compute_all()` on the bar buffer produces
    /// a fresh `IndicatorSet`.
    pub fn update_indicators(&self, symbol: &str, set: IndicatorSet) {
        self.indicators
            .write()
            .unwrap()
            .insert(symbol.to_string(), set);
    }

    /// Get the current indicator set for a symbol.
    pub fn get_indicators(&self, symbol: &str) -> Option<IndicatorSet> {
        self.indicators.read().unwrap().get(symbol).cloned()
    }

    pub async fn process_symbol(
        &self,
        symbol: &str,
    ) -> Result<TradeSignal, Box<dyn std::error::Error + Send + Sync>> {
        let md = self
            .market_data
            .read()
            .unwrap()
            .get(symbol)
            .cloned()
            .ok_or_else(|| format!("no market data for {}", symbol))?;
        let portfolio = self.portfolio.read().unwrap().clone();
        let risk = self.risk_params.read().unwrap().clone();
        let regime_name = self.get_regime_name();
        let regime_multiplier = self.get_regime_multiplier();

        let signal = match &self.claude_client {
            Some(c) => c
                .generate_trade_signal(symbol, &md, &portfolio)
                .unwrap_or_else(|e| {
                    error!("Claude error for {}: {}", symbol, e);
                    TradeSignal {
                        symbol: symbol.into(),
                        signal: SIGNAL_HOLD.into(),
                        order_type: "market".into(),
                        limit_price: None,
                        timestamp: Utc::now(),
                        reasoning: format!("Claude error: {}", e),
                        confidence: None,
                    }
                }),
            None => {
                // Use multi-strategy arbitrator when indicator data is available,
                // otherwise fall back to the simple rule-based engine.
                let indicators = self.indicators.read().unwrap();
                if let Some(ind_set) = indicators.get(symbol) {
                    self.arbitrator.generate_signal(
                        symbol,
                        &md,
                        &portfolio,
                        &risk,
                        &regime_name,
                        regime_multiplier,
                        ind_set,
                    )
                } else {
                    self.local_signal_engine.generate(
                        symbol,
                        &md,
                        &portfolio,
                        &risk,
                        &regime_name,
                        regime_multiplier,
                    )
                }
            }
        };

        self.signals
            .write()
            .unwrap()
            .insert(symbol.into(), signal.clone());
        if let Some(ref cb) = *self.signal_callback.read().unwrap() {
            cb(&signal);
        }
        Ok(signal)
    }

    pub fn get_signal(&self, s: &str) -> Option<TradeSignal> {
        self.signals.read().unwrap().get(s).cloned()
    }
    pub fn get_all_signals(&self) -> HashMap<String, TradeSignal> {
        self.signals.read().unwrap().clone()
    }
    pub fn get_risk_parameters(&self) -> RiskParameters {
        self.risk_params.read().unwrap().clone()
    }
    pub fn update_risk_parameters(&self, p: RiskParameters) {
        *self.risk_params.write().unwrap() = p;
    }

    pub fn set_regime_multiplier(&self, name: &str, mult: f64) {
        *self.regime_name.write().unwrap() = name.into();
        *self.regime_multiplier.write().unwrap() = mult;
    }
    pub fn get_regime_multiplier(&self) -> f64 {
        *self.regime_multiplier.read().unwrap()
    }
    pub fn get_regime_name(&self) -> String {
        self.regime_name.read().unwrap().clone()
    }
    pub fn register_signal_callback(&self, cb: SignalCallback) {
        *self.signal_callback.write().unwrap() = Some(cb);
    }

    pub fn update_portfolio(
        &self,
        balance: f64,
        total_value: f64,
        daily_pnl: f64,
        daily_return: f64,
        positions: HashMap<String, PositionData>,
    ) {
        let mut p = self.portfolio.write().unwrap();
        p.balance = balance;
        p.total_value = total_value;
        p.daily_pnl = daily_pnl;
        p.daily_return = daily_return;
        p.positions = positions;
    }

    pub fn get_portfolio(&self) -> PortfolioData {
        self.portfolio.read().unwrap().clone()
    }

    pub async fn execute_trade(
        &self,
        signal: &TradeSignal,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let client = self
            .alpaca_client
            .as_ref()
            .ok_or("No Alpaca client configured")?;
        let account = client.get_account()?;
        let risk = self.risk_params.read().unwrap();
        let regime_mult = *self.regime_multiplier.read().unwrap();

        // ATR-based position sizing: risk = 1 ATR unit per share,
        // cap at max_position_size_percent of equity × regime.
        let base_position_value =
            account.equity * (risk.max_position_size_percent / 100.0) * regime_mult;

        // If we have ATR data, refine position size so that the dollar risk
        // per trade (shares × ATR) stays within 1% of equity.
        let position_value = self
            .indicators
            .read()
            .unwrap()
            .get(&signal.symbol)
            .and_then(|ind| ind.atr_14)
            .map(|atr| {
                if atr > f64::EPSILON {
                    let risk_budget = account.equity * 0.01; // 1% of equity
                    let shares_by_atr = risk_budget / atr;
                    let price = signal.limit_price.unwrap_or(0.0);
                    if price > f64::EPSILON {
                        let atr_position_value = shares_by_atr * price;
                        // Use the smaller of ATR-based or regime-based position
                        atr_position_value.min(base_position_value)
                    } else {
                        base_position_value
                    }
                } else {
                    base_position_value
                }
            })
            .unwrap_or(base_position_value);

        match signal.signal.as_str() {
            "buy" => {
                let price = signal.limit_price.unwrap_or(0.0);
                if price <= 0.0 {
                    return Err("invalid price for buy order".into());
                }
                let shares = (position_value / price).floor();
                if shares <= 0.0 {
                    return Err("calculated position size too small".into());
                }
                let req = AlpacaOrderRequest {
                    symbol: signal.symbol.clone(),
                    side: "buy".into(),
                    qty: shares,
                    order_type: signal.order_type.clone(),
                    time_in_force: "day".into(),
                    limit_price: signal.limit_price,
                };
                let order = client.place_order(req)?;
                Ok(format!(
                    "Buy order placed: {} shares of {}",
                    order.id, signal.symbol
                ))
            }
            "sell" => {
                let pos = client.get_position(&signal.symbol)?;
                let req = AlpacaOrderRequest {
                    symbol: signal.symbol.clone(),
                    side: "sell".into(),
                    qty: pos.qty,
                    order_type: signal.order_type.clone(),
                    time_in_force: "day".into(),
                    limit_price: signal.limit_price,
                };
                let order = client.place_order(req)?;
                Ok(format!(
                    "Sell order placed: {} shares of {}",
                    order.id, signal.symbol
                ))
            }
            _ => Ok(format!(
                "No trade action for signal type: {}",
                signal.signal
            )),
        }
    }
}
