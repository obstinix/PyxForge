use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Inbound requests
// ---------------------------------------------------------------------------

/// Top-level request envelope. Every request carries a `cmd` field; additional
/// fields are command-specific and deserialized via `#[serde(flatten)]`.
#[derive(Debug, Deserialize)]
pub struct Request {
    pub cmd: String,
    /// The project root directory (required for commands that operate on a project).
    #[serde(default)]
    pub project_root: Option<String>,
    /// The build profile name to use (required for the `build` command).
    #[serde(default)]
    pub profile: Option<String>,
    /// Optional flag to launch QEMU with debugger attached (pre-paused).
    #[serde(default)]
    pub debug: Option<bool>,
    /// Optional PID of QEMU process to stop.
    #[serde(default)]
    pub pid: Option<u32>,
    /// Optional project name for scaffolding initialization.
    #[serde(default)]
    pub project_name: Option<String>,
    /// Optional file path for hex dump operations.
    #[serde(default)]
    pub file_path: Option<String>,
    /// Optional template name (e.g. "assembly", "rust") for scaffolding.
    #[serde(default)]
    pub template: Option<String>,
}

// ---------------------------------------------------------------------------
// Outbound responses
// ---------------------------------------------------------------------------

#[derive(Debug, Serialize)]
pub struct SuccessResponse {
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
}

#[derive(Debug, Serialize)]
pub struct ErrorResponse {
    pub status: String,
    pub message: String,
}

// ---------------------------------------------------------------------------
// Build-specific data carried inside SuccessResponse.data
// ---------------------------------------------------------------------------

#[derive(Debug, Serialize)]
pub struct BuildResultData {
    pub profile: String,
    pub tool: String,
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
}

#[derive(Debug, Serialize)]
pub struct ListProfilesData {
    pub profiles: Vec<ProfileSummary>,
}

#[derive(Debug, Serialize)]
pub struct ProfileSummary {
    pub name: String,
    pub tool: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct QemuLaunchData {
    pub pid: u32,
    pub port: u16,
    pub args_used: Vec<String>,
}

#[derive(Debug, Serialize)]
pub struct DebugConfigData {
    pub gdb_executable: String,
    pub architecture: String,
    pub target: String,
    pub setup_commands: Vec<String>,
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

impl SuccessResponse {
    #[allow(dead_code)]
    pub fn ok(message: impl Into<String>) -> Self {
        Self {
            status: "ok".to_string(),
            version: None,
            message: message.into(),
            data: None,
        }
    }

    pub fn ok_with_version(message: impl Into<String>, version: impl Into<String>) -> Self {
        Self {
            status: "ok".to_string(),
            version: Some(version.into()),
            message: message.into(),
            data: None,
        }
    }

    pub fn ok_with_data(message: impl Into<String>, data: serde_json::Value) -> Self {
        Self {
            status: "ok".to_string(),
            version: None,
            message: message.into(),
            data: Some(data),
        }
    }
}

impl ErrorResponse {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            status: "error".to_string(),
            message: message.into(),
        }
    }
}
