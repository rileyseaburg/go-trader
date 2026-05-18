//! Bollinger Bands.
//!
//! Middle = SMA(period), Upper/Lower = middle ± multiplier × σ


#[derive(Debug, Clone, PartialEq)]
pub struct BollingerResult {
    pub upper: f64,
    pub middle: f64,
    pub lower: f64,
    pub bandwidth: f64,   // (upper - lower) / middle
    pub percent_b: f64,   // (close - lower) / (upper - lower)
}

/// Compute Bollinger Bands from close prices.
///
/// Standard: SMA(20) ± 2σ.
/// Returns `None` if fewer than `period` values.
pub fn bollinger_bands(
    closes: &[f64],
    period: usize,
    multiplier: f64,
) -> Option<BollingerResult> {
    if closes.len() < period || period == 0 {
        return None;
    }

    let slice = &closes[closes.len() - period..];
    let middle = slice.iter().sum::<f64>() / period as f64;

    // Population standard deviation
    let variance: f64 =
        slice.iter().map(|v| (v - middle).powi(2)).sum::<f64>() / period as f64;
    let stddev = variance.sqrt();

    let upper = middle + multiplier * stddev;
    let lower = middle - multiplier * stddev;
    let bandwidth = if middle.abs() > 1e-10 {
        (upper - lower) / middle
    } else {
        0.0
    };

    let current_close = *closes.last()?;
    let percent_b = if (upper - lower).abs() > 1e-10 {
        (current_close - lower) / (upper - lower)
    } else {
        0.5
    };

    Some(BollingerResult {
        upper,
        middle,
        lower,
        bandwidth,
        percent_b,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bollinger_basic() {
        // Constant prices → bands equal SMA, zero bandwidth
        let closes = vec![100.0; 20];
        let r = bollinger_bands(&closes, 20, 2.0).unwrap();
        assert!((r.middle - 100.0).abs() < 1e-10);
        assert!((r.upper - 100.0).abs() < 1e-10);
        assert!((r.lower - 100.0).abs() < 1e-10);
        assert!(r.bandwidth.abs() < 1e-10);
    }

    #[test]
    fn bollinger_spread() {
        let closes: Vec<f64> = (90..=110).map(|i| i as f64).collect();
        let r = bollinger_bands(&closes, 20, 2.0).unwrap();
        assert!(r.upper > r.middle);
        assert!(r.lower < r.middle);
        assert!(r.bandwidth > 0.0);
        assert!(r.percent_b > 0.0 && r.percent_b < 1.0);
    }

    #[test]
    fn bollinger_too_few() {
        let closes = vec![1.0, 2.0, 3.0];
        assert!(bollinger_bands(&closes, 20, 2.0).is_none());
    }
}
