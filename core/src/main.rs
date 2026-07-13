mod build;
mod config;
mod protocol;
mod qemu;

use protocol::{
    BuildResultData, ErrorResponse, ListProfilesData, ProfileSummary, QemuLaunchData,
    SuccessResponse,
};
use std::io::{self, BufRead};
use std::path::PathBuf;

// ---------------------------------------------------------------------------
// Command dispatch
// ---------------------------------------------------------------------------

fn handle_request(input: &str) -> Result<String, String> {
    let req: protocol::Request =
        serde_json::from_str(input).map_err(|e| format!("Failed to parse JSON request: {}", e))?;

    match req.cmd.as_str() {
        "ping" => handle_ping(),
        "build" => handle_build(&req),
        "list-profiles" => handle_list_profiles(&req),
        "launch" => handle_launch(&req),
        "stop" => handle_stop(&req),
        "qemu-status" => handle_qemu_status(&req),
        other => {
            let resp = ErrorResponse::new(format!("unknown command: {}", other));
            let serialized = serde_json::to_string(&resp)
                .map_err(|e| format!("Failed to serialize error response: {}", e))?;
            Err(serialized)
        }
    }
}

// ---------------------------------------------------------------------------
// ping
// ---------------------------------------------------------------------------

fn handle_ping() -> Result<String, String> {
    let resp = SuccessResponse::ok_with_version("pong", env!("CARGO_PKG_VERSION"));
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// build
// ---------------------------------------------------------------------------

fn handle_build(req: &protocol::Request) -> Result<String, String> {
    let profile_name = req
        .profile
        .as_deref()
        .ok_or("Missing required field 'profile'")?;

    let project_root = req
        .project_root
        .as_deref()
        .ok_or("Missing required field 'project_root'")?;

    let project_root = PathBuf::from(project_root);
    if !project_root.exists() {
        return Err(format!(
            "project_root '{}' does not exist",
            project_root.display()
        ));
    }

    let project_config = config::load_config(&project_root)?;
    let results = build::execute_build(&project_config, profile_name, &project_root)?;

    // Report the last result (the primary build target).
    let last = results.last().ok_or("No build results")?;

    if last.success() {
        let data = BuildResultData {
            profile: last.profile_name.clone(),
            tool: last.tool.clone(),
            exit_code: last.exit_code,
            stdout: last.stdout.clone(),
            stderr: last.stderr.clone(),
        };
        let resp = SuccessResponse::ok_with_data(
            format!("Build '{}' succeeded", profile_name),
            serde_json::to_value(&data)
                .map_err(|e| format!("Failed to serialize build data: {}", e))?,
        );
        serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
    } else {
        let msg = format!(
            "Build '{}' failed (exit code {})\n--- stdout ---\n{}\n--- stderr ---\n{}",
            profile_name, last.exit_code, last.stdout, last.stderr
        );
        let resp = ErrorResponse::new(msg);
        let serialized = serde_json::to_string(&resp)
            .map_err(|e| format!("Failed to serialize error response: {}", e))?;
        Err(serialized)
    }
}

// ---------------------------------------------------------------------------
// list-profiles
// ---------------------------------------------------------------------------

fn handle_list_profiles(req: &protocol::Request) -> Result<String, String> {
    let project_root = req
        .project_root
        .as_deref()
        .ok_or("Missing required field 'project_root'")?;

    let project_root = PathBuf::from(project_root);
    if !project_root.exists() {
        return Err(format!(
            "project_root '{}' does not exist",
            project_root.display()
        ));
    }

    let project_config = config::load_config(&project_root)?;
    let profiles = build::list_profiles(&project_config);

    let summaries: Vec<ProfileSummary> = profiles
        .into_iter()
        .map(|(name, profile)| ProfileSummary {
            name,
            tool: profile.tool.clone(),
            description: profile.description.clone(),
        })
        .collect();

    let data = ListProfilesData {
        profiles: summaries,
    };

    let resp = SuccessResponse::ok_with_data(
        "Profiles loaded",
        serde_json::to_value(&data)
            .map_err(|e| format!("Failed to serialize profiles data: {}", e))?,
    );
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// launch
// ---------------------------------------------------------------------------

fn handle_launch(req: &protocol::Request) -> Result<String, String> {
    let project_root = req
        .project_root
        .as_deref()
        .ok_or("Missing required field 'project_root'")?;

    let debug_mode = req.debug.unwrap_or(true);

    let project_root = PathBuf::from(project_root);
    if !project_root.exists() {
        return Err(format!(
            "project_root '{}' does not exist",
            project_root.display()
        ));
    }

    let project_config = config::load_config(&project_root)?;
    let qemu_config = project_config
        .qemu
        .as_ref()
        .ok_or("No [qemu] configuration found in pyxforge.toml. Please add a [qemu] section.")?;

    let (pid, args_used) = qemu::launch_qemu(qemu_config, &project_root, debug_mode)?;

    let port = if debug_mode && qemu_config.debug.enabled {
        qemu_config.debug.gdb_port
    } else {
        0
    };

    let data = QemuLaunchData {
        pid,
        port,
        args_used,
    };

    let resp = SuccessResponse::ok_with_data(
        "QEMU launched successfully",
        serde_json::to_value(&data)
            .map_err(|e| format!("Failed to serialize QEMU launch data: {}", e))?,
    );
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// stop
// ---------------------------------------------------------------------------

fn handle_stop(req: &protocol::Request) -> Result<String, String> {
    let pid = req.pid.ok_or("Missing required field 'pid'")?;

    qemu::stop_qemu(pid)?;

    let resp = SuccessResponse::ok(format!("QEMU process with PID {} stopped", pid));
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// qemu-status
// ---------------------------------------------------------------------------

fn handle_qemu_status(req: &protocol::Request) -> Result<String, String> {
    let pid = req.pid.ok_or("Missing required field 'pid'")?;
    let alive = qemu::is_qemu_running(pid);

    #[derive(serde::Serialize)]
    struct StatusData {
        alive: bool,
    }

    let resp = SuccessResponse::ok_with_data(
        "Status checked",
        serde_json::to_value(&StatusData { alive })
            .map_err(|e| format!("Failed to serialize status data: {}", e))?,
    );
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// Entrypoint
// ---------------------------------------------------------------------------

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
                    let resp = ErrorResponse::new(error_json);
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
        let resp = ErrorResponse::new("no input received on stdin");
        if let Ok(serialized) = serde_json::to_string(&resp) {
            println!("{}", serialized);
        }
        std::process::exit(1);
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

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

    #[test]
    fn test_handle_build_missing_profile() {
        let input = r#"{"cmd":"build","project_root":"."}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("profile"));
    }

    #[test]
    fn test_handle_build_missing_project_root() {
        let input = r#"{"cmd":"build","profile":"bootloader"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("project_root"));
    }

    #[test]
    fn test_handle_list_profiles_missing_project_root() {
        let input = r#"{"cmd":"list-profiles"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("project_root"));
    }

    #[test]
    fn test_handle_launch_missing_project_root() {
        let input = r#"{"cmd":"launch"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("project_root"));
    }

    #[test]
    fn test_handle_stop_missing_pid() {
        let input = r#"{"cmd":"stop"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("pid"));
    }

    #[test]
    fn test_handle_qemu_status_missing_pid() {
        let input = r#"{"cmd":"qemu-status"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("pid"));
    }
}
