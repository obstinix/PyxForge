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
}
