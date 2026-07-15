use crate::config::{GdbConfig, QemuConfig};
use serde::Serialize;

/// The debug launch configuration produced by the core for the extension
/// to pass to the Native Debug adapter.
#[derive(Debug, Serialize)]
pub struct DebugLaunchConfig {
    pub gdb_executable: String,
    pub architecture: String,
    pub target: String,
    pub setup_commands: Vec<String>,
}

/// Build a debug launch configuration from the project's GDB and QEMU configs.
pub fn build_launch_config(gdb_config: &GdbConfig, qemu_config: &QemuConfig) -> DebugLaunchConfig {
    let architecture = resolve_architecture(&gdb_config.architecture);
    let port = qemu_config.debug.gdb_port;
    let target = format!(":{}", port);

    let mut setup_commands = Vec::new();
    setup_commands.push(format!("set architecture {}", architecture));

    DebugLaunchConfig {
        gdb_executable: gdb_config.executable.clone(),
        architecture: architecture.to_string(),
        target,
        setup_commands,
    }
}

/// Map user-facing architecture names to GDB-understood values.
/// Currently "auto" defaults to "i8086" (Track B real-mode).
fn resolve_architecture(arch: &str) -> &str {
    match arch {
        "auto" => "i8086",
        other => other,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::{QemuConfig, QemuDebugConfig};

    fn make_qemu_config(port: u16) -> QemuConfig {
        QemuConfig {
            executable: "qemu-system-x86_64".to_string(),
            machine: "pc".to_string(),
            memory: "128M".to_string(),
            boot_image: "build/boot.bin".to_string(),
            extra_args: Vec::new(),
            debug: QemuDebugConfig {
                enabled: true,
                gdb_port: port,
            },
        }
    }

    #[test]
    fn test_build_launch_config_defaults() {
        let gdb = GdbConfig::default();
        let qemu = make_qemu_config(1234);
        let config = build_launch_config(&gdb, &qemu);

        assert_eq!(config.gdb_executable, "gdb");
        assert_eq!(config.architecture, "i8086");
        assert_eq!(config.target, ":1234");
        assert_eq!(config.setup_commands, vec!["set architecture i8086"]);
    }

    #[test]
    fn test_build_launch_config_protected_mode() {
        let gdb = GdbConfig {
            executable: "x86_64-elf-gdb".to_string(),
            architecture: "i386".to_string(),
        };
        let qemu = make_qemu_config(5678);
        let config = build_launch_config(&gdb, &qemu);

        assert_eq!(config.gdb_executable, "x86_64-elf-gdb");
        assert_eq!(config.architecture, "i386");
        assert_eq!(config.target, ":5678");
        assert_eq!(config.setup_commands, vec!["set architecture i386"]);
    }

    #[test]
    fn test_build_launch_config_long_mode() {
        let gdb = GdbConfig {
            executable: "gdb".to_string(),
            architecture: "i386:x86-64".to_string(),
        };
        let qemu = make_qemu_config(1234);
        let config = build_launch_config(&gdb, &qemu);

        assert_eq!(config.architecture, "i386:x86-64");
        assert_eq!(config.setup_commands, vec!["set architecture i386:x86-64"]);
    }

    #[test]
    fn test_build_launch_config_auto_resolves_to_i8086() {
        let gdb = GdbConfig {
            executable: "gdb".to_string(),
            architecture: "auto".to_string(),
        };
        let qemu = make_qemu_config(1234);
        let config = build_launch_config(&gdb, &qemu);

        assert_eq!(config.architecture, "i8086");
        assert_eq!(config.setup_commands, vec!["set architecture i8086"]);
    }
}
