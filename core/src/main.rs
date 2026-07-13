use std::io::{self, BufRead};
use serde::{Deserialize, Serialize};

#[derive(Deserialize)]
struct Request {
    cmd: String,
}

#[derive(Serialize)]
struct SuccessResponse {
    status: String,
    version: String,
    message: String,
}

#[derive(Serialize)]
struct ErrorResponse {
    status: String,
    message: String,
}

fn handle_request(input: &str) -> Result<String, String> {
    let req: Request = serde_json::from_str(input)
        .map_err(|e| format!("Failed to parse JSON request: {}", e))?;

    if req.cmd == "ping" {
        let resp = SuccessResponse {
            status: "ok".to_string(),
            version: env!("CARGO_PKG_VERSION").to_string(),
            message: "pong".to_string(),
        };
        serde_json::to_string(&resp)
            .map_err(|e| format!("Failed to serialize success response: {}", e))
    } else {
        let resp = ErrorResponse {
            status: "error".to_string(),
            message: format!("unknown command: {}", req.cmd),
        };
        let serialized = serde_json::to_string(&resp)
            .map_err(|e| format!("Failed to serialize error response: {}", e))?;
        Err(serialized)
    }
}

fn main() {
    let stdin = io::stdin();
    let mut iterator = stdin.lock().lines();

    if let Some(Ok(line)) = iterator.next() {
        match handle_request(&line) {
            Ok(success) => {
                println!("{}", success);
                std::process::exit(0);
            }
            Err(error_json) => {
                if error_json.contains(r#""status":"error""#) {
                    println!("{}", error_json);
                } else {
                    let resp = ErrorResponse {
                        status: "error".to_string(),
                        message: error_json,
                    };
                    if let Ok(serialized) = serde_json::to_string(&resp) {
                        println!("{}", serialized);
                    } else {
                        println!(r#"{{"status":"error","message":"failed to serialize error"}}"#);
                    }
                }
                std::process::exit(1);
            }
        }
    } else {
        let resp = ErrorResponse {
            status: "error".to_string(),
            message: "no input received on stdin".to_string(),
        };
        if let Ok(serialized) = serde_json::to_string(&resp) {
            println!("{}", serialized);
        }
        std::process::exit(1);
    }
}
