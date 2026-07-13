use crate::config::QemuConfig;
use std::path::Path;
use std::process::{Command, Stdio};

pub fn build_qemu_args(config: &QemuConfig, project_root: &Path, debug: bool) -> Vec<String> {
    let mut args = vec![
        "-machine".to_string(),
        config.machine.clone(),
        "-m".to_string(),
        config.memory.clone(),
    ];

    let boot_image_path = project_root.join(&config.boot_image);
    // Use raw string format for drive path compatibility.
    let drive_arg = format!("format=raw,file={}", boot_image_path.to_string_lossy());
    args.push("-drive".to_string());
    args.push(drive_arg);

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

    args
}

pub fn launch_qemu(
    config: &QemuConfig,
    project_root: &Path,
    debug: bool,
) -> Result<(u32, Vec<String>), String> {
    let args = build_qemu_args(config, project_root, debug);

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

#[cfg(target_os = "windows")]
pub fn stop_qemu(pid: u32) -> Result<(), String> {
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
pub fn stop_qemu(pid: u32) -> Result<(), String> {
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
            boot_image: "build/boot.bin".to_string(),
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
    }

    #[test]
    fn test_build_qemu_args_custom_debug_port() {
        let config = QemuConfig {
            executable: "qemu-system-x86_64".to_string(),
            machine: "pc".to_string(),
            memory: "128M".to_string(),
            boot_image: "build/boot.bin".to_string(),
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
            boot_image: "build/boot.bin".to_string(),
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
}
