//! Core-side diagnostic parsers for compiler/assembler output.
//!
//! Supports two output families:
//! 1. GNU-style single-line: `file:line:col?: error|warning: message` (NASM, GCC, Clang, ld)
//! 2. Cargo/rustc JSON: newline-delimited JSON with `reason: "compiler-message"` objects

use crate::protocol::DiagnosticEntry;

// ---------------------------------------------------------------------------
// GNU-style parser
// ---------------------------------------------------------------------------

/// Parse GNU-style diagnostics from combined stdout + stderr text.
///
/// Matches lines of the form:
///   file.asm:10: error: instruction expected
///   src/main.c:12:5: warning: implicit declaration
///
/// Unrecognized lines are silently skipped — this is additive, never errors.
pub fn parse_gnu_diagnostics(output: &str) -> Vec<DiagnosticEntry> {
    let mut entries = Vec::new();

    for line in output.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }

        // Try to match: file:line:col: severity: message
        // or:           file:line: severity: message
        if let Some(entry) = try_parse_gnu_line(trimmed) {
            entries.push(entry);
        }
    }

    entries
}

fn try_parse_gnu_line(line: &str) -> Option<DiagnosticEntry> {
    // Strategy: find the pattern file:digits: or file:digits:digits:
    // then look for a severity keyword after the location.
    //
    // We scan for `:digits:` patterns manually to avoid pulling in regex.

    // Skip lines that are clearly not diagnostics
    if line.starts_with(' ') || line.starts_with('\t') {
        return None;
    }

    // Find the first `:digit` pattern. Handle Windows drive letters (e.g. C:\foo)
    // by starting the search after position 2 if the line starts with a drive letter.
    let search_start = if line.len() >= 3
        && line.as_bytes()[0].is_ascii_alphabetic()
        && line.as_bytes()[1] == b':'
        && (line.as_bytes()[2] == b'\\' || line.as_bytes()[2] == b'/')
    {
        3
    } else {
        0
    };

    // Find `:digits:` after the file path portion
    let bytes = line.as_bytes();
    let mut pos = search_start;
    let mut colon1 = None;

    while pos < bytes.len() {
        if bytes[pos] == b':' {
            // Check if digits follow
            let digit_start = pos + 1;
            let mut digit_end = digit_start;
            while digit_end < bytes.len() && bytes[digit_end].is_ascii_digit() {
                digit_end += 1;
            }
            if digit_end > digit_start {
                colon1 = Some(pos);
                break;
            }
        }
        pos += 1;
    }

    let colon1 = colon1?;
    let file = &line[..colon1];

    // Parse the line number
    let after_file = &line[colon1 + 1..];
    let (line_num, rest) = parse_number_prefix(after_file)?;

    // Check for optional column: `:digits:`
    let (column, rest) = if let Some(after_colon) = rest.strip_prefix(':') {
        if let Some((col, rest2)) = parse_number_prefix(after_colon) {
            (Some(col), rest2)
        } else {
            (None, rest)
        }
    } else {
        (None, rest)
    };

    // Now expect `: severity: message` or `severity: message`
    let rest = rest.trim_start_matches(':').trim();

    // Look for severity keyword at the start
    let severity;
    let message;

    let rest_lower = rest.to_lowercase();
    if rest_lower.starts_with("error:") || rest_lower.starts_with("error ") {
        severity = "error";
        message = rest[if rest_lower.starts_with("error:") {
            6
        } else {
            5
        }..]
            .trim();
    } else if rest_lower.starts_with("warning:") || rest_lower.starts_with("warning ") {
        severity = "warning";
        message = rest[if rest_lower.starts_with("warning:") {
            8
        } else {
            7
        }..]
            .trim();
    } else if rest_lower.starts_with("note:") {
        severity = "note";
        message = rest[5..].trim();
    } else if rest_lower.starts_with("fatal error:") {
        severity = "error";
        message = rest[12..].trim();
    } else {
        return None;
    }

    if message.is_empty() {
        return None;
    }

    Some(DiagnosticEntry {
        file: file.to_string(),
        line: line_num,
        column,
        end_line: None,
        end_column: None,
        severity: severity.to_string(),
        message: message.to_string(),
    })
}

