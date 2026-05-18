//! Average True Range (ATR).
//!
/// Wilder's ATR over `period` bars.
///
/// True Range = max(H-L, |H-prev_close|, |L-prev_close|)
/// Returns `None` if fewer than 2 bars (need at least one TR).
pub fn atr(highs: &[f64], lows: &[f64], closes: &[f64], period: usize) -> Option<f64> {
    if highs.len() != lows.len() || highs.len() != closes.len() {
        return None;
    }
    if highs.len() < 2 || period == 0 {
        return None;
    }

    let tr_values: Vec<f64> = closes
        .windows(2)
        .enumerate()
        .map(|(i, w)| {
            let h = highs[i + 1];
            let l = lows[i + 1];
            let prev_close = w[0];
            let tr1 = h - l;
            let tr2 = (h - prev_close).abs();
            let tr3 = (l - prev_close).abs();
            tr1.max(tr2).max(tr3)
        })
        .collect();

    if tr_values.len() < period {
        // Fall back to simple average of available TRs
        return Some(tr_values.iter().sum::<f64>() / tr_values.len() as f64);
    }

    // Seed with SMA of first `period` TRs
    let mut atr_val: f64 = tr_values[..period].iter().sum::<f64>() / period as f64;

    // Wilder's smoothing for the rest
    for tr in &tr_values[period..] {
        atr_val = (atr_val * (period as f64 - 1.0) + tr) / period as f64;
    }

    Some(atr_val)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn atr_basic() {
        // Constant prices → ATR = 0
        let price = vec![100.0; 20];
        let r = atr(&price, &price, &price, 14);
        assert!(r.is_some());
        assert!(r.unwrap().abs() < 1e-10);
    }

    #[test]
    fn atr_with_range() {
        let closes = vec![44.0, 44.15, 44.09, 43.62, 44.35, 44.84, 45.10, 45.38,
                          44.96, 45.28, 45.43, 45.56, 45.44, 45.67, 45.80, 45.19];
        let highs: Vec<f64> = closes.iter().map(|c| c + 0.5).collect();
        let lows: Vec<f64> = closes.iter().map(|c| c - 0.3).collect();
        let r = atr(&highs, &lows, &closes, 14);
        assert!(r.is_some());
        assert!(r.unwrap() > 0.0);
    }

    #[test]
    fn atr_too_few() {
        assert!(atr(&[100.0], &[100.0], &[100.0], 14).is_none());
    }
}
