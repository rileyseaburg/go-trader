//! Signal watcher — tracks state transitions in data signals.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::feed::{DataFeed, DataSignal};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum ChangeKind {
    Triggered,
    Cleared,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChangeEvent {
    pub kind: ChangeKind,
    pub signal: DataSignal,
    pub previous: Option<DataSignal>,
    pub observed_at: DateTime<Utc>,
}

pub struct SignalWatcher {
    prior: Arc<RwLock<HashMap<String, DataSignal>>>,
    seen: Arc<RwLock<bool>>,
}

impl Default for SignalWatcher {
    fn default() -> Self {
        Self::new()
    }
}

impl SignalWatcher {
    pub fn new() -> Self {
        Self {
            prior: Arc::new(RwLock::new(HashMap::new())),
            seen: Arc::new(RwLock::new(false)),
        }
    }

    pub fn observe(&self, feed: &DataFeed) -> Vec<ChangeEvent> {
        let mut prior = self.prior.write().unwrap();
        let seen = *self.seen.read().unwrap();
        let now = if feed.last_fetched.timestamp() == 0 {
            Utc::now()
        } else {
            feed.last_fetched
        };

        let mut events = Vec::new();
        for s in &feed.signals {
            if let Some(prev) = prior.get(&s.source) {
                if seen && prev.triggered != s.triggered {
                    events.push(ChangeEvent {
                        kind: if s.triggered {
                            ChangeKind::Triggered
                        } else {
                            ChangeKind::Cleared
                        },
                        signal: s.clone(),
                        previous: Some(prev.clone()),
                        observed_at: now,
                    });
                }
            }
            prior.insert(s.source.clone(), s.clone());
        }
        *self.seen.write().unwrap() = true;
        events.sort_by(|a, b| a.signal.source.cmp(&b.signal.source));
        events
    }

    pub fn snapshot(&self) -> HashMap<String, DataSignal> {
        self.prior.read().unwrap().clone()
    }
}