/// Parse a prefix of decimal digits, returning the number and the remaining string.
fn parse_number_prefix(s: &str) -> Option<(usize, &str)> {
    let end = s
        .as_bytes()
        .iter()
        .position(|b| !b.is_ascii_digit())
        .unwrap_or(s.len());
    if end == 0 {
        return None;
    }
    let num: usize = s[..end].parse().ok()?;
    Some((num, &s[end..]))
}

// ---------------------------------------------------------------------------
// Cargo/rustc JSON parser
// ---------------------------------------------------------------------------

/// Parse cargo/rustc `--message-format=json` output into diagnostic entries.
///
/// Each line of the output is expected to be a complete JSON object.
/// We look for `{"reason":"compiler-message", "message": {...}}` objects
/// and extract span information from `message.spans`.
pub fn parse_cargo_json_diagnostics(output: &str) -> Vec<DiagnosticEntry> {
    let mut entries = Vec::new();

    for line in output.lines() {
        let trimmed = line.trim();
        if !trimmed.starts_with('{') {
            continue;
        }

        let parsed: Result<serde_json::Value, _> = serde_json::from_str(trimmed);
        let obj = match parsed {
            Ok(v) => v,
            Err(_) => continue,
        };

        if obj.get("reason").and_then(|v| v.as_str()) != Some("compiler-message") {
            continue;
        }

        let msg = match obj.get("message") {
            Some(m) => m,
            None => continue,
        };

        let message_text = msg
            .get("message")
            .and_then(|v| v.as_str())
            .unwrap_or("Compiler error");

        let level = msg.get("level").and_then(|v| v.as_str()).unwrap_or("error");

        let severity = match level {
            "warning" => "warning",
            "note" => "note",
            "help" => "help",
            _ => "error",
        };

        // Find the primary span (or first span)
        let spans = match msg.get("spans").and_then(|v| v.as_array()) {
            Some(s) if !s.is_empty() => s,
            _ => continue,
        };

        let primary = spans
            .iter()
            .find(|s| s.get("is_primary").and_then(|v| v.as_bool()) == Some(true))
            .or_else(|| spans.first());

        let span = match primary {
            Some(s) => s,
            None => continue,
        };

        let file_name = match span.get("file_name").and_then(|v| v.as_str()) {
            Some(f) => f,
            None => continue,
        };

        let line_start = span.get("line_start").and_then(|v| v.as_u64()).unwrap_or(1) as usize;
        let col_start = span
            .get("column_start")
            .and_then(|v| v.as_u64())
            .map(|v| v as usize);
        let line_end = span
            .get("line_end")
            .and_then(|v| v.as_u64())
            .map(|v| v as usize);
        let col_end = span
            .get("column_end")
            .and_then(|v| v.as_u64())
            .map(|v| v as usize);

        entries.push(DiagnosticEntry {
            file: file_name.to_string(),
            line: line_start,
            column: col_start,
            end_line: line_end,
            end_column: col_end,
            severity: severity.to_string(),
            message: message_text.to_string(),
        });
    }

    entries
}

