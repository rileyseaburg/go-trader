//! Stochastic Oscillator (%K and %D).
//!
//! %K = (close - lowest_low) / (highest_high - lowest_low) × 100
//! %D = SMA(smooth_period) of %K

#[derive(Debug, Clone, PartialEq)]
pub struct StochasticResult {
    pub percent_k: f64,
    pub percent_d: f64,
}

/// Compute Stochastic Oscillator.
///
/// - `k_period`: lookback for highest high / lowest low (typically 14)
/// - `k_smooth`: smoothing of raw %K (typically 3)
/// - `d_period`: SMA period for %D line (typically 3)
///
/// Returns `None` if not enough data.
pub fn stochastic(
    highs: &[f64],
    lows: &[f64],
    closes: &[f64],
    k_period: usize,
    k_smooth: usize,
    d_period: usize,
) -> Option<StochasticResult> {
    if highs.len() != lows.len() || highs.len() != closes.len() {
        return None;
    }
    if closes.len() < k_period || k_period == 0 {
        return None;
    }

    // 1. Compute raw %K for each window
    let mut raw_k: Vec<f64> = Vec::with_capacity(closes.len() + 1 - k_period);
    for i in (k_period - 1)..closes.len() {
        let start = i + 1 - k_period; // safe: i >= k_period - 1
        let hh = highs[start..=i]
            .iter()
            .cloned()
            .fold(f64::NEG_INFINITY, f64::max);
        let ll = lows[start..=i]
            .iter()
            .cloned()
            .fold(f64::INFINITY, f64::min);
        let k = if (hh - ll).abs() > 1e-10 {
            (closes[i] - ll) / (hh - ll) * 100.0
        } else {
            50.0
        };
        raw_k.push(k);
    }

    // 2. Smooth %K with SMA(k_smooth)
    if raw_k.len() < k_smooth {
        let k = raw_k.last().copied().unwrap_or(50.0);
        return Some(StochasticResult {
            percent_k: k,
            percent_d: k,
        });
    }

    let smoothed_k: Vec<f64> = raw_k
        .windows(k_smooth)
        .map(|w| w.iter().sum::<f64>() / k_smooth as f64)
        .collect();

    let k_val = *smoothed_k.last()?;

    // 3. %D = SMA(d_period) of smoothed %K
    let d_val = if smoothed_k.len() >= d_period {
        let slice = &smoothed_k[smoothed_k.len() + 1 - d_period..];
        slice.iter().sum::<f64>() / d_period as f64
    } else {
        k_val
    };

    Some(StochasticResult {
        percent_k: k_val,
        percent_d: d_val,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn stochastic_at_high() {
        // Close at highest high → %K should be ~100
        let closes: Vec<f64> = (1..=20).map(|i| i as f64).collect();
        let highs = closes.clone();
        let lows: Vec<f64> = closes.iter().map(|c| c - 1.0).collect();
        let r = stochastic(&highs, &lows, &closes, 14, 3, 3).unwrap();
        assert!(
            r.percent_k > 90.0,
            "close at high should give high %K, got {}",
            r.percent_k
        );
    }

    #[test]
    fn stochastic_at_low() {
        // Close at lowest low → %K should be ~0
        let closes: Vec<f64> = (1..=20).rev().map(|i| i as f64).collect();
        let highs: Vec<f64> = closes.iter().map(|c| c + 1.0).collect();
        let lows = closes.clone();
        let r = stochastic(&highs, &lows, &closes, 14, 3, 3).unwrap();
        assert!(
            r.percent_k < 10.0,
            "close at low should give low %K, got {}",
            r.percent_k
        );
    }

    #[test]
    fn stochastic_too_few() {
        let v = vec![100.0; 5];
        assert!(stochastic(&v, &v, &v, 14, 3, 3).is_none());
    }
}
