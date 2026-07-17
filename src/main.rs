use std::error::Error;
use std::net::SocketAddr;
use tokio::net::{TcpListener, TcpStream};
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

                            let proto = ctx.protocol.to_uppercase();

                            if proto == "UDP" {
                                let listen_addr: SocketAddr = match ctx.listen_addr.parse() {
                                    Ok(addr) => addr,
                                    Err(_) => return,
                                };

                                tokio::spawn(async move {
                                    let manager = UdpSessionManager::new(&key_bytes, remote_addr);
                                    let _ = manager.start_local_listener(listen_addr).await;
                                });
                            } else if proto == "TCP" {
                                let listen_addr: SocketAddr = match ctx.listen_addr.parse() {
                                    Ok(addr) => addr,
                                    Err(_) => return,
                                };

                                tokio::spawn(async move {
                                    if let Ok(tcp_listener) = TcpListener::bind(listen_addr).await {
                                        while let Ok((client_stream, _)) = tcp_listener.accept().await {
                                            if let Ok(remote_stream) = TcpStream::connect(remote_addr).await {
                                                tokio::spawn(async move {
                                                    let _ = tcp_engine::handle_tcp_proxy(client_stream, remote_stream).await;
                                                });
                                            }
                                        }
                                    }
                                });
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