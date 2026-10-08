use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Inbound requests
// ---------------------------------------------------------------------------

/// Top-level request envelope. Every request carries a `cmd` field; additional
/// fields are command-specific and deserialized via `#[serde(flatten)]`.
#[derive(Debug, Deserialize)]
#[serde(tag = "cmd", rename_all = "camelCase")]
pub enum Request {
    /// Simple ping to check if the core process is alive and responsive.
    Ping,
    /// Build the project profile targets using the configured compiler tools.
    Build {
        project_root: String,
        profile: String,
    },
    /// Retrieve lists of all configured build profiles and tool settings.
    ListProfiles {
        project_root: String,
    },
    /// Launch the QEMU machine emulator with the selected boot images.
    Launch {
        project_root: String,
        debug: Option<bool>,
    },
    Stop {
        pid: u32,
        #[serde(default)]
        project_root: Option<String>,
    },
    QemuStatus {
        pid: u32,
    },
    DebugConfig {
        project_root: String,
        profile: Option<String>,
    },
    Init {
        project_root: String,
        project_name: String,
        template: Option<String>,
    },
    HexDump {
        file_path: String,
    },
    QemuSnapshotSave {
        project_root: String,
        tag: String,
    },
    QemuSnapshotLoad {
        project_root: String,
        tag: String,
    },
    QemuSnapshotDelete {
        project_root: String,
        tag: String,
    },
    QemuSnapshotList {
        project_root: String,
    },
    QemuMonitorCommand {
        project_root: String,
        command: String,
    },
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

/// A single parsed compiler/assembler diagnostic entry.
///
/// `file` paths are relative to `project_root` when possible.
#[derive(Debug, Serialize, Clone)]
pub struct DiagnosticEntry {
    pub file: String,
    pub line: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub column: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end_line: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end_column: Option<usize>,
    pub severity: String, // "error" | "warning" | "note" | "help"
    pub message: String,
}

#[derive(Debug, Serialize)]
pub struct BuildResultData {
    pub profile: String,
    pub tool: String,
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
    #[serde(default)]
    pub diagnostics: Vec<DiagnosticEntry>,
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
