//! go-trader-cartography: Economic cartography model, FRED client, Vault loader, signal watcher.

pub mod feed;
pub mod fred;
pub mod vault;
pub mod watcher;

use chrono::{DateTime, Datelike, Utc};
use serde::{Deserialize, Serialize};
use std::f64::consts::PI;

const EPOCH: f64 = 2000.0;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Band {
    pub key: String,
    pub name: String,
    pub period: f64,
    pub amplitude: f64,
    pub phase: f64,
    pub color: String,
    pub description: String,
    pub range: String,
}

pub fn bands() -> Vec<Band> {
    vec![
        Band {
            key: "kondratiev".into(),
            name: "Kondratiev".into(),
            period: 52.0,
            amplitude: 1.10,
            phase: 0.25,
            color: "#c2410c".into(),
            description: "Techno-economic paradigm".into(),
            range: "40-60y".into(),
        },
        Band {
            key: "kuznets".into(),
            name: "Kuznets".into(),
            period: 22.0,
            amplitude: 0.70,
            phase: 0.36,
            color: "#a16207".into(),
            description: "Infrastructure & demographics".into(),
            range: "15-25y".into(),
        },
        Band {
            key: "juglar".into(),
            name: "Juglar".into(),
            period: 11.5,
            amplitude: 0.90,
            phase: 0.01,
            color: "#0e7490".into(),
            description: "Capital & credit".into(),
            range: "7-11y".into(),
        },
        Band {
            key: "kitchin".into(),
            name: "Kitchin".into(),
            period: 4.0,
            amplitude: 0.35,
            phase: 0.00,
            color: "#4d7c0f".into(),
            description: "Inventory".into(),
            range: "3-5y".into(),
        },
    ]
}

pub fn time_to_t(at: DateTime<Utc>) -> f64 {
    (at.year() as f64 + (at.ordinal() as f64 - 1.0) / 365.0) - EPOCH
}

pub fn band_value(b: &Band, t: f64) -> f64 {
    b.amplitude * (2.0 * PI * (t / b.period + b.phase)).sin()
}

pub fn band_phase_deg(b: &Band, t: f64) -> f64 {
    let mut p = ((t / b.period + b.phase) % 1.0) * 360.0;
    if p < 0.0 {
        p += 360.0;
    }
    p
}

fn describe_phase(deg: f64) -> (&'static str, &'static str) {
    match deg {
        d if d < 45.0 => ("ASCENDING", "rising from midline"),
        d if d < 90.0 => ("CRESTING", "approaching peak"),
        d if d < 135.0 => ("CRESTING", "just past peak"),
        d if d < 225.0 => ("DESCENDING", "falling toward midline"),
        d if d < 270.0 => ("TROUGHING", "approaching trough"),
        d if d < 315.0 => ("TROUGHING", "just past trough"),
        _ => ("ASCENDING", "rising toward midline"),
    }
}

fn composite(t: f64) -> f64 {
    bands().iter().map(|b| band_value(b, t)).sum()
}
fn noise(t: f64) -> f64 {
    0.10 * (t * 7.31).sin() * (t * 2.73).cos() + 0.06 * (t * 13.11).sin()
}
fn round2(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}
fn round3(v: f64) -> f64 {
    (v * 1000.0).round() / 1000.0
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Regime {
    pub name: String,
    pub description: String,
    pub tone: String,
    pub multiplier: f64,
}

fn classify_regime(current: f64, direction: f64) -> Regime {
    if current > 1.4 {
        Regime {
            name: "CONSTRUCTIVE INTERFERENCE".into(),
            description: "Multiple bands cresting in phase.".into(),
            tone: "#d97706".into(),
            multiplier: 1.10,
        }
    } else if current < -1.4 {
        Regime {
            name: "DESTRUCTIVE INTERFERENCE".into(),
            description: "Bands collapsed into trough alignment.".into(),
            tone: "#7f1d1d".into(),
            multiplier: 0.30,
        }
    } else if direction > 0.05 {
        Regime {
            name: "RISING WATERS".into(),
            description: "Net ascending.".into(),
            tone: "#a16207".into(),
            multiplier: 1.00,
        }
    } else if direction < -0.05 {
        Regime {
            name: "EBBING TIDE".into(),
            description: "Net descending.".into(),
            tone: "#7c2d12".into(),
            multiplier: 0.60,
        }
    } else {
        Regime {
            name: "CROSSWINDS".into(),
            description: "Bands in opposition.".into(),
            tone: "#5a6a7a".into(),
            multiplier: 0.80,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BandReading {
    pub band: Band,
    pub value: f64,
    pub phase_deg: f64,
    pub state: String,
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Reading {
    pub t: f64,
    pub year: f64,
    pub as_of: DateTime<Utc>,
    pub composite: f64,
    pub composite_ahead: f64,
    pub direction: f64,
    pub bands: Vec<BandReading>,
    pub regime: Regime,
}

pub fn reading_at(at: DateTime<Utc>) -> Reading {
    let t = time_to_t(at);
    let current = composite(t);
    let ahead = composite(t + 0.5);
    let dir = ahead - current;
    let br: Vec<BandReading> = bands()
        .iter()
        .map(|b| {
            let deg = band_phase_deg(b, t);
            let (state, desc) = describe_phase(deg);
            BandReading {
                band: b.clone(),
                value: band_value(b, t),
                phase_deg: deg,
                state: state.into(),
                description: desc.into(),
            }
        })
        .collect();
    Reading {
        t,
        year: EPOCH + t,
        as_of: at,
        composite: current,
        composite_ahead: ahead,
        direction: dir,
        bands: br,
        regime: classify_regime(current, dir),
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SeriesPoint {
    pub year: f64,
    pub composite: f64,
    pub kondratiev: f64,
    pub kuznets: f64,
    pub juglar: f64,
    pub kitchin: f64,
}

pub fn series(start_year: f64, end_year: f64, step: f64) -> Vec<SeriesPoint> {
    let step = if step <= 0.0 { 0.25 } else { step };
    let mut out = Vec::new();
    let mut y = start_year;
    while y <= end_year + 1e-9 {
        let t = y - EPOCH;
        let bs = bands();
        out.push(SeriesPoint {
            year: round2(y),
            composite: round3(composite(t) + noise(t)),
            kondratiev: round3(band_value(&bs[0], t)),
            kuznets: round3(band_value(&bs[1], t)),
            juglar: round3(band_value(&bs[2], t)),
            kitchin: round3(band_value(&bs[3], t)),
        });
        y += step;
    }
    out
}
