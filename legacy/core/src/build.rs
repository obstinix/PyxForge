use crate::config::{self, BuildProfile, ProjectConfig};
use std::collections::HashSet;
use std::path::Path;
use std::process::Command;

// ---------------------------------------------------------------------------
// Build result
// ---------------------------------------------------------------------------

/// Result of executing a single build profile.
#[derive(Debug)]
pub struct BuildResult {
    pub profile_name: String,
    pub tool: String,
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
}

impl BuildResult {
    pub fn success(&self) -> bool {
        self.exit_code == 0
    }
}

// ---------------------------------------------------------------------------
// Build execution
// ---------------------------------------------------------------------------

/// Execute a build profile, resolving dependencies first (depth-first).
///
/// Returns a Vec of `BuildResult` — one per profile executed (dependencies
/// first, then the requested profile). Stops on the first failure.
pub fn execute_build(
    project_config: &ProjectConfig,
    profile_name: &str,
    project_root: &Path,
) -> Result<Vec<BuildResult>, String> {
    // Resolve the dependency order.
    let order = resolve_build_order(project_config, profile_name)?;

    let mut results = Vec::new();

    for name in &order {
        let profile = config::get_profile(project_config, name)?;
        let result = run_tool(profile, name, project_root)?;
        let failed = !result.success();
        results.push(result);

        if failed {
            // Stop on the first failure — don't build things that depend on
            // a profile that already failed.
            break;
        }
    }

    Ok(results)
}

/// Resolve build order via depth-first traversal of depends_on edges.
/// Detects cycles.
fn resolve_build_order(config: &ProjectConfig, profile_name: &str) -> Result<Vec<String>, String> {
    let mut order = Vec::new();
    let mut visited = HashSet::new();
    let mut in_stack = HashSet::new();

    resolve_dfs(
        config,
        profile_name,
        &mut order,
        &mut visited,
        &mut in_stack,
    )?;

    Ok(order)
}

fn resolve_dfs(
    config: &ProjectConfig,
    name: &str,
    order: &mut Vec<String>,
    visited: &mut HashSet<String>,
    in_stack: &mut HashSet<String>,
) -> Result<(), String> {
    if in_stack.contains(name) {
        return Err(format!("Circular dependency detected involving '{}'", name));
    }
    if visited.contains(name) {
        return Ok(());
    }

    in_stack.insert(name.to_string());

    let profile = config::get_profile(config, name)?;
    for dep in &profile.depends_on {
        resolve_dfs(config, dep, order, visited, in_stack)?;
    }

    in_stack.remove(name);
    visited.insert(name.to_string());
    order.push(name.to_string());

    Ok(())
}

/// Run a single tool invocation for a profile.
fn run_tool(
    profile: &BuildProfile,
    profile_name: &str,
    project_root: &Path,
) -> Result<BuildResult, String> {
    let working_dir = project_root.join(&profile.source_dir);
    if !working_dir.exists() {
        return Err(format!(
            "Profile '{}': source_dir '{}' does not exist (resolved to '{}')",
            profile_name,
            profile.source_dir,
            working_dir.display()
        ));
    }

    // Ensure the output directory exists.
    let output_dir = project_root.join(&profile.output_dir);
    if !output_dir.exists() {
        std::fs::create_dir_all(&output_dir).map_err(|e| {
            format!(
                "Profile '{}': failed to create output_dir '{}': {}",
                profile_name,
                output_dir.display(),
                e
            )
        })?;
    }

    let mut cmd = Command::new(&profile.tool);
    cmd.args(&profile.args);
    cmd.current_dir(&working_dir);

    // Set any profile-specific environment variables.
    for (key, value) in &profile.env {
        cmd.env(key, value);
    }

    let output = cmd.output().map_err(|e| {
        format!(
            "Profile '{}': failed to execute '{}': {}",
            profile_name, profile.tool, e
        )
    })?;

    Ok(BuildResult {
        profile_name: profile_name.to_string(),
        tool: profile.tool.clone(),
        exit_code: output.status.code().unwrap_or(-1),
        stdout: String::from_utf8_lossy(&output.stdout).to_string(),
        stderr: String::from_utf8_lossy(&output.stderr).to_string(),
    })
}

/// List all profiles available in the configuration.
pub fn list_profiles(config: &ProjectConfig) -> Vec<(String, &BuildProfile)> {
    let mut profiles: Vec<(String, &BuildProfile)> = config
        .profiles
        .iter()
        .map(|(name, profile)| (name.clone(), profile))
        .collect();
    profiles.sort_by(|a, b| a.0.cmp(&b.0));
    profiles
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn make_config(toml_str: &str) -> ProjectConfig {
        toml::from_str(toml_str).unwrap()
    }

    #[test]
    fn test_resolve_build_order_no_deps() {
        let config = make_config(
            r#"
[project]
name = "test"

[profiles.bootloader]
tool = "nasm"
"#,
        );
        let order = resolve_build_order(&config, "bootloader").unwrap();
        assert_eq!(order, vec!["bootloader"]);
    }

    #[test]
    fn test_resolve_build_order_with_deps() {
        let config = make_config(
            r#"
[project]
name = "test"

[profiles.bootloader]
tool = "nasm"

[profiles.kernel]
tool = "make"
depends_on = ["bootloader"]
"#,
        );
        let order = resolve_build_order(&config, "kernel").unwrap();
        assert_eq!(order, vec!["bootloader", "kernel"]);
    }

    #[test]
    fn test_resolve_build_order_diamond_deps() {
        let config = make_config(
            r#"
[project]
name = "test"

[profiles.common]
tool = "make"

[profiles.boot]
tool = "nasm"
depends_on = ["common"]

[profiles.drivers]
tool = "gcc"
depends_on = ["common"]

[profiles.kernel]
tool = "make"
depends_on = ["boot", "drivers"]
"#,
        );
        let order = resolve_build_order(&config, "kernel").unwrap();
        // common should appear once, before boot and drivers
        assert_eq!(order[0], "common");
        assert!(order.contains(&"boot".to_string()));
        assert!(order.contains(&"drivers".to_string()));
        assert_eq!(order.last().unwrap(), "kernel");
        assert_eq!(order.len(), 4);
    }

    #[test]
    fn test_resolve_build_order_circular() {
        let toml_str = r#"
[project]
name = "test"

[profiles.a]
tool = "make"
depends_on = ["b"]

[profiles.b]
tool = "make"
depends_on = ["a"]
"#;
        // Note: circular deps would fail validation in config::validate_config
        // only for missing deps. Here we test the DFS cycle detection.
        let config: ProjectConfig = toml::from_str(toml_str).unwrap();
        let result = resolve_build_order(&config, "a");
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Circular dependency"));
    }

    #[test]
    fn test_list_profiles_sorted() {
        let config = make_config(
            r#"
[project]
name = "test"

[profiles.kernel]
tool = "make"

[profiles.bootloader]
tool = "nasm"

[profiles.drivers]
tool = "gcc"
"#,
        );
        let profiles = list_profiles(&config);
        let names: Vec<&str> = profiles.iter().map(|(n, _)| n.as_str()).collect();
        assert_eq!(names, vec!["bootloader", "drivers", "kernel"]);
    }
}
