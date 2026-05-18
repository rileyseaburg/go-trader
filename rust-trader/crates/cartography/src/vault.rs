//! Vault KV-v2 loader — reads secrets from HashiCorp Vault.

use serde_json::Value;

#[derive(Clone)]
pub struct VaultLoader {
    addr: String,
    token: String,
    http: reqwest::Client,
}

impl VaultLoader {
    pub fn from_env() -> Option<Self> {
        let addr = std::env::var("VAULT_ADDR").ok()?;
        let token = std::env::var("VAULT_TOKEN").ok()?;
        Some(Self {
            addr: addr.trim_end_matches('/').into(),
            token,
            http: reqwest::Client::new(),
        })
    }

    pub async fn field(
        &self,
        path: &str,
        field: &str,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let (mount, key) = split_mount_path(path);
        let url = format!("{}/v1/{}/data/{}", self.addr, mount, key);
        let resp = self
            .http
            .get(&url)
            .header("X-Vault-Token", &self.token)
            .send()
            .await?;
        if resp.status().as_u16() == 404 {
            return Err(format!("vault: not found at {}", path).into());
        }
        if resp.status().as_u16() == 403 {
            return Err(format!("vault: forbidden at {}", path).into());
        }
        let body: Value = resp.error_for_status()?.json().await?;
        let val = body
            .get("data")
            .and_then(|d| d.get("data"))
            .and_then(|d| d.get(field));
        match val {
            Some(Value::String(s)) => Ok(s.trim().to_string()),
            Some(other) => Ok(other.to_string()),
            None => Err(format!("vault: field {} not found at {}", field, path).into()),
        }
    }
}

fn split_mount_path(p: &str) -> (String, String) {
    let p = p.trim_matches('/');
    if let Some(idx) = p.find('/') {
        (p[..idx].into(), p[idx + 1..].into())
    } else {
        (String::new(), String::new())
    }
}
