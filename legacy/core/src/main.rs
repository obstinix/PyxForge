mod build;
mod config;
mod diagnostics;
mod gdb;
mod hex;
mod protocol;
mod qemu;
mod qmp;
mod scaffold;

use protocol::{
    BuildResultData, DebugConfigData, ErrorResponse, ListProfilesData, ProfileSummary,
    QemuLaunchData, SuccessResponse,
};
use std::io::{self, BufRead};
use std::path::PathBuf;

// ---------------------------------------------------------------------------
// Command dispatch
// ---------------------------------------------------------------------------

/// Dispatches the incoming JSON-RPC request to the corresponding handler.
///
/// Parses the JSON input string into a strongly-typed `Request` enum variant,
/// and delegates parameters directly to the specific command module logic.
fn handle_request(input: &str) -> Result<String, String> {
    let req: protocol::Request =
        serde_json::from_str(input).map_err(|e| format!("Failed to parse JSON request: {}", e))?;

    match req {
        protocol::Request::Ping => handle_ping(),
        protocol::Request::Build {
            project_root,
            profile,
        } => handle_build(&project_root, &profile),
        protocol::Request::ListProfiles { project_root } => handle_list_profiles(&project_root),
        protocol::Request::Launch {
            project_root,
            debug,
        } => handle_launch(&project_root, debug.unwrap_or(true)),
        protocol::Request::Stop { pid, project_root } => handle_stop(pid, project_root.as_deref()),
        protocol::Request::QemuStatus { pid } => handle_qemu_status(pid),
        protocol::Request::DebugConfig {
            project_root,
            profile,
        } => handle_debug_config(&project_root, profile.as_deref()),
        protocol::Request::Init {
            project_root,
            project_name,
            template,
        } => handle_init(&project_root, &project_name, template.as_deref()),
        protocol::Request::HexDump { file_path } => handle_hex_dump(&file_path),
        protocol::Request::QemuSnapshotSave { project_root, tag } => {
            handle_qemu_monitor_cmd(&project_root, &format!("savevm {}", tag))
        }
        protocol::Request::QemuSnapshotLoad { project_root, tag } => {
            handle_qemu_monitor_cmd(&project_root, &format!("loadvm {}", tag))
        }
        protocol::Request::QemuSnapshotDelete { project_root, tag } => {
            handle_qemu_monitor_cmd(&project_root, &format!("delvm {}", tag))
        }
        protocol::Request::QemuSnapshotList { project_root } => {
            handle_qemu_monitor_cmd(&project_root, "info snapshots")
        }
        protocol::Request::QemuMonitorCommand {
            project_root,
            command,
        } => handle_qemu_monitor_cmd(&project_root, &command),
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

fn handle_build(project_root_str: &str, profile_name: &str) -> Result<String, String> {
    let project_root = PathBuf::from(project_root_str);
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

    // Parse diagnostics from the build output.
    let diags = diagnostics::parse_build_diagnostics(&last.stdout, &last.stderr);

    let message = if last.success() {
        format!("Build '{}' succeeded", profile_name)
    } else {
        format!(
            "Build '{}' failed (exit code {})",
            profile_name, last.exit_code
        )
    };

    // Always return SuccessResponse when the tool ran to completion.
    // The caller determines pass/fail from exit_code in the data.
    let data = BuildResultData {
        profile: last.profile_name.clone(),
        tool: last.tool.clone(),
        exit_code: last.exit_code,
        stdout: last.stdout.clone(),
        stderr: last.stderr.clone(),
        diagnostics: diags,
    };
    let resp = SuccessResponse::ok_with_data(
        message,
        serde_json::to_value(&data)
            .map_err(|e| format!("Failed to serialize build data: {}", e))?,
    );
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// list-profiles
// ---------------------------------------------------------------------------

fn handle_list_profiles(project_root_str: &str) -> Result<String, String> {
    let project_root = PathBuf::from(project_root_str);
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

fn handle_launch(project_root_str: &str, debug_mode: bool) -> Result<String, String> {
    let project_root = PathBuf::from(project_root_str);
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

fn handle_stop(pid: u32, project_root_str: Option<&str>) -> Result<String, String> {
    let mut project_root_path = None;
    let mut qemu_config = None;
    let mut _config = None;

    if let Some(root_str) = project_root_str {
        let root = PathBuf::from(root_str);
        let config_opt = if root.exists() {
            config::load_config(&root).ok().filter(|c| c.qemu.is_some())
        } else {
            None
        };
        if let Some(project_config) = config_opt {
            _config = Some(project_config);
            qemu_config = _config.as_ref().and_then(|c| c.qemu.as_ref());
            project_root_path = Some(root);
        }
    }

    qemu::stop_qemu(pid, project_root_path.as_deref(), qemu_config)?;

    let resp = SuccessResponse::ok(format!("QEMU process with PID {} stopped", pid));
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// qemu-status
// ---------------------------------------------------------------------------

fn handle_qemu_status(pid: u32) -> Result<String, String> {
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
// debug-config
// ---------------------------------------------------------------------------

fn handle_debug_config(
    project_root_str: &str,
    profile_name: Option<&str>,
) -> Result<String, String> {
    let project_root = PathBuf::from(project_root_str);
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
        .ok_or("No [qemu] configuration found in pyxforge.toml. Required for debug-config.")?;

    let mut gdb_config = project_config.gdb.as_ref().cloned().unwrap_or_default();

    if let Some(name) = profile_name {
        let profile = config::get_profile(&project_config, name)?;
        if let Some(profile_gdb) = &profile.gdb {
            if let Some(exec) = &profile_gdb.executable {
                gdb_config.executable = exec.clone();
            }
            if let Some(arch) = &profile_gdb.architecture {
                gdb_config.architecture = arch.clone();
            }
        }
    }

    let launch_config = gdb::build_launch_config(&gdb_config, qemu_config);

    let data = DebugConfigData {
        gdb_executable: launch_config.gdb_executable,
        architecture: launch_config.architecture,
        target: launch_config.target,
        setup_commands: launch_config.setup_commands,
    };

    let resp = SuccessResponse::ok_with_data(
        "Debug configuration generated",
        serde_json::to_value(&data)
            .map_err(|e| format!("Failed to serialize debug config data: {}", e))?,
    );
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

fn handle_init(
    project_root_str: &str,
    project_name: &str,
    template_str: Option<&str>,
) -> Result<String, String> {
    let template = template_str.unwrap_or("assembly");

    let project_root = PathBuf::from(project_root_str);
    scaffold::generate_scaffold(project_name, &project_root, template)?;

    let resp = SuccessResponse::ok(format!(
        "Project '{}' initialized successfully",
        project_name
    ));
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// qemu-monitor-command
// ---------------------------------------------------------------------------

fn handle_qemu_monitor_cmd(
    project_root_str: &str,
    command: &str,
) -> Result<String, String> {
    let project_root = PathBuf::from(project_root_str);
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
        .ok_or("No [qemu] configuration found in pyxforge.toml.")?;

    let qmp_addr = qemu::get_qmp_address(qemu_config, &project_root);
    let mut client = qmp::QmpClient::connect(&qmp_addr)
        .map_err(|e| format!("Failed to connect to QEMU monitor. Is QEMU running with QMP enabled? Error: {}", e))?;

    let output = client.execute_hmp(command)?;

    #[derive(serde::Serialize)]
    struct MonitorData {
        command: String,
        output: String,
    }

    let resp = SuccessResponse::ok_with_data(
        "Monitor command executed",
        serde_json::to_value(&MonitorData {
            command: command.to_string(),
            output,
        })
        .map_err(|e| format!("Failed to serialize monitor response: {}", e))?,
    );
    serde_json::to_string(&resp).map_err(|e| format!("Failed to serialize response: {}", e))
}

// ---------------------------------------------------------------------------
// hex-dump
// ---------------------------------------------------------------------------

fn handle_hex_dump(file_path_str: &str) -> Result<String, String> {
    let file_path = std::path::Path::new(file_path_str);
    let data = hex::format_hex_dump(file_path)?;

    let resp = SuccessResponse::ok_with_data(
        "Hex dump generated successfully",
        serde_json::to_value(&data)
            .map_err(|e| format!("Failed to serialize hex dump data: {}", e))?,
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
        assert!(output.contains("unknown variant"));
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
        assert!(output.contains("missing field `profile`"));
    }

    #[test]
    fn test_handle_build_missing_project_root() {
        let input = r#"{"cmd":"build","profile":"bootloader"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `project_root`"));
    }

    #[test]
    fn test_handle_list_profiles_missing_project_root() {
        let input = r#"{"cmd":"listProfiles"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `project_root`"));
    }

    #[test]
    fn test_handle_launch_missing_project_root() {
        let input = r#"{"cmd":"launch"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `project_root`"));
    }

    #[test]
    fn test_handle_stop_missing_pid() {
        let input = r#"{"cmd":"stop"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `pid`"));
    }

    #[test]
    fn test_handle_qemu_status_missing_pid() {
        let input = r#"{"cmd":"qemuStatus"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `pid`"));
    }

    #[test]
    fn test_handle_debug_config_missing_project_root() {
        let input = r#"{"cmd":"debugConfig"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `project_root`"));
    }

    #[test]
    fn test_handle_init_missing_project_root() {
        let input = r#"{"cmd":"init","project_name":"my-os"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `project_root`"));
    }

    #[test]
    fn test_handle_init_missing_project_name() {
        let input = r#"{"cmd":"init","project_root":"."}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `project_name`"));
    }

    #[test]
    fn test_handle_hex_dump_missing_file_path() {
        let input = r#"{"cmd":"hexDump"}"#;
        let res = handle_request(input);
        assert!(res.is_err());
        let output = res.unwrap_err();
        assert!(output.contains("missing field `file_path`"));
    }
}
