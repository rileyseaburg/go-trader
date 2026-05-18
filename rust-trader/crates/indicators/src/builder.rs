//! Test builder for `IndicatorSet`.

use crate::types::*;

/// Builder for constructing `IndicatorSet` in unit tests.
#[derive(Debug, Clone, Default)]
pub struct IndicatorSetBuilder {
    inner: IndicatorSet,
}

impl IndicatorSetBuilder {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_adx(mut self, adx: f64) -> Self {
        self.inner.adx_14 = Some(adx);
        self.inner.plus_di_14 = Some(25.0);
        self.inner.minus_di_14 = Some(20.0);
        self
    }

    pub fn with_sma_trend(mut self, trend: TrendDirection) -> Self {
        self.inner.trend = trend;
        self
    }

    pub fn with_macd_signal(mut self, signal: MACDSignal) -> Self {
        match signal {
            MACDSignal::BullishCross => {
                self.inner.macd_line = Some(1.0);
                self.inner.macd_signal = Some(0.5);
                self.inner.macd_histogram = Some(-0.1);
            }
            MACDSignal::BullishAbove => {
                self.inner.macd_line = Some(1.0);
                self.inner.macd_signal = Some(0.5);
                self.inner.macd_histogram = Some(0.5);
            }
            MACDSignal::BearishCross => {
                self.inner.macd_line = Some(-1.0);
                self.inner.macd_signal = Some(-0.5);
                self.inner.macd_histogram = Some(0.1);
            }
            MACDSignal::BearishBelow => {
                self.inner.macd_line = Some(-1.0);
                self.inner.macd_signal = Some(-0.5);
                self.inner.macd_histogram = Some(-0.5);
            }
            MACDSignal::Neutral => {
                self.inner.macd_line = Some(0.0);
                self.inner.macd_signal = Some(0.0);
                self.inner.macd_histogram = Some(0.0);
            }
        }
        self
    }

    pub fn with_macd_value(mut self, val: f64) -> Self {
        self.inner.macd_line = Some(val);
        self
    }

    pub fn with_rsi(mut self, rsi: f64) -> Self {
        self.inner.rsi_14 = Some(rsi);
        self
    }

    pub fn with_bb(mut self, lower: f64, upper: f64) -> Self {
        let mid = (upper + lower) / 2.0;
        self.inner.bb_lower = Some(lower);
        self.inner.bb_upper = Some(upper);
        self.inner.bb_middle = Some(mid);
        self.inner.bb_bandwidth = Some((upper - lower) / mid);
        self
    }

    pub fn with_volatility_level(mut self, level: VolatilityLevel) -> Self {
        self.inner.volatility = level;
        self
    }

    pub fn with_momentum_state(mut self, state: MomentumState) -> Self {
        self.inner.momentum = state;
        self
    }

    pub fn with_volume_ratio(mut self, ratio: f64) -> Self {
        self.inner.volume_ratio = Some(ratio);
        self
    }

    pub fn build(self) -> IndicatorSet {
        self.inner
    }
}
