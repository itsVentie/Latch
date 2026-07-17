use std::error::Error;
use tokio::net::TcpListener;
use std::net::SocketAddr;
use tokio::io::AsyncReadExt;

mod ipc;
mod tcp_engine;
mod udp_engine;

use ipc::SessionContext;
use udp_engine::UdpSessionManager;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let listener = TcpListener::bind("127.0.0.1:49151").await?;
    println!("Latch Dataplane listening on 127.0.0.1:49151");

    loop {
        match listener.accept().await {
            Ok((mut stream, _addr)) => {
                tokio::spawn(async move {
                    let mut buf = vec![0u8; 4096];
                    if let Ok(n) = stream.read(&mut buf).await {
                        if n == 0 { return; }
                        
                        if let Ok(ctx) = serde_json::from_slice::<SessionContext>(&buf[..n]) {
                            let remote_addr: SocketAddr = match ctx.remote_addr.parse() {
                                Ok(addr) => addr,
                                Err(_) => return,
                            };

                            let key_bytes = match hex::decode(&ctx.crypto_key) {
                                Ok(bytes) => bytes,
                                Err(_) => return,
                            };

                            if ctx.protocol.to_uppercase() == "UDP" {
                                let manager = UdpSessionManager::new(&key_bytes, remote_addr);
                                let listen_addr: SocketAddr = "0.0.0.0:3000".parse().unwrap();
                                let _ = manager.start_local_listener(listen_addr).await;
                            }
                        }
                    }
                });
            }
            Err(e) => ephemeral_log_error(e),
        }
    }
}

fn ephemeral_log_error(e: impl std::fmt::Display) {
    eprintln!("Accept error: {}", e);
}