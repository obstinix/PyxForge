use std::io::Write as _;
use std::path::PathBuf;
use std::process::{Command, Stdio};
use std::sync::Mutex;
use tauri::Emitter;
use portable_pty::{native_pty_system, CommandBuilder, MasterPty, PtySize};

struct PtyState {
    master: Mutex<Option<Box<dyn MasterPty + Send>>>,
    writer: Mutex<Option<Box<dyn std::io::Write + Send>>>,
}

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

#[tauri::command]
fn spawn_pty(
    state: tauri::State<'_, PtyState>,
    app_handle: tauri::AppHandle,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    let pty_system = native_pty_system();
    let cmd = if cfg!(target_os = "windows") {
        CommandBuilder::new("powershell.exe")
    } else {
        CommandBuilder::new("sh")
    };

    let pair = pty_system
        .openpty(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| format!("Failed to open PTY: {}", e))?;

    let _child = pair
        .slave
        .spawn_command(cmd)
        .map_err(|e| format!("Failed to spawn command in PTY: {}", e))?;

    drop(pair.slave);

    let master = pair.master;
    let writer = master
        .take_writer()
        .map_err(|e| format!("Failed to take PTY writer: {}", e))?;

    let mut reader = master
        .try_clone_reader()
        .map_err(|e| format!("Failed to clone PTY reader: {}", e))?;

    *state.master.lock().unwrap() = Some(master);
    *state.writer.lock().unwrap() = Some(writer);

    std::thread::spawn(move || {
        let mut buf = [0u8; 1024];
        loop {
            match reader.read(&mut buf) {
                Ok(n) if n > 0 => {
                    let text = String::from_utf8_lossy(&buf[..n]).to_string();
                    let _ = app_handle.emit("pty-data", text);
                }
                _ => break,
            }
        }
    });

    Ok(())
}

#[tauri::command]
fn write_to_pty(
    state: tauri::State<'_, PtyState>,
    data: String,
) -> Result<(), String> {
    if let Some(writer) = state.writer.lock().unwrap().as_mut() {
        writer
            .write_all(data.as_bytes())
            .map_err(|e| format!("Failed to write to PTY: {}", e))?;
        writer.flush().map_err(|e| format!("Failed to flush PTY: {}", e))?;
    }
    Ok(())
}

#[tauri::command]
fn resize_pty(
    state: tauri::State<'_, PtyState>,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    if let Some(master) = state.master.lock().unwrap().as_mut() {
        master
            .resize(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| format!("Failed to resize PTY: {}", e))?;
    }
    Ok(())
}

#[tauri::command]
fn read_plugin_file(path: String) -> Result<String, String> {
    std::fs::read_to_string(path).map_err(|e| format!("Failed to read plugin file: {}", e))
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .manage(PtyState {
            master: Mutex::new(None),
            writer: Mutex::new(None),
        })
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            greet,
            call_core,
            spawn_pty,
            write_to_pty,
            resize_pty,
            read_plugin_file
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
