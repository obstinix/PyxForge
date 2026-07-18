use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Stdio};

// Learn more about Tauri commands at https://tauri.app/develop/calling-rust/
#[tauri::command]
fn greet(name: &str) -> String {
    format!("Hello, {}! You've been greeted from Rust!", name)
}

#[tauri::command]
fn call_core(request_json: String) -> Result<String, String> {
    let binary_name = if cfg!(target_os = "windows") {
        "pyxforge-core.exe"
    } else {
        "pyxforge-core"
    };

    let paths = vec![
        PathBuf::from("../core/target/debug").join(binary_name),
        PathBuf::from("core/target/debug").join(binary_name),
        PathBuf::from("target/debug").join(binary_name),
    ];

    let mut core_path = None;
    for p in paths {
        if p.exists() {
            core_path = Some(p);
            break;
        }
    }

    let core_path = core_path.ok_or_else(|| {
        "Could not find pyxforge-core binary in target debug directories".to_string()
    })?;

    let mut child = Command::new(core_path)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("Failed to spawn pyxforge-core: {}", e))?;

    {
        let stdin = child
            .stdin
            .as_mut()
            .ok_or_else(|| "Failed to open core stdin".to_string())?;
        stdin
            .write_all(request_json.as_bytes())
            .map_err(|e| format!("Failed to write request: {}", e))?;
        stdin
            .write_all(b"\n")
            .map_err(|e| format!("Failed to write newline: {}", e))?;
    }

    let output = child
        .wait_with_output()
        .map_err(|e| format!("Failed to read core output: {}", e))?;
    let stdout_str = String::from_utf8_lossy(&output.stdout).trim().to_string();

    if output.status.success() {
        Ok(stdout_str)
    } else {
        Err(format!(
            "Core exited with non-zero code {}. Stderr: {}",
            output.status.code().unwrap_or(-1),
            String::from_utf8_lossy(&output.stderr)
        ))
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![greet, call_core])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
