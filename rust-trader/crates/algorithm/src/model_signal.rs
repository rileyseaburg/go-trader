//! External model-backed trading signal client.
//!
//! This module intentionally keeps the model integration behind the existing
//! `ClaudeClient` trait so the algorithm can fall back to the deterministic
//! local engine when no model API key is configured.

use chrono::Utc;
use go_trader_types::{
    MarketData, TradeSignal, SIGNAL_BUY, SIGNAL_CLOSE, SIGNAL_HOLD, SIGNAL_SELL,
};
use serde::Deserialize;
use serde_json::{json, Value};
use tracing::info;

use crate::types::{ClaudeClient, PortfolioData};

const DEFAULT_ANTHROPIC_URL: &str = "https://api.anthropic.com/v1/messages";
const DEFAULT_ANTHROPIC_MODEL: &str = "claude-3-5-sonnet-20241022";

/// Synchronous Anthropic/Claude client used by `TradingAlgorithm`.
#[derive(Debug, Clone)]
pub struct ModelSignalClient {
    api_key: String,
    api_url: String,
    model: String,
}

impl ModelSignalClient {
    pub fn new(api_key: impl Into<String>, model: impl Into<String>) -> Self {
        Self {
            api_key: api_key.into(),
            api_url: DEFAULT_ANTHROPIC_URL.to_string(),
            model: model.into(),
        }
    }

    pub fn with_api_url(mut self, api_url: impl Into<String>) -> Self {
        self.api_url = api_url.into();
        self
    }

    /// Build a model client from environment when credentials are present.
    ///
    /// Supported variables:
    /// - `ANTHROPIC_API_KEY` or `CLAUDE_API_KEY`
    /// - `ANTHROPIC_MODEL` or `CLAUDE_MODEL` (optional)
    /// - `ANTHROPIC_API_URL` (optional)
    pub fn from_env() -> Option<Self> {
        let api_key = std::env::var("ANTHROPIC_API_KEY")
            .or_else(|_| std::env::var("CLAUDE_API_KEY"))
            .ok()
            .filter(|v| !v.trim().is_empty())?;
        let model = std::env::var("ANTHROPIC_MODEL")
            .or_else(|_| std::env::var("CLAUDE_MODEL"))
            .unwrap_or_else(|_| DEFAULT_ANTHROPIC_MODEL.to_string());
        let api_url = std::env::var("ANTHROPIC_API_URL")
            .unwrap_or_else(|_| DEFAULT_ANTHROPIC_URL.to_string());
        Some(Self::new(api_key, model).with_api_url(api_url))
    }

    pub fn model_name(&self) -> &str {
        &self.model
    }

    fn prompt(symbol: &str, market_data: &MarketData, portfolio: &PortfolioData) -> String {
        json!({
            "task": "Generate a conservative trading signal suggestion. Return JSON only.",
            "required_schema": {
                "signal": "buy | sell | hold | close",
                "order_type": "market | limit",
                "limit_price": "number or null",
                "confidence": "0.0 to 1.0",
                "reasoning": "brief risk-aware explanation"
            },
            "constraints": [
                "Do not invent market data.",
                "Prefer hold when evidence is weak or data is incomplete.",
                "This is a suggestion only; downstream risk controls may reject it."
            ],
            "symbol": symbol,
            "market_data": market_data,
            "portfolio": portfolio
        })
        .to_string()
    }

    fn parse_signal(
        symbol: &str,
        text: &str,
    ) -> Result<TradeSignal, Box<dyn std::error::Error + Send + Sync>> {
        let json_text = extract_json_object(text)
            .ok_or_else(|| "model response did not contain a JSON object".to_string())?;
        let parsed: ModelSignal = serde_json::from_str(json_text)?;
        let signal = normalize_signal(parsed.signal.as_deref().unwrap_or(SIGNAL_HOLD));
        let order_type = normalize_order_type(parsed.order_type.as_deref().unwrap_or("market"));
        let confidence = parsed.confidence.map(|c| c.clamp(0.0, 1.0));
        let limit_price = if order_type == "limit" {
            parsed.limit_price
        } else {
            None
        };
        Ok(TradeSignal {
            symbol: parsed.symbol.unwrap_or_else(|| symbol.to_string()),
            signal,
            order_type,
            limit_price,
            timestamp: Utc::now(),
            reasoning: format!(
                "model={} source=external_llm; confidence={}; {}",
                DEFAULT_ANTHROPIC_MODEL,
                confidence.map(|c| format!("{:.0}%", c * 100.0)).unwrap_or_else(|| "unknown".to_string()),
                parsed
                    .reasoning
                    .unwrap_or_else(|| "model returned no reasoning".to_string())
            ),
            confidence,
            audit: Some(json!({
                "pipeline": "external_llm",
                "canonical_confidence": confidence,
                "model": DEFAULT_ANTHROPIC_MODEL,
                "note": "confidence comes directly from clamped model JSON"
            })),
        })
    }
}

