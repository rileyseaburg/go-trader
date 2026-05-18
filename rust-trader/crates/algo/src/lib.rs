//! go-trader-algo: Mathematical trading algorithm implementations.
//!
//! Provides López de Prado-style algorithms: triple barrier labeling,
//! meta-labeling, fractional differentiation, purged cross-validation,
//! sequential bootstrap, CUSUM filter, HRP, MVO, entropy pooling,
//! and position sizing.

use serde::{Deserialize, Serialize};

/// Result of running an algorithm.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlgorithmResult {
    pub algorithm: String,
    pub symbol: String,
    pub success: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signal: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub confidence: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub explanation: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
}

/// Metadata about a registered algorithm.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlgorithmMetadata {
    pub name: String,
    pub description: String,
    pub parameters: Vec<ParameterDesc>,
}

/// Description of an algorithm parameter.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParameterDesc {
    pub name: String,
    pub description: String,
    pub param_type: String,
    pub default_value: Option<serde_json::Value>,
}

/// Returns the list of available algorithms and their parameter schemas.
pub fn get_registered_algorithms() -> Vec<AlgorithmMetadata> {
    vec![
        AlgorithmMetadata {
            name: "triple_barrier".into(),
            description:
                "Triple-barrier labeling method from Advances in Financial Machine Learning".into(),
            parameters: vec![
                ParameterDesc {
                    name: "pt_sl".into(),
                    description: "Profit-take / stop-loss ratio".into(),
                    param_type: "float".into(),
                    default_value: Some(serde_json::json!(1.0)),
                },
                ParameterDesc {
                    name: "num_days".into(),
                    description: "Maximum holding period in bars".into(),
                    param_type: "int".into(),
                    default_value: Some(serde_json::json!(10)),
                },
            ],
        },
        AlgorithmMetadata {
            name: "meta_labeling".into(),
            description: "Meta-labeling for secondary model confidence".into(),
            parameters: vec![],
        },
        AlgorithmMetadata {
            name: "fractional_diff".into(),
            description: "Fractional differentiation for stationary series".into(),
            parameters: vec![ParameterDesc {
                name: "d".into(),
                description: "Differencing order (0..1)".into(),
                param_type: "float".into(),
                default_value: Some(serde_json::json!(0.4)),
            }],
        },
        AlgorithmMetadata {
            name: "hrp".into(),
            description: "Hierarchical Risk Parity portfolio construction".into(),
            parameters: vec![],
        },
        AlgorithmMetadata {
            name: "cusum_filter".into(),
            description: "CUSUM filter for event detection".into(),
            parameters: vec![ParameterDesc {
                name: "threshold".into(),
                description: "Filter threshold".into(),
                param_type: "float".into(),
                default_value: Some(serde_json::json!(1.0)),
            }],
        },
    ]
}
