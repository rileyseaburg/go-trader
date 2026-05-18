//! Core backtesting engine.

use crate::report::*;
use chrono::Utc;
use go_trader_indicators::{Bar, compute_all};
use go_trader_algorithm::{Arbitrator, PortfolioData, PositionData, RiskParameters};
use go_trader_types::{MarketData, TradeSignal, SIGNAL_BUY, SIGNAL_SELL, SIGNAL_HOLD};
use std::collections::HashMap;

#[derive(Debug, Clone)]
pub struct BacktestConfig {
    pub starting_equity: f64,
    pub slippage_per_share: f64,
    pub commission_per_trade: f64,
    pub warmup_bars: usize,
    pub risk: RiskParameters,
    pub min_confidence: f64,
    pub fixed_lot_size: Option<f64>,
    pub risk_per_trade_pct: Option<f64>,
    pub regime_name: String,
    pub regime_multiplier: f64,
}

impl Default for BacktestConfig {
    fn default() -> Self {
        Self {
            starting_equity: 5000.0,
            slippage_per_share: 0.02,
            commission_per_trade: 0.50,
            warmup_bars: 50,
            risk: RiskParameters::default(),
            min_confidence: 0.6,
            fixed_lot_size: Some(100.0),
            risk_per_trade_pct: None,
            regime_name: "NEUTRAL".to_string(),
            regime_multiplier: 1.0,
        }
    }
}

#[derive(Debug, Clone)]
#[allow(dead_code)]
struct SimPosition {
    symbol: String,
    qty: f64,
    avg_price: f64,
    entry_bar: usize,
}

pub struct BacktestEngine {
    config: BacktestConfig,
    arbitrator: Arbitrator,
    bar_buffer: Vec<Bar>,
    positions: HashMap<String, SimPosition>,
    trades: Vec<TradeRecord>,
    equity_curve: Vec<EquityPoint>,
    cash: f64,
    next_trade_id: usize,
    bars_processed: usize,
    daily_trades: usize,
    last_date: String,
}

impl BacktestEngine {
    pub fn new(config: BacktestConfig) -> Self {
        let arbitrator = Arbitrator::new_default();
        Self {
            cash: config.starting_equity,
            arbitrator,
            bar_buffer: Vec::new(),
            positions: HashMap::new(),
            trades: Vec::new(),
            equity_curve: Vec::new(),
            next_trade_id: 0,
            bars_processed: 0,
            config,
            daily_trades: 0,
            last_date: String::new(),
        }
    }

    pub fn run(&mut self, symbol: &str, bars: Vec<Bar>) -> BacktestReport {
        for bar in &bars { self.process_bar(symbol, bar); }
        self.build_report(symbol)
    }

    fn process_bar(&mut self, symbol: &str, bar: &Bar) {
        self.bar_buffer.push(bar.clone());
        self.bars_processed += 1;
        let date = chrono::DateTime::from_timestamp(bar.timestamp / 1000, 0)
            .map(|dt| dt.format("%Y-%m-%d").to_string())
            .unwrap_or_default();
        if date != self.last_date {
            self.daily_trades = 0;
            self.last_date = date;
        }
        if self.bar_buffer.len() < self.config.warmup_bars {
            self.record_equity(bar.timestamp);
            return;
        }
        let indicators = compute_all(&self.bar_buffer);
        self.check_exits(symbol, bar);
        let md = MarketData {
            symbol: symbol.to_string(),
            price: bar.close,
            high_24h: bar.high,
            low_24h: bar.low,
            volume_24h: bar.volume,
            change_24h: 0.0,
        };
        let portfolio = self.build_portfolio(symbol, bar);
        let signal = self.arbitrator.generate_signal(
            symbol, &md, &portfolio, &self.config.risk,
            &self.config.regime_name, self.config.regime_multiplier, &indicators,
        );
        if signal.signal != SIGNAL_HOLD {
            let conf = signal.confidence.unwrap_or(0.0);
            if conf >= self.config.min_confidence {
                self.execute_signal(symbol, bar, &signal);
            }
        }
        self.record_equity(bar.timestamp);
    }
}

impl BacktestEngine {
    fn check_exits(&mut self, _symbol: &str, bar: &Bar) {
        let mut to_exit: Vec<(String, f64, String)> = Vec::new();
        for (sym, pos) in &self.positions {
            let pnl_pct = (bar.close - pos.avg_price) / pos.avg_price * 100.0;
            if pnl_pct <= -self.config.risk.stop_loss_percent {
                to_exit.push((sym.clone(), bar.close, format!("Stop-loss: {:.1}%", pnl_pct)));
            } else if pnl_pct >= self.config.risk.take_profit_percent {
                to_exit.push((sym.clone(), bar.close, format!("Take-profit: {:.1}%", pnl_pct)));
            }
        }
        for (sym, price, reason) in to_exit {
            self.close_position(&sym, price, bar.timestamp, &reason);
        }
    }

