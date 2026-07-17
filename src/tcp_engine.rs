use std::error::Error;
use tokio::net::TcpStream;

pub async fn handle_tcp_proxy(mut client_stream: TcpStream, mut remote_stream: TcpStream) -> Result<(), Box<dyn Error>> {
    let (mut client_reader, mut client_writer) = client_stream.split();
    let (mut remote_reader, mut remote_writer) = remote_stream.split();

    let client_to_remote = tokio::io::copy(&mut client_reader, &mut remote_writer);
    let remote_to_client = tokio::io::copy(&mut remote_reader, &mut client_writer);

    tokio::select! {
        res = client_to_remote => { res?; },
        res = remote_to_client => { res?; },
    };

    Ok(())
}