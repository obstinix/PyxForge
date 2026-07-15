use serde::Deserialize;
use std::collections::HashMap;
use std::path::Path;

// ---------------------------------------------------------------------------
// pyxforge.toml schema
// ---------------------------------------------------------------------------

/// Root of the `pyxforge.toml` configuration file.
#[derive(Debug, Deserialize)]
pub struct ProjectConfig {
    pub project: ProjectMeta,
    /// Keyed by profile name (e.g. "bootloader", "kernel").
    #[serde(default)]
    pub profiles: HashMap<String, BuildProfile>,
    /// Optional QEMU configuration.
    #[serde(default)]
    pub qemu: Option<QemuConfig>,
    /// Optional GDB configuration.
    #[serde(default)]
    pub gdb: Option<GdbConfig>,
}

#[derive(Debug, Deserialize)]
pub struct ProjectMeta {
    pub name: String,
    #[serde(default)]
    #[allow(dead_code)]
    pub description: Option<String>,
}

/// A single build profile definition.
#[derive(Debug, Clone, Deserialize)]
pub struct BuildProfile {
    /// The build tool to invoke (e.g. "nasm", "make", "gcc", "rustc").
    pub tool: String,

    /// Human-readable description of this profile.
    #[serde(default)]
    pub description: Option<String>,

    /// Working directory relative to the project root. Defaults to ".".
    #[serde(default = "default_source_dir")]
    pub source_dir: String,

    /// Output directory relative to the project root. Defaults to "build".
    #[serde(default = "default_output_dir")]
    pub output_dir: String,

    /// Arguments to pass to the tool.
    #[serde(default)]
    pub args: Vec<String>,

    /// Environment variables to set when invoking the tool.
    #[serde(default)]
    pub env: HashMap<String, String>,

    /// Other profiles that must be built first (by name).
    #[serde(default)]
    pub depends_on: Vec<String>,
}

fn default_source_dir() -> String {
    ".".to_string()
}

fn default_output_dir() -> String {
    "build".to_string()
}

#[derive(Debug, Deserialize, Clone)]
pub struct QemuConfig {
    #[serde(default = "default_qemu_executable")]
    pub executable: String,
    #[serde(default = "default_qemu_machine")]
    pub machine: String,
    #[serde(default = "default_qemu_memory")]
    pub memory: String,
    pub boot_image: String,
    #[serde(default)]
    pub extra_args: Vec<String>,
    #[serde(default)]
    pub debug: QemuDebugConfig,
}

#[derive(Debug, Deserialize, Clone)]
pub struct QemuDebugConfig {
    #[serde(default = "default_true")]
    pub enabled: bool,
    #[serde(default = "default_gdb_port")]
    pub gdb_port: u16,
}

impl Default for QemuDebugConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            gdb_port: 1234,
        }
    }
}

fn default_qemu_executable() -> String {
    "qemu-system-x86_64".to_string()
}

fn default_qemu_machine() -> String {
    "pc".to_string()
}

fn default_qemu_memory() -> String {
    "128M".to_string()
}

fn default_true() -> bool {
    true
}

fn default_gdb_port() -> u16 {
    1234
}

#[derive(Debug, Deserialize, Clone)]
pub struct GdbConfig {
    #[serde(default = "default_gdb_executable")]
    pub executable: String,
    #[serde(default = "default_gdb_architecture")]
    pub architecture: String,
}

impl Default for GdbConfig {
    fn default() -> Self {
        Self {
            executable: default_gdb_executable(),
            architecture: default_gdb_architecture(),
        }
    }
}

fn default_gdb_executable() -> String {
    "gdb".to_string()
}

fn default_gdb_architecture() -> String {
    "i8086".to_string()
}

/// Valid GDB architecture values.
const VALID_GDB_ARCHITECTURES: &[&str] = &["i8086", "i386", "i386:x86-64", "auto"];

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

const CONFIG_FILE_NAME: &str = "pyxforge.toml";

