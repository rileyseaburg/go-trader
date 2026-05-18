//! VWAP (Volume-Weighted Average Price).
//!
/// Compute VWAP from bars.
///
/// VWAP = cumulative(typical_price × volume) / cumulative(volume)
/// Typical price = (high + low + close) / 3
pub fn vwap(bars: &[crate::types::Bar]) -> Option<f64> {
    if bars.is_empty() {
        return None;
    }

    let mut cumulative_tp_vol = 0.0;
    let mut cumulative_vol = 0.0;

    for bar in bars {
        let typical = (bar.high + bar.low + bar.close) / 3.0;
        cumulative_tp_vol += typical * bar.volume;
        cumulative_vol += bar.volume;
    }

    if cumulative_vol.abs() < 1e-10 {
        return None;
    }

    Some(cumulative_tp_vol / cumulative_vol)
}

/// Incremental VWAP update — feed a new bar into existing accumulators.
#[allow(dead_code)]
pub struct VwapAccumulator {
    cumulative_tp_vol: f64,
    cumulative_vol: f64,
}

impl VwapAccumulator {
    #[allow(dead_code)]
    pub fn new() -> Self {
        Self {
            cumulative_tp_vol: 0.0,
            cumulative_vol: 0.0,
        }
    }

    #[allow(dead_code)]
    pub fn update(&mut self, bar: &crate::types::Bar) {
        let typical = (bar.high + bar.low + bar.close) / 3.0;
        self.cumulative_tp_vol += typical * bar.volume;
        self.cumulative_vol += bar.volume;
    }

    #[allow(dead_code)]
    pub fn value(&self) -> Option<f64> {
        if self.cumulative_vol.abs() < 1e-10 {
            None
        } else {
            Some(self.cumulative_tp_vol / self.cumulative_vol)
        }
    }

    /// Reset for a new session (VWAP typically resets daily).
    #[allow(dead_code)]
    pub fn reset(&mut self) {
        self.cumulative_tp_vol = 0.0;
        self.cumulative_vol = 0.0;
    }
}

impl Default for VwapAccumulator {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::Bar;

    fn bar(close: f64, vol: f64) -> Bar {
        Bar {
            timestamp: 0,
            open: close - 0.1,
            high: close + 0.5,
            low: close - 0.5,
            close,
            volume: vol,
        }
    }

    #[test]
    fn vwap_uniform_volume() {
        let bars = vec![bar(100.0, 1000.0), bar(101.0, 1000.0), bar(102.0, 1000.0)];
        let v = vwap(&bars).unwrap();
        // typical prices: 100.0, 101.0, 102.0 with equal volume → vwap = avg of typicals
        let expected = (100.0 + 101.0 + 102.0) / 3.0;
        assert!((v - expected).abs() < 1e-10);
    }

    #[test]
    fn vwap_accumulator_matches() {
        let bars = vec![bar(50.0, 200.0), bar(52.0, 300.0), bar(51.0, 500.0)];
        let batch_vwap = vwap(&bars).unwrap();

        let mut acc = VwapAccumulator::new();
        for b in &bars {
            acc.update(b);
        }
        assert!((acc.value().unwrap() - batch_vwap).abs() < 1e-10);
    }
}
