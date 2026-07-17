use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SessionContext {
    pub session_id: String,
    pub crypto_key: String,
    pub remote_addr: String,
    pub protocol: String,
}