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

#[derive(serde::Serialize, serde::Deserialize)]
pub struct FileNode {
    pub name: String,
    pub path: String,
    pub is_dir: bool,
    pub is_binary: bool,
}

#[tauri::command]
fn list_workspace_files(dir_path: Option<String>) -> Result<Vec<FileNode>, String> {
    let target = match dir_path {
        Some(p) if !p.trim().is_empty() => PathBuf::from(p),
        _ => std::env::current_dir().map_err(|e| format!("Failed to get cwd: {}", e))?,
    };

    let entries = std::fs::read_dir(&target)
        .map_err(|e| format!("Failed to read directory '{:?}': {}", target, e))?;

    let mut nodes = Vec::new();
    for entry in entries.flatten() {
        let path = entry.path();
        let name = entry.file_name().to_string_lossy().to_string();

        // Skip hidden files/dirs and build noise
        if name.starts_with('.') || name == "target" || name == "node_modules" {
            continue;
        }

        let is_dir = path.is_dir();
        let is_binary = if is_dir {
            false
        } else {
            match std::fs::File::open(&path) {
                Ok(mut file) => {
                    use std::io::Read;
                    let mut buf = [0u8; 512];
                    if let Ok(n) = file.read(&mut buf) {
                        buf[..n].contains(&0)
                    } else {
                        false
                    }
                }
                Err(_) => false,
            }
        };

        nodes.push(FileNode {
            name,
            path: path.to_string_lossy().to_string(),
            is_dir,
            is_binary,
        });
    }

    nodes.sort_by(|a, b| {
        if a.is_dir == b.is_dir {
            a.name.cmp(&b.name)
        } else if a.is_dir {
            std::cmp::Ordering::Less
        } else {
            std::cmp::Ordering::Greater
        }
    });

    Ok(nodes)
}

#[tauri::command]
fn read_workspace_file(file_path: String) -> Result<String, String> {
    let path = PathBuf::from(&file_path);
    let bytes = std::fs::read(&path)
        .map_err(|e| format!("Failed to read file '{}': {}", file_path, e))?;

    if bytes.contains(&0) {
        return Err("ERR_BINARY_FILE".to_string());
    }

    String::from_utf8(bytes).map_err(|e| format!("Invalid UTF-8 in file '{}': {}", file_path, e))
}

#[tauri::command]
fn write_workspace_file(file_path: String, content: String) -> Result<(), String> {
    let path = PathBuf::from(&file_path);
    std::fs::write(&path, content)
        .map_err(|e| format!("Failed to write file '{}': {}", file_path, e))
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
            read_plugin_file,
            list_workspace_files,
            read_workspace_file,
            write_workspace_file
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
