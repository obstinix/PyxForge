use crate::qemu::QmpAddress;
use serde::Serialize;
use std::io::{BufRead, BufReader, Write};
use std::net::TcpStream;
use std::time::Duration;

#[cfg(unix)]
use std::os::unix::net::UnixStream;

#[derive(Debug)]
pub enum QmpStream {
    Tcp(TcpStream),
    #[cfg(unix)]
    Unix(UnixStream),
}

impl QmpStream {
    pub fn connect(addr: &QmpAddress, timeout: Duration) -> Result<Self, String> {
        match addr {
            QmpAddress::Tcp(tcp_addr) => {
                let stream = TcpStream::connect_timeout(
                    &tcp_addr
                        .parse()
                        .map_err(|e| format!("Invalid TCP address: {}", e))?,
                    timeout,
                )
                .map_err(|e| format!("Failed to connect via TCP to {}: {}", tcp_addr, e))?;
                stream.set_read_timeout(Some(timeout)).ok();
                stream.set_write_timeout(Some(timeout)).ok();
                Ok(QmpStream::Tcp(stream))
            }
            #[cfg(unix)]
            QmpAddress::Unix(path) => {
                let stream = UnixStream::connect(path).map_err(|e| {
                    format!(
                        "Failed to connect via Unix socket to {}: {}",
                        path.display(),
                        e
                    )
                })?;
                stream.set_read_timeout(Some(timeout)).ok();
                stream.set_write_timeout(Some(timeout)).ok();
                Ok(QmpStream::Unix(stream))
            }
            #[cfg(not(unix))]
            _ => Err("Unix sockets are not supported on this platform".to_string()),
        }
    }
}

impl std::io::Read for QmpStream {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        match self {
            QmpStream::Tcp(s) => s.read(buf),
            #[cfg(unix)]
            QmpStream::Unix(s) => s.read(buf),
        }
    }
}

impl std::io::Write for QmpStream {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        match self {
            QmpStream::Tcp(s) => s.write(buf),
            #[cfg(unix)]
            QmpStream::Unix(s) => s.write(buf),
        }
    }
    fn flush(&mut self) -> std::io::Result<()> {
        match self {
            QmpStream::Tcp(s) => s.flush(),
            #[cfg(unix)]
            QmpStream::Unix(s) => s.flush(),
        }
    }
}

pub struct QmpClient {
    reader: BufReader<QmpStream>,
}

#[derive(Serialize)]
struct QmpCommand {
    execute: String,
}

impl QmpClient {
    pub fn connect(addr: &QmpAddress) -> Result<Self, String> {
        let timeout = Duration::from_secs(2);
        let stream = QmpStream::connect(addr, timeout)?;
        let mut client = QmpClient {
            reader: BufReader::new(stream),
        };

        // 1. Read greeting message
        let mut greeting_line = String::new();
        client
            .reader
            .read_line(&mut greeting_line)
            .map_err(|e| format!("Failed to read QMP greeting: {}", e))?;

        // 2. Perform capabilities negotiation
        let cmd = QmpCommand {
            execute: "qmp_capabilities".to_string(),
        };
        client.send_command(&cmd)?;

        // 3. Read capabilities negotiation response
        let resp = client.read_response()?;
        if resp.get("error").is_some() {
            return Err(format!("QMP capabilities negotiation failed: {:?}", resp));
        }

        Ok(client)
    }

    #[allow(dead_code)]
    pub fn query_status(&mut self) -> Result<String, String> {
        let cmd = QmpCommand {
            execute: "query-status".to_string(),
        };
        self.send_command(&cmd)?;
        let resp = self.read_response()?;
        if let Some(err) = resp.get("error") {
            return Err(format!("query-status failed: {:?}", err));
        }

        resp.get("return")
            .and_then(|ret| ret.get("status"))
            .and_then(|status| status.as_str())
            .map(|s| s.to_string())
            .ok_or_else(|| "Invalid query-status response format".to_string())
    }

    pub fn graceful_shutdown(&mut self) -> Result<(), String> {
        let cmd = QmpCommand {
            execute: "system_powerdown".to_string(),
        };
        self.send_command(&cmd)?;
        let resp = self.read_response()?;
        if let Some(err) = resp.get("error") {
            return Err(format!("system_powerdown failed: {:?}", err));
        }
        Ok(())
    }

    fn send_command<T: Serialize>(&mut self, cmd: &T) -> Result<(), String> {
        let mut json = serde_json::to_string(cmd)
            .map_err(|e| format!("Failed to serialize QMP command: {}", e))?;
        json.push('\n');

        let inner = self.reader.get_mut();
        inner
            .write_all(json.as_bytes())
            .map_err(|e| format!("Failed to write QMP command: {}", e))?;
        inner
            .flush()
            .map_err(|e| format!("Failed to flush QMP command: {}", e))?;
        Ok(())
    }

    fn read_response(&mut self) -> Result<serde_json::Value, String> {
        let mut line = String::new();
        loop {
            line.clear();
            self.reader
                .read_line(&mut line)
                .map_err(|e| format!("Failed to read line from QMP: {}", e))?;

            if line.trim().is_empty() {
                return Err("Connection closed by remote QEMU instance".to_string());
            }

            let val: serde_json::Value = serde_json::from_str(&line)
                .map_err(|e| format!("Failed to parse QMP JSON line '{}': {}", line, e))?;

            if val.get("event").is_some() {
                // Ignore asynchronous QMP events, keep reading for the response
                continue;
            }

            if val.get("return").is_some() || val.get("error").is_some() {
                return Ok(val);
            }
        }
    }
}
