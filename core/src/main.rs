use serde::{Deserialize, Serialize};
use std::io::{self, BufRead};

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
    let req: Request =
        serde_json::from_str(input).map_err(|e| format!("Failed to parse JSON request: {}", e))?;

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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_handle_ping() {
        let input = r#"{"cmd":"ping"}"#;
        let res = handle_request(input);
        assert!(res.is_ok());
        let output = res.unwrap();
        assert!(output.contains(r#""status":"ok""#));
        assert!(output.contains(r#""message":"pong""#));
        assert!(output.contains(r#""version":"0.1.0""#));
    }

    #[test]
    fn test_handle_invalid_cmd() {
        let input = r#"{"cmd":"invalid"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains(r#""status":"error""#));
        assert!(output.contains("unknown command: invalid"));
    }

    #[test]
    fn test_handle_bad_json() {
        let input = r#"{"invalid_json"#;
        let res = handle_request(input);
        assert!(res.is_err());
    }
}