/// Load and parse `pyxforge.toml` from the given project root directory.
pub fn load_config(project_root: &Path) -> Result<ProjectConfig, String> {
    let config_path = project_root.join(CONFIG_FILE_NAME);
    if !config_path.exists() {
        return Err(format!(
            "No {} found in {}",
            CONFIG_FILE_NAME,
            project_root.display()
        ));
    }

    let content = std::fs::read_to_string(&config_path)
        .map_err(|e| format!("Failed to read {}: {}", config_path.display(), e))?;

    let config: ProjectConfig = toml::from_str(&content)
        .map_err(|e| format!("Failed to parse {}: {}", config_path.display(), e))?;

    validate_config(&config)?;

    Ok(config)
}

/// Look up a profile by name, returning a descriptive error if not found.
pub fn get_profile<'a>(
    config: &'a ProjectConfig,
    profile_name: &str,
) -> Result<&'a BuildProfile, String> {
    config.profiles.get(profile_name).ok_or_else(|| {
        let available: Vec<&str> = config.profiles.keys().map(|s| s.as_str()).collect();
        format!(
            "Unknown profile '{}'. Available profiles: [{}]",
            profile_name,
            available.join(", ")
        )
    })
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

fn validate_config(config: &ProjectConfig) -> Result<(), String> {
    if config.project.name.is_empty() {
        return Err("project.name must not be empty".to_string());
    }

    // Check that depends_on references exist.
    for (name, profile) in &config.profiles {
        for dep in &profile.depends_on {
            if !config.profiles.contains_key(dep) {
                return Err(format!(
                    "Profile '{}' depends on '{}', which does not exist",
                    name, dep
                ));
            }
        }
    }

    // Validate QEMU configuration if present.
    if config
        .qemu
        .as_ref()
        .is_some_and(|q| q.boot_image.is_empty())
    {
        return Err("qemu.boot_image must not be empty if [qemu] is configured".to_string());
    }

    // Validate GDB configuration if present.
    if let Some(gdb) = &config.gdb
        && !VALID_GDB_ARCHITECTURES.contains(&gdb.architecture.as_str())
    {
        return Err(format!(
            "gdb.architecture '{}' is invalid. Valid values: {}",
            gdb.architecture,
            VALID_GDB_ARCHITECTURES.join(", ")
        ));
    }

    Ok(())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_minimal_config() {
        let toml_str = r#"
[project]
name = "test-os"

[profiles.bootloader]
tool = "nasm"
args = ["-f", "bin", "boot.asm", "-o", "boot.bin"]
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert_eq!(config.project.name, "test-os");
        assert_eq!(config.profiles.len(), 1);

        let boot = config.profiles.get("bootloader").unwrap();
        assert_eq!(boot.tool, "nasm");
        assert_eq!(boot.args, vec!["-f", "bin", "boot.asm", "-o", "boot.bin"]);
        assert_eq!(boot.source_dir, ".");
        assert_eq!(boot.output_dir, "build");
        assert!(boot.depends_on.is_empty());
    }

    #[test]
    fn test_parse_full_config() {
        let toml_str = r#"
[project]
name = "my-os"
description = "A test operating system"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the bootloader"
source_dir = "boot"
output_dir = "build"
args = ["-f", "bin", "boot.asm", "-o", "boot.bin"]

[profiles.kernel]
tool = "make"
description = "Build the kernel"
source_dir = "."
output_dir = "build"
args = ["all"]
depends_on = ["bootloader"]

[profiles.kernel.env]
CC = "x86_64-elf-gcc"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert_eq!(config.project.name, "my-os");
        assert_eq!(config.profiles.len(), 2);

        let kernel = config.profiles.get("kernel").unwrap();
        assert_eq!(kernel.tool, "make");
        assert_eq!(kernel.depends_on, vec!["bootloader"]);
        assert_eq!(kernel.env.get("CC").unwrap(), "x86_64-elf-gcc");
    }

    #[test]
    fn test_validate_missing_dependency() {
        let toml_str = r#"
[project]
name = "test-os"

[profiles.kernel]
tool = "make"
depends_on = ["bootloader"]
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let result = validate_config(&config);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("does not exist"));
    }

    #[test]
    fn test_validate_empty_project_name() {
        let toml_str = r#"
[project]
name = ""

[profiles.bootloader]
tool = "nasm"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let result = validate_config(&config);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("must not be empty"));
    }

    #[test]
    fn test_get_profile_found() {
        let toml_str = r#"
[project]
name = "test-os"

[profiles.bootloader]
tool = "nasm"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let profile = get_profile(&config, "bootloader");
        assert!(profile.is_ok());
        assert_eq!(profile.unwrap().tool, "nasm");
    }

    #[test]
    fn test_get_profile_not_found() {
        let toml_str = r#"
[project]
name = "test-os"

[profiles.bootloader]
tool = "nasm"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let result = get_profile(&config, "nonexistent");
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Unknown profile"));
    }

    #[test]
    fn test_parse_qemu_config_defaults() {
        let toml_str = r#"
[project]
name = "test-os"

[qemu]
boot_image = "build/boot.bin"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert!(config.qemu.is_some());
        let qemu = config.qemu.unwrap();
        assert_eq!(qemu.executable, "qemu-system-x86_64");
        assert_eq!(qemu.machine, "pc");
        assert_eq!(qemu.memory, "128M");
        assert_eq!(qemu.boot_image, "build/boot.bin");
        assert_eq!(qemu.extra_args.len(), 0);
        assert!(qemu.debug.enabled);
        assert_eq!(qemu.debug.gdb_port, 1234);
    }

    #[test]
    fn test_parse_qemu_config_custom() {
        let toml_str = r#"
[project]
name = "test-os"

[qemu]
executable = "qemu-system-i386"
machine = "q35"
memory = "256M"
boot_image = "build/custom_boot.bin"
extra_args = ["-nographic", "-serial", "mon:stdio"]

[qemu.debug]
enabled = false
gdb_port = 5678
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert!(config.qemu.is_some());
        let qemu = config.qemu.unwrap();
        assert_eq!(qemu.executable, "qemu-system-i386");
        assert_eq!(qemu.machine, "q35");
        assert_eq!(qemu.memory, "256M");
        assert_eq!(qemu.boot_image, "build/custom_boot.bin");
        assert_eq!(qemu.extra_args, vec!["-nographic", "-serial", "mon:stdio"]);
        assert!(!qemu.debug.enabled);
        assert_eq!(qemu.debug.gdb_port, 5678);
    }

    #[test]
    fn test_validate_missing_qemu_boot_image() {
        let toml_str = r#"
[project]
name = "test-os"

[qemu]
boot_image = ""
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let result = validate_config(&config);
        assert!(result.is_err());
        assert!(
            result
                .unwrap_err()
                .contains("qemu.boot_image must not be empty")
        );
    }

    #[test]
    fn test_parse_gdb_config_defaults() {
        let toml_str = r#"
[project]
name = "test-os"

[gdb]
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert!(config.gdb.is_some());
        let gdb = config.gdb.unwrap();
        assert_eq!(gdb.executable, "gdb");
        assert_eq!(gdb.architecture, "i8086");
    }

    #[test]
    fn test_parse_gdb_config_custom() {
        let toml_str = r#"
[project]
name = "test-os"

[gdb]
executable = "x86_64-elf-gdb"
architecture = "i386"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert!(config.gdb.is_some());
        let gdb = config.gdb.unwrap();
        assert_eq!(gdb.executable, "x86_64-elf-gdb");
        assert_eq!(gdb.architecture, "i386");
    }

    #[test]
    fn test_validate_invalid_gdb_architecture() {
        let toml_str = r#"
[project]
name = "test-os"

[gdb]
architecture = "arm64"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let result = validate_config(&config);
        assert!(result.is_err());
        assert!(
            result
                .unwrap_err()
                .contains("gdb.architecture 'arm64' is invalid")
        );
    }

    #[test]
    fn test_no_gdb_section_is_valid() {
        let toml_str = r#"
[project]
name = "test-os"
"#;
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        assert!(config.gdb.is_none());
        let result = validate_config(&config);
        assert!(result.is_ok());
    }
}
