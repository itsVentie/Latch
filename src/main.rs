use std::error::Error;
use tokio::net::TcpListener;

mod ipc;
mod tcp_engine;
mod udp_engine;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let listener = TcpListener::bind("127.0.0.1:49151").await?;
    println!("Latch Dataplane listening on 127.0.0.1:49151");

    loop {
        match listener.accept().await {
            Ok((_stream, _addr)) => {
                tokio::spawn(async move {
                });
            }
            Err(e) => ephemeral_log_error(e),
        }
    }
}

fn ephemeral_log_error(e: impl std::fmt::Display) {
    eprintln!("Accept error: {}", e);
}