    fn build_portfolio(&self, _symbol: &str, bar: &Bar) -> PortfolioData {
        let mut positions = HashMap::new();
        let total_mv: f64 = self.positions.values().map(|p| p.qty * p.avg_price).sum();
        let total_pnl: f64 = self.positions.values().map(|p| (bar.close - p.avg_price) * p.qty).sum();
        for (sym, pos) in &self.positions {
            let profit = (bar.close - pos.avg_price) * pos.qty;
            let ret = if pos.avg_price > 0.0 { (bar.close - pos.avg_price) / pos.avg_price * 100.0 } else { 0.0 };
            positions.insert(sym.clone(), PositionData {
                symbol: sym.clone(), quantity: pos.qty, avg_price: pos.avg_price,
                market_val: pos.qty * bar.close, profit, return_pct: ret,
            });
        }
        PortfolioData {
            balance: self.cash,
            total_value: self.cash + total_mv + total_pnl,
            daily_pnl: 0.0, daily_return: 0.0, positions,
        }
    }

    fn current_equity(&self, current_price: f64) -> f64 {
        let pos_val: f64 = self.positions.values().map(|p| p.qty * current_price).sum();
        self.cash + pos_val
    }

    fn compute_size(&self, price: f64, equity: f64) -> f64 {
        if let Some(lot) = self.config.fixed_lot_size {
            if lot * price > self.cash { (self.cash / price).floor() } else { lot }
        } else if let Some(pct) = self.config.risk_per_trade_pct {
            (equity * pct / 100.0 / price).floor()
        } else {
            let lot = 100.0;
            if lot * price > self.cash { (self.cash / price).floor() } else { lot }
        }
    }
}

impl BacktestEngine {
    fn execute_signal(&mut self, symbol: &str, bar: &Bar, signal: &TradeSignal) {
        if self.daily_trades >= self.config.risk.max_daily_trades { return; }
        if self.positions.len() >= self.config.risk.max_open_positions
            && !self.positions.contains_key(symbol) { return; }
        let equity = self.current_equity(bar.close);

        if signal.signal == SIGNAL_BUY && !self.positions.contains_key(symbol) {
            let qty = self.compute_size(bar.close, equity);
            if qty > 0.0 {
                let fill = bar.close + self.config.slippage_per_share;
                let cost = fill * qty + self.config.commission_per_trade;
                if cost <= self.cash {
                    self.cash -= cost;
                    self.positions.insert(symbol.to_string(), SimPosition {
                        symbol: symbol.to_string(), qty, avg_price: fill,
                        entry_bar: self.bars_processed,
                    });
                    self.record_trade(symbol, TradeSide::Buy, qty, bar.close, fill,
                        &signal.reasoning, signal.confidence.unwrap_or(0.0), bar.timestamp);
                    self.daily_trades += 1;
                }
            }
        } else if signal.signal == SIGNAL_SELL && self.positions.contains_key(symbol) {
            let fill = bar.close - self.config.slippage_per_share;
            self.close_position_at(symbol, fill, bar.timestamp, &signal.reasoning);
            self.daily_trades += 1;
        }
    }

    fn close_position(&mut self, symbol: &str, fill_price: f64, timestamp: i64, reason: &str) {
        self.close_position_at(symbol, fill_price, timestamp, reason);
    }

    fn close_position_at(&mut self, symbol: &str, fill_price: f64, timestamp: i64, reason: &str) {
        if let Some(pos) = self.positions.remove(symbol) {
            let proceeds = fill_price * pos.qty - self.config.commission_per_trade;
            self.cash += proceeds;
            let realized_pnl = (fill_price - pos.avg_price) * pos.qty
                - self.config.commission_per_trade * 2.0;
            let id = self.next_trade_id;
            self.next_trade_id += 1;
            self.trades.push(TradeRecord {
                id, symbol: symbol.to_string(), side: TradeSide::Sell,
                qty: pos.qty, signal_price: fill_price + self.config.slippage_per_share,
                fill_price, slippage: -self.config.slippage_per_share,
                commission: self.config.commission_per_trade,
                bar_timestamp: timestamp, sim_timestamp: Utc::now(),
                reasoning: reason.to_string(), confidence: 0.0,
                realized_pnl: Some(realized_pnl),
            });
        }
    }

    fn record_trade(&mut self, symbol: &str, side: TradeSide, qty: f64,
        signal_price: f64, fill_price: f64, reason: &str, confidence: f64, timestamp: i64) {
        let id = self.next_trade_id;
        self.next_trade_id += 1;
        self.trades.push(TradeRecord {
            id, symbol: symbol.to_string(), side, qty, signal_price, fill_price,
            slippage: self.config.slippage_per_share,
            commission: self.config.commission_per_trade,
            bar_timestamp: timestamp, sim_timestamp: Utc::now(),
            reasoning: reason.to_string(), confidence, realized_pnl: None,
        });
    }

    fn record_equity(&mut self, timestamp: i64) {
        let price = self.bar_buffer.last().map(|b| b.close).unwrap_or(self.config.starting_equity);
        let equity = self.current_equity(price);
        let peak = self.equity_curve.iter().map(|p| p.equity)
            .fold(self.config.starting_equity, f64::max);
        let dd = if equity < peak { peak - equity } else { 0.0 };
        let dd_pct = if peak > 0.0 { dd / peak * 100.0 } else { 0.0 };
        self.equity_curve.push(EquityPoint {
            bar_timestamp: timestamp, equity, drawdown: dd, drawdown_pct: dd_pct,
            open_positions: self.positions.len(),
        });
    }

    fn build_report(&self, symbol: &str) -> BacktestReport {
        BacktestReport::from_trades(
            symbol, self.bars_processed, self.trades.clone(),
            self.equity_curve.clone(), self.config.starting_equity,
        )
    }
}
