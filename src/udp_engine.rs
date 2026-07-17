use std::collections::HashMap;
use std::error::Error;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::net::UdpSocket;
use tokio::sync::RwLock;
use tokio::time::{self, Duration};
use chacha20poly1305::ChaCha20Poly1305;
use chacha20poly1305::aead::{Aead, KeyInit, Payload};

pub struct UdpSessionManager {
    sessions: Arc<RwLock<HashMap<SocketAddr, Arc<UdpSocket>>>>,
    cipher: Arc<ChaCha20Poly1305>,
    remote_target: SocketAddr,
}

impl UdpSessionManager {
    pub fn new(key_bytes: &[u8], remote_target: SocketAddr) -> Self {
        let key = chacha20poly1305::Key::from_slice(key_bytes);
        let cipher = Arc::new(ChaCha20Poly1305::new(key));
        Self {
            sessions: Arc::new(RwLock::new(HashMap::new())),
            cipher,
            remote_target,
        }
    }

    pub async fn start_local_listener(&self, listen_addr: SocketAddr) -> Result<(), Box<dyn Error>> {
        let local_socket = Arc::new(UdpSocket::bind(listen_addr).await?);
        let mut buf = vec![0u8; 65535];

        loop {
            let (len, src_addr) = local_socket.recv_from(&mut buf).await?;
            let sessions_guard = self.sessions.read().await;

            let tunnel_socket = match sessions_guard.get(&src_addr) {
                Some(sock) => sock.clone(),
                None => {
                    drop(sessions_guard);
                    let mut write_guard = self.sessions.write().await;
                    
                    let sock = match write_guard.get(&src_addr) {
                        Some(s) => s.clone(),
                        None => {
                            let s = Arc::new(UdpSocket::bind("0.0.0.0:0").await?);
                            write_guard.insert(src_addr, s.clone());
                            
                            let sessions_clone = self.sessions.clone();
                            let local_socket_clone = local_socket.clone();
                            let cipher_clone = self.cipher.clone();
                            let s_task = s.clone();
                            
                            tokio::spawn(async move {
                                let mut outbound_buf = vec![0u8; 65535];
                                loop {
                                    tokio::select! {
                                        res = s_task.recv_from(&mut outbound_buf) => {
                                            if let Ok((out_len, _)) = res {
                                                let nonce = chacha20poly1305::Nonce::from_slice(&[0u8; 12]);
                                                let payload = Payload {
                                                    msg: &outbound_buf[..out_len],
                                                    aad: &[],
                                                };
                                                if let Ok(decrypted) = cipher_clone.decrypt(nonce, payload) {
                                                    let _ = local_socket_clone.send_to(&decrypted, src_addr).await;
                                                }
                                            }
                                        }
                                        _ = time::sleep(Duration::from_secs(60)) => {
                                            let mut guard = sessions_clone.write().await;
                                            guard.remove(&src_addr);
                                            break;
                                        }
                                    }
                                }
                            });
                            s
                        }
                    };
                    sock
                }
            };

            let nonce = chacha20poly1305::Nonce::from_slice(&[0u8; 12]);
            let payload = Payload {
                msg: &buf[..len],
                aad: &[],
            };
            if let Ok(encrypted) = self.cipher.encrypt(nonce, payload) {
                let _ = tunnel_socket.send_to(&encrypted, self.remote_target).await;
            }
        }
    }
}