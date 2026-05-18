//! go-trader-notification: Notification management for the trading platform.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use uuid::Uuid;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum NotificationPriority {
    Low,
    Medium,
    High,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub enum NotificationType {
    SignalGenerated,
    OrderExecuted,
    MarketEvent,
    SystemAlert,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Notification {
    pub id: String,
    #[serde(rename = "type")]
    pub notification_type: NotificationType,
    pub title: String,
    pub message: String,
    pub priority: NotificationPriority,
    pub timestamp: DateTime<Utc>,
    pub read: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, serde_json::Value>>,
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

pub struct NotificationManager {
    notifications: Arc<RwLock<Vec<Notification>>>,
    max_notifications: usize,
}

impl NotificationManager {
    pub fn new(max_notifications: usize) -> Self {
        Self {
            notifications: Arc::new(RwLock::new(Vec::new())),
            max_notifications,
        }
    }

    pub fn add_notification(&self, notification: Notification) {
        let mut guard = self.notifications.write().unwrap();
        guard.push(notification);
        // Trim oldest if over capacity
        while guard.len() > self.max_notifications {
            guard.remove(0);
        }
    }

    pub fn get_notifications(&self) -> Vec<Notification> {
        self.notifications.read().unwrap().clone()
    }

    pub fn get_unread(&self) -> Vec<Notification> {
        self.notifications
            .read()
            .unwrap()
            .iter()
            .filter(|n| !n.read)
            .cloned()
            .collect()
    }

    pub fn get_by_type(&self, notification_type: &NotificationType) -> Vec<Notification> {
        self.notifications
            .read()
            .unwrap()
            .iter()
            .filter(|n| &n.notification_type == notification_type)
            .cloned()
            .collect()
    }

    pub fn get_by_symbol(&self, symbol: &str) -> Vec<Notification> {
        let lower = symbol.to_lowercase();
        self.notifications
            .read()
            .unwrap()
            .iter()
            .filter(|n| {
                n.title.to_lowercase().contains(&lower)
                    || n.message.to_lowercase().contains(&lower)
                    || n.metadata
                        .as_ref()
                        .map(|m| {
                            m.values().any(|v| {
                                v.as_str()
                                    .map(|s| s.to_lowercase().contains(&lower))
                                    .unwrap_or(false)
                            })
                        })
                        .unwrap_or(false)
            })
            .cloned()
            .collect()
    }

    pub fn delete_notification(&self, id: &str) -> bool {
        let mut guard = self.notifications.write().unwrap();
        let before = guard.len();
        guard.retain(|n| n.id != id);
        guard.len() < before
    }

    pub fn mark_as_read(&self, id: &str) -> bool {
        let mut guard = self.notifications.write().unwrap();
        for n in guard.iter_mut() {
            if n.id == id {
                n.read = true;
                return true;
            }
        }
        false
    }

    pub fn mark_all_as_read(&self) {
        let mut guard = self.notifications.write().unwrap();
        for n in guard.iter_mut() {
            n.read = true;
        }
    }

    pub fn clear(&self) {
        self.notifications.write().unwrap().clear();
    }
}

// ---------------------------------------------------------------------------
// Factory functions
// ---------------------------------------------------------------------------

fn generate_id() -> String {
    format!("{}", Uuid::new_v4())
}

pub fn create_signal_generated_notification(
    symbol: &str,
    signal: &str,
    reasoning: &str,
    priority: NotificationPriority,
    metadata: Option<HashMap<String, serde_json::Value>>,
) -> Notification {
    Notification {
        id: generate_id(),
        notification_type: NotificationType::SignalGenerated,
        title: format!("Signal: {} {}", signal.to_uppercase(), symbol),
        message: format!(
            "{} signal for {} — {}",
            signal.to_uppercase(),
            symbol,
            reasoning
        ),
        priority,
        timestamp: Utc::now(),
        read: false,
        metadata,
    }
}

pub fn create_order_executed_notification(
    symbol: &str,
    order_type: &str,
    quantity: f64,
    price: f64,
    metadata: Option<HashMap<String, serde_json::Value>>,
) -> Notification {
    Notification {
        id: generate_id(),
        notification_type: NotificationType::OrderExecuted,
        title: format!(
            "Order: {} {} {}",
            order_type.to_uppercase(),
            quantity,
            symbol
        ),
        message: format!(
            "{} order executed for {} shares of {} at ${:.2}",
            order_type.to_uppercase(),
            format_quantity(quantity),
            symbol,
            price
        ),
        priority: NotificationPriority::High,
        timestamp: Utc::now(),
        read: false,
        metadata,
    }
}

pub fn create_market_event_notification(
    symbol: &str,
    event_type: &str,
    message: &str,
) -> Notification {
    Notification {
        id: generate_id(),
        notification_type: NotificationType::MarketEvent,
        title: format!("{}: {}", symbol, event_type),
        message: message.to_string(),
        priority: NotificationPriority::Medium,
        timestamp: Utc::now(),
        read: false,
        metadata: None,
    }
}

pub fn create_system_alert_notification(
    title: &str,
    message: &str,
    metadata: Option<HashMap<String, serde_json::Value>>,
) -> Notification {
    Notification {
        id: generate_id(),
        notification_type: NotificationType::SystemAlert,
        title: title.to_string(),
        message: message.to_string(),
        priority: NotificationPriority::Medium,
        timestamp: Utc::now(),
        read: false,
        metadata,
    }
}

fn format_quantity(q: f64) -> String {
    if q.fract() == 0.0 {
        format!("{}", q as i64)
    } else {
        format!("{:.2}", q)
    }
}
