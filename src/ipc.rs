use serde::{Deserialize, Serialize};
use zeroize::{Zeroize, ZeroizeOnDrop};

#[derive(Serialize, Deserialize, Debug, Clone, Zeroize, ZeroizeOnDrop)]
pub struct SessionContext {
    pub session_id: String,
    pub crypto_key: String,
    pub listen_addr: String,
    pub remote_addr: String,
    pub protocol: String,
}