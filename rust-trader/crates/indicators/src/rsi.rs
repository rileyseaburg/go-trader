//! Relative Strength Index (RSI).
//!
/// Wilder's smoothed RSI over `period` bars.
/// Returns `None` if fewer than `period + 1` bars provided.
pub fn rsi(closes: &[f64], period: usize) -> Option<f64> {
    if closes.len() < period + 1 || period == 0 {
        return None;
    }

    let deltas: Vec<f64> = closes
        .windows(2)
        .map(|w| w[1] - w[0])
        .collect();

    // First `period` deltas used to seed average gain/loss
    let (init_gains, init_losses): (Vec<f64>, Vec<f64>) =
        deltas[..period].iter().partition(|&&d| d > 0.0);

    let mut avg_gain: f64 = init_gains.iter().sum::<f64>() / period as f64;
    let mut avg_loss: f64 = init_losses.iter().map(|l| l.abs()).sum::<f64>() / period as f64;

    // Smooth remaining deltas (Wilder's smoothing)
    for delta in &deltas[period..] {
        let gain = if *delta > 0.0 { *delta } else { 0.0 };
        let loss = if *delta < 0.0 { delta.abs() } else { 0.0 };
        avg_gain = (avg_gain * (period as f64 - 1.0) + gain) / period as f64;
        avg_loss = (avg_loss * (period as f64 - 1.0) + loss) / period as f64;
    }

    if avg_loss < 1e-10 {
        return Some(100.0);
    }
    let rs = avg_gain / avg_loss;
    Some(100.0 - (100.0 / (1.0 + rs)))
}

/// Full RSI series (one per bar, None until enough data).
#[allow(dead_code)]
pub fn rsi_series(closes: &[f64], period: usize) -> Vec<Option<f64>> {
    if closes.len() < period + 1 {
        return vec![None; closes.len()];
    }
    let mut result = vec![None; closes.len()];

    let deltas: Vec<f64> = closes
        .windows(2)
        .map(|w| w[1] - w[0])
        .collect();

    let (init_gains, init_losses): (Vec<f64>, Vec<f64>) =
        deltas[..period].iter().partition(|&&d| d > 0.0);

    let mut avg_gain: f64 = init_gains.iter().sum::<f64>() / period as f64;
    let mut avg_loss: f64 = init_losses.iter().map(|l| l.abs()).sum::<f64>() / period as f64;

    // First RSI value at index `period`
    let rs = if avg_loss < 1e-10 {
        f64::INFINITY
    } else {
        avg_gain / avg_loss
    };
    result[period] = Some(if avg_loss < 1e-10 {
        100.0
    } else {
        100.0 - (100.0 / (1.0 + rs))
    });

    // Smooth remaining
    for (i, delta) in deltas[period..].iter().enumerate() {
        let gain = if *delta > 0.0 { *delta } else { 0.0 };
        let loss = if *delta < 0.0 { delta.abs() } else { 0.0 };
        avg_gain = (avg_gain * (period as f64 - 1.0) + gain) / period as f64;
        avg_loss = (avg_loss * (period as f64 - 1.0) + loss) / period as f64;
        let rsi_val = if avg_loss < 1e-10 {
            100.0
        } else {
            100.0 - (100.0 / (1.0 + avg_gain / avg_loss))
        };
        result[period + 1 + i] = Some(rsi_val);
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rsi_all_up() {
        // Monotonically increasing → RSI should be very high
        let closes: Vec<f64> = (1..=20).map(|i| i as f64).collect();
        let r = rsi(&closes, 14);
        assert!(r.is_some());
        assert!(r.unwrap() > 90.0);
    }

    #[test]
    fn rsi_all_down() {
        let closes: Vec<f64> = (1..=20).rev().map(|i| i as f64).collect();
        let r = rsi(&closes, 14);
        assert!(r.is_some());
        assert!(r.unwrap() < 10.0);
    }

    #[test]
    fn rsi_too_few_bars() {
        let closes = vec![100.0, 101.0];
        assert!(rsi(&closes, 14).is_none());
    }

    #[test]
    fn rsi_range() {
        let closes = vec![
            44.0, 44.15, 44.09, 43.62, 44.35, 44.84, 45.10, 45.38,
            44.96, 45.28, 45.43, 45.56, 45.44, 45.67, 45.80, 45.19,
        ];
        let r = rsi(&closes, 14);
        assert!(r.is_some());
        let val = r.unwrap();
        assert!(val > 0.0 && val < 100.0, "RSI should be in (0, 100), got {}", val);
    }
}
