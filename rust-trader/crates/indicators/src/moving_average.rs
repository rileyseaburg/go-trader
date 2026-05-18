//! Simple and Exponential Moving Averages.

/// Simple Moving Average over `period` values.
///
/// Returns `None` if `values.len() < period`.
pub fn sma(values: &[f64], period: usize) -> Option<f64> {
    if values.len() < period || period == 0 {
        return None;
    }
    let slice = &values[values.len() - period..];
    let sum: f64 = slice.iter().sum();
    Some(sum / period as f64)
}

/// Exponential Moving Average over `period` values.
///
/// Uses the standard multiplier `k = 2 / (period + 1)`.
/// Returns `None` if `values.len() < period`.
pub fn ema(values: &[f64], period: usize) -> Option<f64> {
    if values.len() < period || period == 0 {
        return None;
    }
    let k = 2.0 / (period as f64 + 1.0);
    // Seed with SMA of first `period` values
    let seed: f64 = values[..period].iter().sum::<f64>() / period as f64;
    let mut ema_val = seed;
    for val in &values[period..] {
        ema_val = (*val - ema_val) * k + ema_val;
    }
    Some(ema_val)
}

/// Full SMA series — one value per input element (None until enough data).
#[allow(dead_code)]
pub fn sma_series(values: &[f64], period: usize) -> Vec<Option<f64>> {
    values
        .iter()
        .enumerate()
        .map(|(i, _)| sma(&values[..=i], period))
        .collect()
}

/// Full EMA series.
#[allow(dead_code)]
pub fn ema_series(values: &[f64], period: usize) -> Vec<Option<f64>> {
    if values.len() < period || period == 0 {
        return vec![None; values.len()];
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
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sma_basic() {
        let vals = vec![1.0, 2.0, 3.0, 4.0, 5.0];
        assert_eq!(sma(&vals, 3), Some(4.0)); // (3+4+5)/3
        assert_eq!(sma(&vals, 5), Some(3.0)); // (1+2+3+4+5)/5
        assert_eq!(sma(&vals, 6), None);
    }

    #[test]
    fn ema_basic() {
        // EMA(3) seeded with SMA(3)=2.0, k=0.5
        let vals = vec![1.0, 2.0, 3.0, 4.0, 5.0];
        let result = ema(&vals, 3);
        assert!(result.is_some());
        // seed=2.0, ema(4)=0.5*(4-2)+2=3.0, ema(5)=0.5*(5-3)+3=4.0
        assert!((result.unwrap() - 4.0).abs() < 1e-10);
    }

    #[test]
    fn ema_series_matches_single() {
        let vals: Vec<f64> = (1..=20).map(|i| i as f64).collect();
        let series = ema_series(&vals, 5);
        assert_eq!(series[3], None);
        assert!(series[4].is_some());
        let single = ema(&vals, 5);
        assert!((series[19].unwrap() - single.unwrap()).abs() < 1e-10);
    }
}
