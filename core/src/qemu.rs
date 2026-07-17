use crate::config::QemuConfig;
use std::path::Path;
use std::process::{Command, Stdio};
use std::time::Duration;

#[derive(Debug, Clone, PartialEq)]
pub enum QmpAddress {
    Unix(std::path::PathBuf),
    Tcp(String),
}

/// Determines the QMP address based on the host operating system.
///
/// On Windows, we use a local TCP loopback address (using `gdb_port + 1` to prevent port conflicts).
/// On UNIX-like systems, we prefer a local UNIX domain socket created in the project's build directory.
pub fn get_qmp_address(config: &QemuConfig, project_root: &Path) -> QmpAddress {
    if cfg!(target_os = "windows") {
        QmpAddress::Tcp(format!("127.0.0.1:{}", config.debug.gdb_port + 1))
    } else {
        QmpAddress::Unix(project_root.join("build").join("qemu-qmp.sock"))
    }
}

pub fn build_qemu_args(config: &QemuConfig, project_root: &Path, debug: bool) -> Vec<String> {
    let mut args = vec![
        "-machine".to_string(),
        config.machine.clone(),
        "-m".to_string(),
        config.memory.clone(),
    ];

    if let Some(boot_image) = &config.boot_image {
        let boot_image_path = project_root.join(boot_image);
        let drive_arg = format!("format=raw,file={}", boot_image_path.to_string_lossy());
        args.push("-drive".to_string());
        args.push(drive_arg);
    }

    if let Some(kernel) = &config.kernel {
        let kernel_path = project_root.join(kernel);
        args.push("-kernel".to_string());
        args.push(kernel_path.to_string_lossy().to_string());
    }

    for arg in &config.extra_args {
        args.push(arg.clone());
    }

    if debug && config.debug.enabled {
        if config.debug.gdb_port == 1234 {
            args.push("-s".to_string());
        } else {
            args.push("-gdb".to_string());
            args.push(format!("tcp::{}", config.debug.gdb_port));
        }
        args.push("-S".to_string());
    }

    let qmp_addr = get_qmp_address(config, project_root);
    args.push("-qmp".to_string());
    match qmp_addr {
        QmpAddress::Unix(path) => {
            args.push(format!(
                "unix:{},server=on,wait=off",
                path.to_string_lossy()
            ));
        }
        QmpAddress::Tcp(addr) => {
            args.push(format!("tcp:{},server=on,wait=off", addr));
        }
    }

    args
}

pub fn launch_qemu(
    config: &QemuConfig,
    project_root: &Path,
    debug: bool,
) -> Result<(u32, Vec<String>), String> {
    let args = build_qemu_args(config, project_root, debug);

    // If Unix socket is used, make sure the directory containing it exists
    if let Some(parent) = match &get_qmp_address(config, project_root) {
        QmpAddress::Unix(path) => path.parent(),
        _ => None,
    } {
        std::fs::create_dir_all(parent).map_err(|e| {
            format!(
                "Failed to create output directory for QMP socket '{}': {}",
                parent.display(),
                e
            )
        })?;
    }

    // Spawn QEMU as a detached process.
    let child = Command::new(&config.executable)
        .args(&args)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map_err(|e| {
            format!(
                "Failed to spawn QEMU binary '{}': {}. Make sure QEMU is installed and in your PATH.",
                config.executable, e
            )
        })?;

    Ok((child.id(), args))
}

pub fn stop_qemu(
    pid: u32,
    project_root: Option<&Path>,
    config: Option<&QemuConfig>,
) -> Result<(), String> {
    if let (Some(root), Some(cfg)) = (project_root, config) {
        let qmp_addr = get_qmp_address(cfg, root);
        if let Ok(mut client) = crate::qmp::QmpClient::connect(&qmp_addr) {
            let shutdown_ok = client.graceful_shutdown().is_ok();
            if shutdown_ok {
                // Wait up to 2 seconds (20 iterations * 100ms) for it to exit
                for _ in 0..20 {
                    std::thread::sleep(Duration::from_millis(100));
                    if !is_qemu_running(pid) {
                        return Ok(());
                    }
                }
            }
        }
    }

    // Fallback to raw kill
    raw_kill(pid)
}

#[cfg(target_os = "windows")]
fn raw_kill(pid: u32) -> Result<(), String> {
    let mut cmd = Command::new("taskkill");
    cmd.arg("/F").arg("/PID").arg(pid.to_string());
    let output = cmd
        .output()
        .map_err(|e| format!("Failed to run taskkill: {}", e))?;
    if output.status.success() {
        Ok(())
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        Err(format!("taskkill failed: {}", stderr.trim()))
    }
}

#[cfg(not(target_os = "windows"))]
fn raw_kill(pid: u32) -> Result<(), String> {
    let mut cmd = Command::new("kill");
    cmd.arg("-9").arg(pid.to_string());
    let output = cmd
        .output()
        .map_err(|e| format!("Failed to run kill: {}", e))?;
    if output.status.success() {
        Ok(())
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        Err(format!("kill failed: {}", stderr.trim()))
    }
}

