//! MACD (Moving Average Convergence Divergence).
//!
//! Standard MACD: EMA(12) - EMA(26), signal = EMA(9) of MACD line.

use crate::moving_average::ema;

#[derive(Debug, Clone, PartialEq)]
pub struct MacdResult {
    pub macd_line: f64,
    pub signal_line: f64,
    pub histogram: f64,
}

/// Compute MACD from close prices.
///
/// Returns `None` if fewer than 26 bars (need full EMA(26) seed).
pub fn macd(closes: &[f64]) -> Option<MacdResult> {
    macd_with_params(closes, 12, 26, 9)
}

/// Compute MACD with custom fast/slow/signal periods.
pub fn macd_with_params(
    closes: &[f64],
    fast_period: usize,
    slow_period: usize,
    signal_period: usize,
) -> Option<MacdResult> {
    if closes.len() < slow_period {
        return None;
    }

    // Compute EMA series for fast and slow
    let fast_ema_series = ema_series_full(closes, fast_period)?;
    let slow_ema_series = ema_series_full(closes, slow_period)?;

    // MACD line = fast_ema - slow_ema (where both exist)
    let macd_line: Vec<f64> = fast_ema_series
        .iter()
        .zip(slow_ema_series.iter())
        .filter_map(|(f, s)| match (f, s) {
            (Some(fv), Some(sv)) => Some(fv - sv),
            _ => None,
        })
        .collect();

    if macd_line.is_empty() {
        return None;
    }

    // Signal line = EMA(signal_period) of MACD line
    let signal = if macd_line.len() >= signal_period {
        ema(&macd_line, signal_period)
    } else {
        None
    };

    let macd_val = *macd_line.last()?;
    let signal_val = signal.unwrap_or(0.0);
    let histogram = macd_val - signal_val;

    Some(MacdResult {
        macd_line: macd_val,
        signal_line: signal_val,
        histogram,
    })
}

/// Internal: full EMA series with seeding (returns Vec<Option<f64>>).
fn ema_series_full(values: &[f64], period: usize) -> Option<Vec<Option<f64>>> {
    if values.len() < period || period == 0 {
        return None;
    }
    let k = 2.0 / (period as f64 + 1.0);
    let seed: f64 = values[..period].iter().sum::<f64>() / period as f64;
    let mut result = vec![None; values.len()];
    result[period - 1] = Some(seed);
    let mut ema_val = seed;
    for i in period..values.len() {
        ema_val = (values[i] - ema_val) * k + ema_val;
        result[i] = Some(ema_val);
    }
    Some(result)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn macd_basic() {
        // 30 closes to get past the 26-period slow EMA + 9-period signal
        let closes: Vec<f64> = (1..=40).map(|i| i as f64 * 1.0).collect();
        let result = macd(&closes);
        assert!(result.is_some());
        let r = result.unwrap();
        // Strong uptrend → MACD line should be positive
        assert!(r.macd_line > 0.0);
        // Histogram = macd - signal
        assert!((r.histogram - (r.macd_line - r.signal_line)).abs() < 1e-10);
    }

    #[test]
    fn macd_too_few() {
        let closes = vec![1.0, 2.0, 3.0];
        assert!(macd(&closes).is_none());
    }
}
