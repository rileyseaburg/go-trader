//! FRED client for economic data signals.

use chrono::{NaiveDate, Utc};
use serde::Deserialize;

use crate::feed::{compose_multiplier, DataFeed, DataSignal};

#[derive(Debug, Clone)]
pub struct FREDClient {
    api_key: String,
    http: reqwest::Client,
}

type SignalComputer = fn(&[f64], &[NaiveDate]) -> Option<DataSignal>;

#[derive(Deserialize)]
struct FredResp {
    observations: Vec<FredObs>,
}
#[derive(Deserialize)]
struct FredObs {
    date: String,
    value: String,
}

impl FREDClient {
    pub fn new(api_key: impl Into<String>) -> Self {
        Self {
            api_key: api_key.into(),
            http: reqwest::Client::new(),
        }
    }

    async fn fetch_observations(
        &self,
        series_id: &str,
        limit: usize,
    ) -> Result<Vec<FredObs>, Box<dyn std::error::Error + Send + Sync>> {
        let resp: FredResp = self
            .http
            .get("https://api.stlouisfed.org/fred/series/observations")
            .query(&[
                ("series_id", series_id),
                ("api_key", self.api_key.as_str()),
                ("file_type", "json"),
                ("sort_order", "desc"),
                ("limit", &limit.to_string()),
            ])
            .send()
            .await?
            .json()
            .await?;
        Ok(resp.observations)
    }

    fn parse_vals_dates(obs: &[FredObs]) -> (Vec<f64>, Vec<NaiveDate>) {
        obs.iter()
            .filter_map(|o| {
                if o.value == "." || o.value.is_empty() {
                    return None;
                }
                let v: f64 = o.value.parse().ok()?;
                let d = NaiveDate::parse_from_str(&o.date, "%Y-%m-%d").ok()?;
                Some((v, d))
            })
            .unzip()
    }

    fn sahm(vals: &[f64], dates: &[NaiveDate]) -> Option<DataSignal> {
        if vals.len() < 14 {
            return None;
        }
        let cur = (vals[0] + vals[1] + vals[2]) / 3.0;
        let mut min_mma = f64::INFINITY;
        for i in 1..=12 {
            if i + 2 >= vals.len() {
                break;
            }
            let mma = (vals[i] + vals[i + 1] + vals[i + 2]) / 3.0;
            if mma < min_mma {
                min_mma = mma;
            }
        }
        let delta = cur - min_mma;
        let as_of = dates.first()?.and_hms_opt(0, 0, 0)?.and_utc();
        Some(DataSignal {
            source: "FRED:UNRATE".into(),
            name: "Sahm Recession Indicator".into(),
            value: crate::round2(delta),
            threshold: 0.50,
            triggered: delta >= 0.50,
            as_of,
            description: format!("3M MA {:.2}% vs min {:.2}% (d={:+.2})", cur, min_mma, delta),
        })
    }

    fn yield_curve(vals: &[f64], dates: &[NaiveDate]) -> Option<DataSignal> {
        let v = vals.first()?;
        let as_of = dates.first()?.and_hms_opt(0, 0, 0)?.and_utc();
        Some(DataSignal {
            source: "FRED:T10Y2Y".into(),
            name: "10Y-2Y Treasury Spread".into(),
            value: crate::round2(*v),
            threshold: 0.0,
            triggered: *v < 0.0,
            as_of,
            description: format!("10Y-2Y = {:+.2}%", v),
        })
    }

    fn nfci(vals: &[f64], dates: &[NaiveDate]) -> Option<DataSignal> {
        let v = vals.first()?;
        let as_of = dates.first()?.and_hms_opt(0, 0, 0)?.and_utc();
        Some(DataSignal {
            source: "FRED:NFCI".into(),
            name: "Financial Conditions (NFCI)".into(),
            value: crate::round2(*v),
            threshold: 0.0,
            triggered: *v > 0.0,
            as_of,
            description: format!("NFCI = {:+.2}", v),
        })
    }

    fn hy_spread(vals: &[f64], dates: &[NaiveDate]) -> Option<DataSignal> {
        let v = vals.first()?;
        let as_of = dates.first()?.and_hms_opt(0, 0, 0)?.and_utc();
        Some(DataSignal {
            source: "FRED:BAMLH0A0HYM2".into(),
            name: "High-Yield Credit Spread (OAS)".into(),
            value: crate::round2(*v),
            threshold: 7.0,
            triggered: *v >= 7.0,
            as_of,
            description: format!("HY OAS = {:.2}%", v),
        })
    }

    fn ig_spread(vals: &[f64], dates: &[NaiveDate]) -> Option<DataSignal> {
        let v = vals.first()?;
        let as_of = dates.first()?.and_hms_opt(0, 0, 0)?.and_utc();
        Some(DataSignal {
            source: "FRED:BAMLC0A0CM".into(),
            name: "Investment-Grade Credit Spread (OAS)".into(),
            value: crate::round2(*v),
            threshold: 2.0,
            triggered: *v >= 2.0,
            as_of,
            description: format!("IG corporate OAS = {:.2}%", v),
        })
    }

    pub async fn snapshot(&self) -> Result<DataFeed, Box<dyn std::error::Error + Send + Sync>> {
        let mut feed = DataFeed {
            signals: Vec::new(),
            triggers: Vec::new(),
            multiplier: 1.0,
            warnings: Vec::new(),
            last_fetched: Utc::now(),
        };

        let series: &[(&str, usize, SignalComputer)] = &[
            ("UNRATE", 24, Self::sahm),
            ("T10Y2Y", 5, Self::yield_curve),
            ("NFCI", 5, Self::nfci),
            ("BAMLH0A0HYM2", 5, Self::hy_spread),
            ("BAMLC0A0CM", 5, Self::ig_spread),
        ];

        for (sid, limit, compute) in series {
            match self.fetch_observations(sid, *limit).await {
                Ok(obs) => {
                    let (vals, dates) = Self::parse_vals_dates(&obs);
                    if let Some(sig) = compute(&vals, &dates) {
                        feed.signals.push(sig);
                    }
                }
                Err(e) => feed.warnings.push(format!("{}: {}", sid, e)),
            }
        }

        (feed.multiplier, feed.triggers) = compose_multiplier(&feed.signals);
        Ok(feed)
    }
}