/// Attempt to parse diagnostics from build output, trying cargo JSON first,
/// then falling back to GNU-style parsing.
pub fn parse_build_diagnostics(stdout: &str, stderr: &str) -> Vec<DiagnosticEntry> {
    let combined = format!("{}\n{}", stdout, stderr);

    // Try cargo JSON first — if we find any, those are authoritative
    let json_diags = parse_cargo_json_diagnostics(&combined);
    if !json_diags.is_empty() {
        return json_diags;
    }

    // Fall back to GNU-style
    parse_gnu_diagnostics(&combined)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    // -- GNU-style parser tests --

    #[test]
    fn test_gnu_nasm_error() {
        let output = "boot.asm:10: error: instruction expected";
        let diags = parse_gnu_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].file, "boot.asm");
        assert_eq!(diags[0].line, 10);
        assert_eq!(diags[0].column, None);
        assert_eq!(diags[0].severity, "error");
        assert_eq!(diags[0].message, "instruction expected");
    }

    #[test]
    fn test_gnu_gcc_warning_with_column() {
        let output = "src/main.c:12:5: warning: implicit declaration of function 'foo'";
        let diags = parse_gnu_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].file, "src/main.c");
        assert_eq!(diags[0].line, 12);
        assert_eq!(diags[0].column, Some(5));
        assert_eq!(diags[0].severity, "warning");
        assert!(diags[0].message.contains("implicit declaration"));
    }

    #[test]
    fn test_gnu_fatal_error() {
        let output = "test.c:1:10: fatal error: nonexistent.h: No such file or directory";
        let diags = parse_gnu_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].severity, "error");
    }

    #[test]
    fn test_gnu_multiple_diagnostics() {
        let output = "\
boot.asm:5: error: comma expected
boot.asm:10: warning: label alone on a line without a colon might be in error
boot.asm:20: error: instruction expected";
        let diags = parse_gnu_diagnostics(output);
        assert_eq!(diags.len(), 3);
        assert_eq!(diags[0].line, 5);
        assert_eq!(diags[1].line, 10);
        assert_eq!(diags[1].severity, "warning");
        assert_eq!(diags[2].line, 20);
    }

    #[test]
    fn test_gnu_ignores_non_diagnostic_lines() {
        let output = "\
Compiling boot.asm...
nasm -f bin boot.asm -o boot.bin
boot.asm:10: error: instruction expected
Build complete.";
        let diags = parse_gnu_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].line, 10);
    }

    #[test]
    fn test_gnu_empty_output() {
        let diags = parse_gnu_diagnostics("");
        assert!(diags.is_empty());
    }

    // -- Cargo JSON parser tests --

    #[test]
    fn test_cargo_json_error() {
        let output = r#"{"reason":"compiler-message","message":{"message":"cannot find value `x` in this scope","code":{"code":"E0425"},"level":"error","spans":[{"file_name":"src/main.rs","byte_start":100,"byte_end":101,"line_start":10,"line_end":10,"column_start":15,"column_end":16,"is_primary":true}]}}"#;
        let diags = parse_cargo_json_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].file, "src/main.rs");
        assert_eq!(diags[0].line, 10);
        assert_eq!(diags[0].column, Some(15));
        assert_eq!(diags[0].end_line, Some(10));
        assert_eq!(diags[0].end_column, Some(16));
        assert_eq!(diags[0].severity, "error");
        assert!(diags[0].message.contains("cannot find value"));
    }

    #[test]
    fn test_cargo_json_warning() {
        let output = r#"{"reason":"compiler-message","message":{"message":"unused variable: `y`","code":{"code":"W0001"},"level":"warning","spans":[{"file_name":"src/lib.rs","byte_start":50,"byte_end":51,"line_start":5,"line_end":5,"column_start":9,"column_end":10,"is_primary":true}]}}"#;
        let diags = parse_cargo_json_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].severity, "warning");
    }

    #[test]
    fn test_cargo_json_skips_non_diagnostic_lines() {
        let output = "\
{\"reason\":\"compiler-artifact\",\"package_id\":\"foo 0.1.0\"}\n\
{\"reason\":\"compiler-message\",\"message\":{\"message\":\"unused import\",\"level\":\"warning\",\"spans\":[{\"file_name\":\"src/main.rs\",\"line_start\":1,\"line_end\":1,\"column_start\":5,\"column_end\":10,\"is_primary\":true}]}}\n\
{\"reason\":\"build-finished\",\"success\":false}";
        let diags = parse_cargo_json_diagnostics(output);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].message, "unused import");
    }

    #[test]
    fn test_cargo_json_empty() {
        let diags = parse_cargo_json_diagnostics("");
        assert!(diags.is_empty());
    }

    // -- Combined parser tests --

    #[test]
    fn test_parse_build_diagnostics_prefers_json() {
        let stdout = r#"{"reason":"compiler-message","message":{"message":"test error","level":"error","spans":[{"file_name":"src/main.rs","line_start":1,"line_end":1,"column_start":1,"column_end":2,"is_primary":true}]}}"#;
        let stderr = "src/main.rs:1:1: error: test error"; // also present as GNU
        let diags = parse_build_diagnostics(stdout, stderr);
        // Should prefer cargo JSON
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].file, "src/main.rs");
    }

    #[test]
    fn test_parse_build_diagnostics_falls_back_to_gnu() {
        let stdout = "";
        let stderr = "boot.asm:10: error: instruction expected";
        let diags = parse_build_diagnostics(stdout, stderr);
        assert_eq!(diags.len(), 1);
        assert_eq!(diags[0].file, "boot.asm");
    }
}
