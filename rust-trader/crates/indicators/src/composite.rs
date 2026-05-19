//! Composite indicator computation — compute the full `IndicatorSet` from bars.

use crate::types::*;
use crate::moving_average::{sma, ema};
use crate::rsi::rsi;
use crate::macd::macd;
use crate::bollinger::bollinger_bands;
use crate::atr::atr;
use crate::vwap::vwap;
use crate::adx::adx;
use crate::stochastic::stochastic;

/// Compute the full indicator set from a bar series.
///
/// Any indicator that lacks sufficient data will remain `None` / `Neutral`.
pub fn compute_all(bars: &[Bar]) -> IndicatorSet {
    let closes: Vec<f64> = bars.iter().map(|b| b.close).collect();
    let highs: Vec<f64> = bars.iter().map(|b| b.high).collect();
    let lows: Vec<f64> = bars.iter().map(|b| b.low).collect();
    let volumes: Vec<f64> = bars.iter().map(|b| b.volume).collect();

    let mut set = IndicatorSet::default();

    // Moving averages
    set.sma_10 = sma(&closes, 10);
    set.sma_20 = sma(&closes, 20);
    set.sma_50 = sma(&closes, 50);
    set.sma_200 = sma(&closes, 200);
    set.ema_9 = ema(&closes, 9);
    set.ema_12 = ema(&closes, 12);
    set.ema_21 = ema(&closes, 21);
    set.ema_26 = ema(&closes, 26);
    set.ema_50 = ema(&closes, 50);
    set.ema_200 = ema(&closes, 200);

    // RSI
    set.rsi_14 = rsi(&closes, 14);

    // MACD
    if let Some(m) = macd(&closes) {
        set.macd_line = Some(m.macd_line);
        set.macd_signal = Some(m.signal_line);
        set.macd_histogram = Some(m.histogram);
    }

    // Bollinger Bands (20, 2σ)
    if let Some(bb) = bollinger_bands(&closes, 20, 2.0) {
        set.bb_upper = Some(bb.upper);
        set.bb_middle = Some(bb.middle);
        set.bb_lower = Some(bb.lower);
        set.bb_bandwidth = Some(bb.bandwidth);
        set.bb_percent_b = Some(bb.percent_b);
    }

    // ATR
    set.atr_14 = atr(&highs, &lows, &closes, 14);

    // VWAP
    set.vwap = vwap(bars);

    // ADX
    if let Some(a) = adx(&highs, &lows, &closes, 14) {
        set.adx_14 = Some(a.adx);
        set.plus_di_14 = Some(a.plus_di);
        set.minus_di_14 = Some(a.minus_di);
    }

    // Stochastic
    if let Some(st) = stochastic(&highs, &lows, &closes, 14, 3, 3) {
        set.stoch_k = Some(st.percent_k);
        set.stoch_d = Some(st.percent_d);
    }

    // Volume surge ratio: latest bar volume relative to the trailing average.
    // Exclude the latest bar from the baseline so spikes are not diluted by
    // the very bar we are trying to classify.
    if volumes.len() >= 21 {
        let latest = *volumes.last().unwrap_or(&0.0);
        let baseline: Vec<f64> = volumes[..volumes.len() - 1]
            .iter()
            .rev()
            .take(20)
            .copied()
            .filter(|v| *v > 0.0)
            .collect();
        if !baseline.is_empty() && latest > 0.0 {
            let avg = baseline.iter().sum::<f64>() / baseline.len() as f64;
            if avg > f64::EPSILON {
                set.volume_ratio = Some(latest / avg);
            }
        }
    }

    // --- Derived signals ---
    derive_signals(&mut set, &closes);

    set
}

/// Derive trend/volatility/momentum from computed indicators.
fn derive_signals(set: &mut IndicatorSet, closes: &[f64]) {
    let _price = closes.last().copied().unwrap_or(0.0);

    // Trend: EMA crossover + ADX
    set.trend = match (set.ema_9, set.ema_21, set.adx_14) {
        (Some(ema9), Some(ema21), Some(adx)) => {
            if ema9 > ema21 && adx > 25.0 {
                TrendDirection::Bullish
            } else if ema9 < ema21 && adx > 25.0 {
                TrendDirection::Bearish
            } else {
                TrendDirection::Neutral
            }
        }
        _ => TrendDirection::Neutral,
    };

    // Volatility: from Bollinger Bandwidth
    set.volatility = match set.bb_bandwidth {
        Some(bw) if bw > 0.08 => VolatilityLevel::Extreme,
        Some(bw) if bw > 0.05 => VolatilityLevel::High,
        Some(bw) if bw > 0.035 => VolatilityLevel::Elevated,
        Some(bw) if bw < 0.02 => VolatilityLevel::Low,
        Some(_) => VolatilityLevel::Normal,
        None => VolatilityLevel::Normal,
    };

    // Momentum: from RSI
    set.momentum = match set.rsi_14 {
        Some(rsi) if rsi > 70.0 => MomentumState::Overbought,
        Some(rsi) if rsi < 30.0 => MomentumState::Oversold,
        Some(rsi) if rsi > 55.0 => MomentumState::Rising,
        Some(rsi) if rsi < 45.0 => MomentumState::Falling,
        _ => MomentumState::Neutral,
    };

    // Clamp momentum if Bollinger %B confirms
    if let Some(pb) = set.bb_percent_b {
        if matches!(set.momentum, MomentumState::Overbought) && pb < 0.8 {
            set.momentum = MomentumState::Rising;
        }
        if matches!(set.momentum, MomentumState::Oversold) && pb > 0.2 {
            set.momentum = MomentumState::Falling;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_bars(count: usize, start_price: f64, trend: f64) -> Vec<Bar> {
        (0..count)
            .map(|i| {
                let close = start_price + (i as f64) * trend;
                Bar {
                    timestamp: i as i64 * 60_000,
                    open: close - 0.1,
                    high: close + 1.0,
                    low: close - 1.0,
                    close,
                    volume: 1000.0,
                }
            })
            .collect()
    }

    #[test]
    fn compute_all_uptrend() {
        let bars = make_bars(50, 100.0, 0.5);
        let set = compute_all(&bars);
        assert!(set.sma_20.is_some());
        assert!(set.ema_12.is_some());
        assert!(set.rsi_14.is_some());
        assert!(set.macd_line.is_some());
        assert!(set.bb_upper.is_some());
        assert!(set.atr_14.is_some());
        assert!(set.vwap.is_some());
        assert_eq!(set.trend, TrendDirection::Bullish);
    }

    #[test]
    fn compute_all_downtrend() {
        let bars = make_bars(50, 150.0, -0.5);
        let set = compute_all(&bars);
        assert_eq!(set.trend, TrendDirection::Bearish);
    }

    #[test]
    fn compute_all_too_few() {
        let bars = make_bars(5, 100.0, 0.0);
        let set = compute_all(&bars);
        assert!(set.sma_200.is_none());
        assert!(set.ema_200.is_none());
        assert_eq!(set.trend, TrendDirection::Neutral);
        assert!(set.volume_ratio.is_none());
    }

    #[test]
    fn compute_all_sets_volume_ratio_from_latest_vs_prior_average() {
        let mut bars = make_bars(25, 100.0, 0.1);
        for bar in &mut bars[..24] {
            bar.volume = 100.0;
        }
        bars[24].volume = 250.0;

        let set = compute_all(&bars);
        assert_eq!(set.volume_ratio, Some(2.5));
    }
}