impl ClaudeClient for ModelSignalClient {
    fn generate_trade_signal(
        &self,
        symbol: &str,
        market_data: &MarketData,
        portfolio: &PortfolioData,
    ) -> Result<TradeSignal, Box<dyn std::error::Error + Send + Sync>> {
        let request = json!({
            "model": self.model,
            "max_tokens": 700,
            "temperature": 0.1,
            "system": "You are a conservative trading risk assistant. You only output valid JSON matching the requested schema. Never include markdown.",
            "messages": [{"role": "user", "content": Self::prompt(symbol, market_data, portfolio)}]
        });

        let response = post_anthropic_message(self.api_url.clone(), self.api_key.clone(), request)?;

        let text = response
            .content
            .into_iter()
            .filter_map(|c| c.text)
            .collect::<Vec<_>>()
            .join("\n");
        info!(symbol, model = %self.model, "received external model trading suggestion");
        let mut signal = Self::parse_signal(symbol, &text)?;
        signal.reasoning = signal
            .reasoning
            .replacen(DEFAULT_ANTHROPIC_MODEL, &self.model, 1);
        Ok(signal)
    }
}

#[derive(Debug, Deserialize)]
struct AnthropicResponse {
    content: Vec<AnthropicContent>,
}

#[derive(Debug, Deserialize)]
struct AnthropicContent {
    text: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ModelSignal {
    symbol: Option<String>,
    signal: Option<String>,
    order_type: Option<String>,
    limit_price: Option<f64>,
    confidence: Option<f64>,
    reasoning: Option<String>,
}

fn post_anthropic_message(
    api_url: String,
    api_key: String,
    request: Value,
) -> Result<AnthropicResponse, Box<dyn std::error::Error + Send + Sync>> {
    std::thread::spawn(move || {
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()?;
        runtime.block_on(async move {
            let client = reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(45))
                .build()?;
            Ok(client
                .post(api_url)
                .header("x-api-key", api_key)
                .header("anthropic-version", "2023-06-01")
                .header("content-type", "application/json")
                .json(&request)
                .send()
                .await?
                .error_for_status()?
                .json::<AnthropicResponse>()
                .await?)
        })
    })
    .join()
    .map_err(|_| "model request worker panicked".to_string())?
}

fn normalize_signal(value: &str) -> String {
    match value.trim().to_ascii_lowercase().as_str() {
        SIGNAL_BUY => SIGNAL_BUY.to_string(),
        SIGNAL_SELL => SIGNAL_SELL.to_string(),
        SIGNAL_CLOSE => SIGNAL_CLOSE.to_string(),
        _ => SIGNAL_HOLD.to_string(),
    }
}

fn normalize_order_type(value: &str) -> String {
    match value.trim().to_ascii_lowercase().as_str() {
        "limit" => "limit".to_string(),
        _ => "market".to_string(),
    }
}

fn extract_json_object(text: &str) -> Option<&str> {
    let start = text.find('{')?;
    let mut depth = 0_i32;
    let mut in_string = false;
    let mut escape = false;
    for (offset, ch) in text[start..].char_indices() {
        if escape {
            escape = false;
            continue;
        }
        match ch {
            '\\' if in_string => escape = true,
            '"' => in_string = !in_string,
            '{' if !in_string => depth += 1,
            '}' if !in_string => {
                depth -= 1;
                if depth == 0 {
                    let end = start + offset + ch.len_utf8();
                    return Some(&text[start..end]);
                }
            }
            _ => {}
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extracts_and_normalizes_model_signal() {
        let text = r#"Here is JSON: {"signal":"BUY","order_type":"limit","limit_price":123.45,"confidence":1.7,"reasoning":"Momentum improved"}"#;
        let sig = ModelSignalClient::parse_signal("AAPL", text).unwrap();
        assert_eq!(sig.symbol, "AAPL");
        assert_eq!(sig.signal, SIGNAL_BUY);
        assert_eq!(sig.order_type, "limit");
        assert_eq!(sig.limit_price, Some(123.45));
        assert_eq!(sig.confidence, Some(1.0));
        assert!(sig.reasoning.contains("external_llm"));
    }

    #[test]
    fn defaults_unknown_signal_to_hold() {
        let text = r#"{"signal":"moon","order_type":"stop","reasoning":"bad schema"}"#;
        let sig = ModelSignalClient::parse_signal("MSFT", text).unwrap();
        assert_eq!(sig.signal, SIGNAL_HOLD);
        assert_eq!(sig.order_type, "market");
    }
}
