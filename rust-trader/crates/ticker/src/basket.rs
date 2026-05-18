//! Basket management — persistent JSON-file storage for symbol groups.

use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::{Arc, RwLock};

use chrono::Utc;
use serde::{Deserialize, Serialize};
use tokio::fs;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TickerBasket {
    pub id: String,
    pub name: String,
    pub description: String,
    pub symbols: Vec<String>,
    pub created_at: String,
    pub updated_at: String,
    #[serde(default)]
    pub created_by: Option<String>,
    #[serde(default)]
    pub tags: Vec<String>,
    pub is_active: bool,
    #[serde(default)]
    pub category: Option<String>,
}

pub struct BasketManager {
    data_dir: PathBuf,
    baskets: Arc<RwLock<HashMap<String, TickerBasket>>>,
}

impl BasketManager {
    pub async fn new(
        data_dir: impl AsRef<Path>,
    ) -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        let data_dir = data_dir.as_ref().to_path_buf();
        fs::create_dir_all(data_dir.join("baskets")).await?;
        let mut mgr = Self {
            data_dir,
            baskets: Arc::new(RwLock::new(HashMap::new())),
        };
        mgr.load_baskets().await?;
        Ok(mgr)
    }

    async fn load_baskets(&mut self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let basket_dir = self.data_dir.join("baskets");
        let _ = fs::create_dir_all(&basket_dir).await;
        let mut entries = fs::read_dir(&basket_dir).await?;
        while let Some(entry) = entries.next_entry().await? {
            let path = entry.path();
            if path.extension().map(|e| e == "json").unwrap_or(false) {
                let data = fs::read_to_string(&path).await?;
                if let Ok(basket) = serde_json::from_str::<TickerBasket>(&data) {
                    if !basket.id.is_empty() {
                        self.baskets
                            .write()
                            .unwrap()
                            .insert(basket.id.clone(), basket);
                    }
                }
            }
        }
        Ok(())
    }

    pub async fn save_basket(
        &self,
        mut basket: TickerBasket,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        if basket.id.is_empty() {
            basket.id = format!("basket_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0));
        }
        if basket.created_at.is_empty() {
            basket.created_at = Utc::now().to_rfc3339();
        }
        basket.updated_at = Utc::now().to_rfc3339();
        let dir = self.data_dir.join("baskets");
        fs::create_dir_all(&dir).await?;
        let path = dir.join(format!("{}.json", basket.id));
        fs::write(path, serde_json::to_string_pretty(&basket)?).await?;
        self.baskets
            .write()
            .unwrap()
            .insert(basket.id.clone(), basket);
        Ok(())
    }

    pub fn get_basket(&self, id: &str) -> Option<TickerBasket> {
        self.baskets.read().unwrap().get(id).cloned()
    }

    pub async fn delete_basket(
        &self,
        id: &str,
    ) -> Result<bool, Box<dyn std::error::Error + Send + Sync>> {
        let existed = self.baskets.write().unwrap().remove(id).is_some();
        if existed {
            let _ =
                fs::remove_file(self.data_dir.join("baskets").join(format!("{}.json", id))).await;
        }
        Ok(existed)
    }

    pub fn list_baskets(&self) -> Vec<TickerBasket> {
        self.baskets.read().unwrap().values().cloned().collect()
    }

    pub fn get_symbols_from_baskets(&self, ids: &[String]) -> Vec<String> {
        let guard = self.baskets.read().unwrap();
        let mut set = std::collections::HashSet::new();
        for id in ids {
            if let Some(b) = guard.get(id) {
                for s in &b.symbols {
                    set.insert(s.clone());
                }
            }
        }
        set.into_iter().collect()
    }
}
