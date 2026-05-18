//! Shared types for the algorithm crate.

use go_trader_types::{MarketData, TradeSignal};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PositionData {
    pub symbol: String,
    pub quantity: f64,
    pub avg_price: f64,
    pub market_val: f64,
    pub profit: f64,
    pub return_pct: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PortfolioData {
    pub balance: f64,
    pub total_value: f64,
    pub daily_pnl: f64,
    pub daily_return: f64,
    pub positions: HashMap<String, PositionData>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RiskParameters {
    pub max_position_size_percent: f64,
    pub max_account_allocation: f64,
    pub stop_loss_percent: f64,
    pub take_profit_percent: f64,
    pub daily_loss_limit: f64,
    pub max_open_positions: usize,
    pub max_daily_trades: usize,
}

impl Default for RiskParameters {
    fn default() -> Self {
        Self {
            max_position_size_percent: 5.0,
            max_account_allocation: 10.0,
            stop_loss_percent: 5.0,
            take_profit_percent: 15.0,
            daily_loss_limit: 10.0,
            max_open_positions: 10,
            max_daily_trades: 10,
        }
    }
}

pub trait ClaudeClient: Send + Sync {
    fn generate_trade_signal(
        &self,
        symbol: &str,
        market_data: &MarketData,
        portfolio: &PortfolioData,
    ) -> Result<TradeSignal, Box<dyn std::error::Error + Send + Sync>>;
}

pub trait AlpacaClient: Send + Sync {
    fn get_account(&self) -> Result<AlpacaAccount, Box<dyn std::error::Error + Send + Sync>>;
    fn get_position(
        &self,
        symbol: &str,
    ) -> Result<AlpacaPosition, Box<dyn std::error::Error + Send + Sync>>;
    fn place_order(
        &self,
        req: AlpacaOrderRequest,
    ) -> Result<AlpacaOrder, Box<dyn std::error::Error + Send + Sync>>;
    fn get_positions(
        &self,
    ) -> Result<Vec<AlpacaPosition>, Box<dyn std::error::Error + Send + Sync>>;
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlpacaAccount {
    pub cash: f64,
    pub equity: f64,
    pub portfolio_value: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlpacaPosition {
    pub symbol: String,
    pub qty: f64,
    pub avg_entry_price: f64,
    pub market_value: f64,
    pub unrealized_pl: f64,
    pub unrealized_plpc: f64,
}

#[derive(Debug, Clone)]
pub struct AlpacaOrderRequest {
    pub symbol: String,
    pub side: String,
    pub qty: f64,
    pub order_type: String,
    pub time_in_force: String,
    pub limit_price: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlpacaOrder {
    pub id: String,
    pub status: String,
    pub filled_avg_price: Option<f64>,
}

pub type SignalCallback = Box<dyn Fn(&TradeSignal) + Send + Sync>;
