//! Backtesting trade records and performance report.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
pub enum TradeSide { Buy, Sell }

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TradeRecord {
    pub id: usize,
    pub symbol: String,
    pub side: TradeSide,
    pub qty: f64,
    pub signal_price: f64,
    pub fill_price: f64,
    pub slippage: f64,
    pub commission: f64,
    pub bar_timestamp: i64,
    pub sim_timestamp: DateTime<Utc>,
    pub reasoning: String,
    pub confidence: f64,
    pub realized_pnl: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EquityPoint {
    pub bar_timestamp: i64,
    pub equity: f64,
    pub drawdown: f64,
    pub drawdown_pct: f64,
    pub open_positions: usize,
}

fn compute_avg_holding(trades: &[TradeRecord]) -> f64 {
    let mut buy_ts: Vec<i64> = Vec::new();
    let mut holding: Vec<i64> = Vec::new();
    for t in trades {
        match t.side {
            TradeSide::Buy => buy_ts.push(t.bar_timestamp),
            TradeSide::Sell => {
                if let Some(open) = buy_ts.pop() { holding.push(t.bar_timestamp - open); }
            }
        }
    }
    if holding.is_empty() { 0.0 } else { holding.iter().sum::<i64>() as f64 / holding.len() as f64 }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BacktestReport {
    pub symbol: String,
    pub bars_processed: usize,
    pub total_trades: usize,
    pub winning_trades: usize,
    pub losing_trades: usize,
    pub win_rate: f64,
    pub total_pnl: f64,
    pub gross_profit: f64,
    pub gross_loss: f64,
    pub profit_factor: Option<f64>,
    pub max_drawdown: f64,
    pub max_drawdown_pct: f64,
    pub starting_equity: f64,
    pub ending_equity: f64,
    pub total_return_pct: f64,
    pub annualized_return_pct: Option<f64>,
    pub sharpe_ratio: Option<f64>,
    pub avg_trade_pnl: f64,
    pub avg_win: f64,
    pub avg_loss: f64,
    pub largest_win: f64,
    pub largest_loss: f64,
    pub avg_holding_bars: f64,
    pub total_commission: f64,
    pub total_slippage: f64,
    pub trades: Vec<TradeRecord>,
    pub equity_curve: Vec<EquityPoint>,
}

impl BacktestReport {
    pub fn from_trades(
        symbol: &str,
        bars_processed: usize,
        trades: Vec<TradeRecord>,
        equity_curve: Vec<EquityPoint>,
        starting_equity: f64,
    ) -> Self {
        let closing: Vec<&TradeRecord> = trades.iter().filter(|t| t.realized_pnl.is_some()).collect();
        let num_closing = closing.len();
        let win_pnls: Vec<f64> = closing.iter().filter_map(|t| t.realized_pnl.filter(|&p| p > 0.0)).collect();
        let loss_pnls: Vec<f64> = closing.iter().filter_map(|t| t.realized_pnl.filter(|&p| p < 0.0)).collect();
        let winning_trades = win_pnls.len();
        let losing_trades = loss_pnls.len();
        let win_rate = if num_closing > 0 { winning_trades as f64 / num_closing as f64 } else { 0.0 };
        let gross_profit: f64 = win_pnls.iter().sum();
        let gross_loss: f64 = loss_pnls.iter().sum();
        let total_pnl = gross_profit + gross_loss;
        let profit_factor = if gross_loss.abs() > f64::EPSILON {
            Some(gross_profit / gross_loss.abs())
        } else if gross_profit > 0.0 { Some(f64::INFINITY) } else { None };
        let ending_equity = equity_curve.last().map(|p| p.equity).unwrap_or(starting_equity);
        let total_return_pct = (ending_equity - starting_equity) / starting_equity * 100.0;
        let mut peak = starting_equity;
        let mut max_dd = 0.0_f64;
        let mut max_drawdown_pct = 0.0_f64;
        for pt in &equity_curve {
            if pt.equity > peak { peak = pt.equity; }
            let dd = peak - pt.equity;
            let dd_pct = if peak > 0.0 { dd / peak * 100.0 } else { 0.0 };
            if dd > max_dd { max_dd = dd; }
            if dd_pct > max_drawdown_pct { max_drawdown_pct = dd_pct; }
        }
        let bars_per_year: f64 = 252.0 * 78.0;
        let annualized_return_pct = if bars_processed > (bars_per_year / 4.0) as usize {
            let years = bars_processed as f64 / bars_per_year;
            if years > 0.0 && starting_equity > 0.0 {
                Some(((ending_equity / starting_equity).powf(1.0 / years) - 1.0) * 100.0)
            } else { None }
        } else { None };
        let all_pnls: Vec<f64> = closing.iter().filter_map(|t| t.realized_pnl).collect();
        let sharpe_ratio = if all_pnls.len() >= 10 {
            let mean = all_pnls.iter().sum::<f64>() / all_pnls.len() as f64;
            let var = all_pnls.iter().map(|p| (p - mean).powi(2)).sum::<f64>() / all_pnls.len() as f64;
            let std = var.sqrt();
            if std > f64::EPSILON { Some(mean / std * (10.0 * 252.0_f64).sqrt()) } else { None }
        } else { None };
        let avg_trade_pnl = if num_closing > 0 { total_pnl / num_closing as f64 } else { 0.0 };
        let avg_win = if winning_trades > 0 { gross_profit / winning_trades as f64 } else { 0.0 };
        let avg_loss = if losing_trades > 0 { gross_loss / losing_trades as f64 } else { 0.0 };
        let largest_win = win_pnls.iter().cloned().fold(0.0_f64, f64::max);
        let largest_loss = loss_pnls.iter().cloned().fold(0.0_f64, f64::min);
        let total_commission: f64 = trades.iter().map(|t| t.commission).sum();
        let total_slippage: f64 = trades.iter().map(|t| t.slippage * t.qty).sum();
        let avg_holding_bars = compute_avg_holding(&trades);
        Self {
            symbol: symbol.to_string(), bars_processed, total_trades: trades.len(),
            winning_trades, losing_trades, win_rate, total_pnl, gross_profit, gross_loss,
            profit_factor, max_drawdown: max_dd, max_drawdown_pct, starting_equity,
            ending_equity, total_return_pct, annualized_return_pct, sharpe_ratio,
            avg_trade_pnl, avg_win, avg_loss, largest_win, largest_loss, avg_holding_bars,
            total_commission, total_slippage, trades, equity_curve,
        }
    }

    pub fn print_summary(&self) {
        println!("========== BACKTEST: {} ==========", self.symbol);
        println!("Bars: {} | Equity: ${:.2} -> ${:.2} | Return: {:.2}%",
            self.bars_processed, self.starting_equity, self.ending_equity, self.total_return_pct);
        if let Some(ar) = self.annualized_return_pct { println!("Annualized: {:.2}%", ar); }
        println!("Trades: {} | Wins: {} | Losses: {} | Win Rate: {:.1}%",
            self.total_trades, self.winning_trades, self.losing_trades, self.win_rate * 100.0);
        println!("P&L: ${:.2} | Avg Win: ${:.2} | Avg Loss: ${:.2}",
            self.total_pnl, self.avg_win, self.avg_loss);
        if let Some(pf) = self.profit_factor { println!("Profit Factor: {:.2}", pf); }
        if let Some(sr) = self.sharpe_ratio { println!("Sharpe: {:.2}", sr); }
        println!("Max DD: ${:.2} ({:.1}%)", self.max_drawdown, self.max_drawdown_pct);
        println!("Commissions: ${:.2} | Slippage: ${:.2}", self.total_commission, self.total_slippage);
        println!("Avg Holding: {:.0} bars", self.avg_holding_bars);
    }
}
