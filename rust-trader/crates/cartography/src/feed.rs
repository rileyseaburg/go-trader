//! Feed types, cache, and multiplier composition.

use chrono::{DateTime, Duration, Utc};
use serde::{Deserialize, Serialize};
use std::sync::{Arc, RwLock};

use crate::fred::FREDClient;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataSignal {
    pub source: String,
    pub name: String,
    pub value: f64,
    pub threshold: f64,
    pub triggered: bool,
    pub as_of: DateTime<Utc>,
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataFeed {
    pub signals: Vec<DataSignal>,
    pub triggers: Vec<String>,
    pub multiplier: f64,
    pub warnings: Vec<String>,
    pub last_fetched: DateTime<Utc>,
}

fn signal_haircut(name: &str) -> f64 {
    match name {
        "Sahm Recession Indicator" => 0.40,
        "High-Yield Credit Spread (OAS)" => 0.50,
        "Investment-Grade Credit Spread (OAS)" => 0.75,
        "10Y-2Y Treasury Spread" => 0.85,
        "Financial Conditions (NFCI)" => 0.85,
        _ => 1.0,
    }
}

pub fn compose_multiplier(signals: &[DataSignal]) -> (f64, Vec<String>) {
    let mut mult = 1.0;
    let mut fired = Vec::new();
    for s in signals {
        if s.triggered {
            mult *= signal_haircut(&s.name);
            fired.push(s.name.clone());
        }
    }
    fired.sort();
    (mult, fired)
}

pub fn applied_multiplier(formula: f64, feed: Option<&DataFeed>) -> f64 {
    match feed {
        Some(f) if f.multiplier < formula => f.multiplier,
        _ => formula,
    }
}

pub struct FeedCache {
    #[allow(dead_code)]
    client: FREDClient,
    current: Arc<RwLock<Option<DataFeed>>>,
    #[allow(dead_code)]
    max_age: Duration,
}

impl FeedCache {
    pub fn new(client: FREDClient, max_age: Duration) -> Self {
        Self {
            client,
            current: Arc::new(RwLock::new(None)),
            max_age: if max_age.num_seconds() == 0 {
                Duration::hours(6)
            } else {
                max_age
            },
        }
    }

    pub fn get(&self) -> Option<DataFeed> {
        self.current.read().ok()?.clone()
    }

    pub fn get_fresh(&self, now: DateTime<Utc>) -> Option<DataFeed> {
        let feed = self.get()?;
        if now.signed_duration_since(feed.last_fetched) <= self.max_age {
            Some(feed)
        } else {
            None
        }
    }

    pub async fn refresh(&self) -> Result<DataFeed, Box<dyn std::error::Error + Send + Sync>> {
        let feed = self.client.snapshot().await?;
        *self.current.write().unwrap() = Some(feed.clone());
        Ok(feed)
    }
}