#[cfg(target_os = "windows")]
pub fn is_qemu_running(pid: u32) -> bool {
    let mut cmd = Command::new("tasklist");
    cmd.arg("/FI").arg(format!("PID eq {}", pid));
    if let Ok(output) = cmd.output() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        stdout.contains(&pid.to_string())
    } else {
        false
    }
}

#[cfg(not(target_os = "windows"))]
pub fn is_qemu_running(pid: u32) -> bool {
    let mut cmd = Command::new("kill");
    cmd.arg("-0").arg(pid.to_string());
    if let Ok(output) = cmd.output() {
        output.status.success()
    } else {
        false
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::QemuDebugConfig;

    #[test]
    fn test_build_qemu_args_default() {
        let config = QemuConfig {
            executable: "qemu-system-x86_64".to_string(),
            machine: "pc".to_string(),
            memory: "128M".to_string(),
            boot_image: Some("build/boot.bin".to_string()),
            kernel: None,
            extra_args: Vec::new(),
            debug: QemuDebugConfig {
                enabled: true,
                gdb_port: 1234,
            },
        };
        let project_root = Path::new("C:\\Projects\\my-os");
        let args = build_qemu_args(&config, project_root, true);
        assert!(args.contains(&"-machine".to_string()));
        assert!(args.contains(&"pc".to_string()));
        assert!(args.contains(&"-m".to_string()));
        assert!(args.contains(&"128M".to_string()));
        assert!(args.contains(&"-drive".to_string()));
        assert!(
            args.contains(&"format=raw,file=C:\\Projects\\my-os\\build/boot.bin".to_string())
                || args
                    .contains(&"format=raw,file=C:\\Projects\\my-os\\build\\boot.bin".to_string())
        );
        assert!(args.contains(&"-s".to_string()));
        assert!(args.contains(&"-S".to_string()));

        // QMP assertions
        assert!(args.contains(&"-qmp".to_string()));
        let qmp_val = args
            .iter()
            .find(|arg| arg.contains("server=on") && arg.contains("wait=off"))
            .expect("No QMP argument found");
        if cfg!(target_os = "windows") {
            assert!(qmp_val.contains("tcp:127.0.0.1:1235"));
        } else {
            assert!(qmp_val.contains("unix:"));
            assert!(qmp_val.contains("qemu-qmp.sock"));
        }
    }

    #[test]
    fn test_build_qemu_args_custom_debug_port() {
        let config = QemuConfig {
            executable: "qemu-system-x86_64".to_string(),
            machine: "pc".to_string(),
            memory: "128M".to_string(),
            boot_image: Some("build/boot.bin".to_string()),
            kernel: None,
            extra_args: Vec::new(),
            debug: QemuDebugConfig {
                enabled: true,
                gdb_port: 5678,
            },
        };
        let project_root = Path::new("C:\\Projects\\my-os");
        let args = build_qemu_args(&config, project_root, true);
        assert!(args.contains(&"-gdb".to_string()));
        assert!(args.contains(&"tcp::5678".to_string()));
        assert!(args.contains(&"-S".to_string()));
        assert!(!args.contains(&"-s".to_string()));
    }

    #[test]
    fn test_build_qemu_args_no_debug() {
        let config = QemuConfig {
            executable: "qemu-system-x86_64".to_string(),
            machine: "pc".to_string(),
            memory: "128M".to_string(),
            boot_image: Some("build/boot.bin".to_string()),
            kernel: None,
            extra_args: Vec::new(),
            debug: QemuDebugConfig {
                enabled: true,
                gdb_port: 1234,
            },
        };
        let project_root = Path::new("C:\\Projects\\my-os");
        let args = build_qemu_args(&config, project_root, false);
        assert!(!args.contains(&"-s".to_string()));
        assert!(!args.contains(&"-S".to_string()));
    }

    #[test]
    fn test_build_qemu_args_kernel_boot() {
        let config = QemuConfig {
            executable: "qemu-system-x86_64".to_string(),
            machine: "pc".to_string(),
            memory: "256M".to_string(),
            boot_image: None,
            kernel: Some("build/kernel.elf".to_string()),
            extra_args: Vec::new(),
            debug: QemuDebugConfig {
                enabled: true,
                gdb_port: 1234,
            },
        };
        let project_root = Path::new("C:\\Projects\\my-os");
        let args = build_qemu_args(&config, project_root, true);
        assert!(args.contains(&"-machine".to_string()));
        assert!(args.contains(&"pc".to_string()));
        assert!(args.contains(&"-m".to_string()));
        assert!(args.contains(&"256M".to_string()));
        assert!(!args.contains(&"-drive".to_string()));
        assert!(args.contains(&"-kernel".to_string()));
        assert!(
            args.contains(&"C:\\Projects\\my-os\\build/kernel.elf".to_string())
                || args.contains(&"C:\\Projects\\my-os\\build\\kernel.elf".to_string())
        );
    }
}
