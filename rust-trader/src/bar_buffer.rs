//! In-memory bar buffer — ring buffer per symbol for OHLCV data.
//!
//! Accumulates bars from the Alpaca market data stream and periodic
//! historical refreshes, providing a sliding window for indicator
//! computation.

#![allow(dead_code)]

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use go_trader_indicators::Bar;
use tracing::debug;

/// Default max bars per symbol (covers 200-SMA with headroom).
const DEFAULT_CAPACITY: usize = 300;

pub struct BarBuffer {
    capacity: usize,
    buffers: Arc<RwLock<HashMap<String, Vec<Bar>>>>,
}

impl BarBuffer {
    pub fn new(capacity: usize) -> Self {
        Self {
            capacity,
            buffers: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub fn with_default_capacity() -> Self {
        Self::new(DEFAULT_CAPACITY)
    }

    /// Append a single bar. Deduplicates by timestamp.
    pub fn push(&self, symbol: &str, bar: Bar) {
        let mut buffers = self.buffers.write().unwrap();
        let buf = buffers.entry(symbol.to_string()).or_insert_with(Vec::new);

        // Dedup: if last bar has same timestamp, update in place
        if let Some(last) = buf.last_mut() {
            if last.timestamp == bar.timestamp {
                *last = bar;
                return;
            }
        }

        buf.push(bar);

        // Trim to capacity (keep the most recent)
        if buf.len() > self.capacity {
            let excess = buf.len() - self.capacity;
            buf.drain(..excess);
        }
    }

    /// Bulk-load bars (e.g. from a historical fetch on startup).
    /// Merges with existing data, deduplicating by timestamp.
    pub fn load_bars(&self, symbol: &str, bars: Vec<Bar>) {
        if bars.is_empty() {
            return;
        }
        let mut buffers = self.buffers.write().unwrap();
        let buf = buffers.entry(symbol.to_string()).or_insert_with(Vec::new);

        // Build a set of existing timestamps for dedup
        let existing_ts: std::collections::HashSet<i64> =
            buf.iter().map(|b| b.timestamp).collect();

        for bar in bars {
            if !existing_ts.contains(&bar.timestamp) {
                buf.push(bar);
            }
        }

        // Sort by timestamp ascending
        buf.sort_by_key(|b| b.timestamp);

        // Trim to capacity
        if buf.len() > self.capacity {
            let excess = buf.len() - self.capacity;
            buf.drain(..excess);
        }

        debug!(
            "BarBuffer loaded {} bars for {} (total: {})",
            buf.len(),
            symbol,
            buf.len()
        );
    }

    /// Get bars for a symbol (read-only snapshot).
    pub fn get_bars(&self, symbol: &str) -> Vec<Bar> {
        self.buffers
            .read()
            .unwrap()
            .get(symbol)
            .cloned()
            .unwrap_or_default()
    }

    /// Get the last N bars for a symbol.
    pub fn get_recent(&self, symbol: &str, count: usize) -> Vec<Bar> {
        let buffers = self.buffers.read().unwrap();
        match buffers.get(symbol) {
            Some(buf) => {
                let start = buf.len().saturating_sub(count);
                buf[start..].to_vec()
            }
            None => vec![],
        }
    }

    /// Get the latest bar for a symbol.
    pub fn latest(&self, symbol: &str) -> Option<Bar> {
        self.buffers
            .read()
            .unwrap()
            .get(symbol)
            .and_then(|buf| buf.last())
            .cloned()
    }

    /// Number of bars stored for a symbol.
    pub fn len(&self, symbol: &str) -> usize {
        self.buffers
            .read()
            .unwrap()
            .get(symbol)
            .map(|b| b.len())
            .unwrap_or(0)
    }

    /// List all symbols with stored bars.
    pub fn symbols(&self) -> Vec<String> {
        self.buffers
            .read()
            .unwrap()
            .keys()
            .cloned()
            .collect()
    }

    /// Clear all bars for a symbol.
    pub fn clear(&self, symbol: &str) {
        if let Some(buf) = self.buffers.write().unwrap().get_mut(symbol) {
            buf.clear();
        }
    }

    /// Clear all data.
    pub fn clear_all(&self) {
        self.buffers.write().unwrap().clear();
    }
}

impl Default for BarBuffer {
    fn default() -> Self {
        Self::with_default_capacity()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn bar(ts: i64, close: f64) -> Bar {
        Bar {
            timestamp: ts,
            open: close - 0.5,
            high: close + 1.0,
            low: close - 1.0,
            close,
            volume: 1000.0,
        }
    }

    #[test]
    fn push_and_get() {
        let buf = BarBuffer::new(10);
        buf.push("AAPL", bar(1, 100.0));
        buf.push("AAPL", bar(2, 101.0));
        assert_eq!(buf.len("AAPL"), 2);
        assert_eq!(buf.latest("AAPL").unwrap().close, 101.0);
    }

    #[test]
    fn dedup_by_timestamp() {
        let buf = BarBuffer::new(10);
        buf.push("AAPL", bar(1, 100.0));
        buf.push("AAPL", bar(1, 102.0)); // same ts → update
        assert_eq!(buf.len("AAPL"), 1);
        assert_eq!(buf.latest("AAPL").unwrap().close, 102.0);
    }

    #[test]
    fn capacity_trim() {
        let buf = BarBuffer::new(5);
        for i in 0..10 {
            buf.push("AAPL", bar(i, 100.0 + i as f64));
        }
        assert_eq!(buf.len("AAPL"), 5);
        // Should keep bars 5..9
        assert_eq!(buf.latest("AAPL").unwrap().timestamp, 9);
        assert_eq!(buf.get_bars("AAPL")[0].timestamp, 5);
    }

    #[test]
    fn load_merges() {
        let buf = BarBuffer::new(100);
        buf.push("AAPL", bar(1, 100.0));
        buf.push("AAPL", bar(3, 103.0));
        buf.load_bars("AAPL", vec![bar(2, 101.0), bar(4, 104.0)]);
        assert_eq!(buf.len("AAPL"), 4);
        // Should be sorted
        let bars = buf.get_bars("AAPL");
        assert_eq!(bars[0].timestamp, 1);
        assert_eq!(bars[3].timestamp, 4);
    }
}
