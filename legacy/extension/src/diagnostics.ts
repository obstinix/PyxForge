import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';

export interface ParsedDiagnostic {
	file: string;
	line: number;
	column: number;
	severity: vscode.DiagnosticSeverity;
	code?: string;
	message: string;
}

/**
 * Parse build output stdout/stderr into VS Code Diagnostics grouped by file path.
 */
export function parseBuildOutput(
	stdout: string,
	stderr: string,
	projectRoot: string
): Map<string, vscode.Diagnostic[]> {
	const diagnosticsMap = new Map<string, vscode.Diagnostic[]>();
	const allLines = (stdout + '\n' + stderr).split(/\r?\n/);

	// Rust human-readable compiler state machine variables
	let rustSeverity: vscode.DiagnosticSeverity | null = null;
	let rustCode: string | undefined = undefined;
	let rustMessage: string | null = null;

	for (const line of allLines) {
		const trimmed = line.trim();
		if (!trimmed) {
			continue;
		}

		// 1. Try Parsing Cargo/Rust JSON Diagnostic
		if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
			try {
				const obj = JSON.parse(trimmed);
				if (obj && obj.reason === 'compiler-message' && obj.message) {
					const msgObj = obj.message;
					const message = msgObj.message || 'Compiler error';
					const level = (msgObj.level || 'error').toLowerCase();
					const code = msgObj.code?.code;

					let severity = vscode.DiagnosticSeverity.Error;
					if (level.includes('warning')) {
						severity = vscode.DiagnosticSeverity.Warning;
					} else if (level.includes('note')) {
						severity = vscode.DiagnosticSeverity.Information;
					} else if (level.includes('help') || level.includes('hint')) {
						severity = vscode.DiagnosticSeverity.Hint;
					}

					const primarySpan = msgObj.spans?.find((s: any) => s.is_primary) || msgObj.spans?.[0];
					if (primarySpan) {
						const relativeFile = primarySpan.file_name;
						const absoluteFile = path.isAbsolute(relativeFile)
							? relativeFile
							: path.join(projectRoot, relativeFile);

						const startLine = Math.max(0, (primarySpan.line_start || 1) - 1);
						const endLine = Math.max(0, (primarySpan.line_end || 1) - 1);
						const startCol = Math.max(0, (primarySpan.column_start || 1) - 1);
						const endCol = Math.max(0, (primarySpan.column_end || 1) - 1);

						const range = new vscode.Range(startLine, startCol, endLine, endCol);
						const diagnostic = new vscode.Diagnostic(range, message, severity);
						if (code) {
							diagnostic.code = code;
						}
						diagnostic.source = 'pyxforge-rustc';

						addDiagnostic(diagnosticsMap, absoluteFile, diagnostic);
					}
					continue; // Parsed JSON successfully, skip other parsers
				}
			} catch {
				// Not valid JSON or failed to parse, fallback to line regexes
			}
		}

		// 2. Rust human-readable compiler state machine parser
		// Match: error[E0425]: cannot find value `x` in this scope
		const rustHeaderMatch = line.match(/^(error|warning|note|help)(?:\[(E\d+)\])?: (.*)$/);
		if (rustHeaderMatch) {
			const rawSev = rustHeaderMatch[1];
			rustCode = rustHeaderMatch[2];
			rustMessage = rustHeaderMatch[3];

			if (rawSev === 'error') {
				rustSeverity = vscode.DiagnosticSeverity.Error;
			} else if (rawSev === 'warning') {
				rustSeverity = vscode.DiagnosticSeverity.Warning;
			} else if (rawSev === 'note') {
				rustSeverity = vscode.DiagnosticSeverity.Information;
			} else {
				rustSeverity = vscode.DiagnosticSeverity.Hint;
			}
			continue;
		}

		// Match: --> src/main.rs:10:15
		if (rustSeverity !== null && rustMessage !== null) {
			const rustLocMatch = line.match(/^\s*-->\s*((?:[a-zA-Z]:)?[^:\n]+):(\d+):(\d+)/);
			if (rustLocMatch) {
				const relativeFile = rustLocMatch[1];
				const absoluteFile = path.isAbsolute(relativeFile)
					? relativeFile
					: path.join(projectRoot, relativeFile);
				const lNum = Math.max(0, parseInt(rustLocMatch[2], 10) - 1);
				const cNum = Math.max(0, parseInt(rustLocMatch[3], 10) - 1);

				const range = new vscode.Range(lNum, cNum, lNum, cNum + 1);
				const diagnostic = new vscode.Diagnostic(range, rustMessage, rustSeverity);
				if (rustCode) {
					diagnostic.code = rustCode;
				}
				diagnostic.source = 'pyxforge-rustc';

				addDiagnostic(diagnosticsMap, absoluteFile, diagnostic);

				// Reset state to avoid reusing the same message for sibling references unless desired
				rustSeverity = null;
				rustCode = undefined;
				rustMessage = null;
				continue;
			}
		}

		// 3. GCC / Clang standard diagnostics
		// Matches: C:\path\to\file.c:10:5: error: message
		// Matches: src/main.c:12: warning: message
		const gccMatch = line.match(/^((?:[a-zA-Z]:)?[^:\n]+):(\d+):(?:(\d+):)?\s*(error|warning|note|info|hint):\s*(.*)$/i);
		if (gccMatch) {
			const file = gccMatch[1];
			const absoluteFile = path.isAbsolute(file) ? file : path.join(projectRoot, file);
			const lineNum = Math.max(0, parseInt(gccMatch[2], 10) - 1);
			const colNum = gccMatch[3] ? Math.max(0, parseInt(gccMatch[3], 10) - 1) : 0;
			const severityStr = gccMatch[4].toLowerCase();
			const message = gccMatch[5];

			let severity = vscode.DiagnosticSeverity.Error;
			if (severityStr.includes('warning')) {
				severity = vscode.DiagnosticSeverity.Warning;
			} else if (severityStr.includes('note') || severityStr.includes('info')) {
				severity = vscode.DiagnosticSeverity.Information;
			} else if (severityStr.includes('hint')) {
				severity = vscode.DiagnosticSeverity.Hint;
			}

			const range = new vscode.Range(lineNum, colNum, lineNum, colNum + 1);
			const diagnostic = new vscode.Diagnostic(range, message, severity);
			diagnostic.source = 'pyxforge-gcc-clang';

			addDiagnostic(diagnosticsMap, absoluteFile, diagnostic);
			continue;
		}

		// 4. MSVC cl.exe / link.exe diagnostics
		// Matches: C:\path\to\file.cpp(10) : error C2143: message
		// Matches: C:\path\to\file.cpp : error LNK2019: message
		const msvcMatch = line.match(/^((?:[a-zA-Z]:)?[^(:\n]+)(?:\((\d+)\))?\s*:\s*(error|warning|note|info)\s+([a-zA-Z0-9]+)\s*:\s*(.*)$/i);
		if (msvcMatch) {
			const file = msvcMatch[1].trim();
			const absoluteFile = path.isAbsolute(file) ? file : path.join(projectRoot, file);
			const lineNum = msvcMatch[2] ? Math.max(0, parseInt(msvcMatch[2], 10) - 1) : 0;
			const severityStr = msvcMatch[3].toLowerCase();
			const code = msvcMatch[4];
			const message = msvcMatch[5];

			let severity = vscode.DiagnosticSeverity.Error;
			if (severityStr.includes('warning')) {
				severity = vscode.DiagnosticSeverity.Warning;
			} else if (severityStr.includes('note') || severityStr.includes('info')) {
				severity = vscode.DiagnosticSeverity.Information;
			}

			const range = new vscode.Range(lineNum, 0, lineNum, 1);
			const diagnostic = new vscode.Diagnostic(range, message, severity);
			diagnostic.code = code;
			diagnostic.source = 'pyxforge-msvc';

			addDiagnostic(diagnosticsMap, absoluteFile, diagnostic);
			continue;
		}

		// 5. GNU ld / Linker errors
		// Matches: filename:line: undefined reference to 'symbol'
		const ldLineMatch = line.match(/^((?:[a-zA-Z]:)?[^:\n]+):(\d+):\s*(undefined reference to.*|multiple definition of.*|ld returned \d+ exit status.*)$/i);
		if (ldLineMatch) {
			const file = ldLineMatch[1];
			const absoluteFile = path.isAbsolute(file) ? file : path.join(projectRoot, file);
			const lineNum = Math.max(0, parseInt(ldLineMatch[2], 10) - 1);
			const message = ldLineMatch[3];

			const range = new vscode.Range(lineNum, 0, lineNum, 1);
			const diagnostic = new vscode.Diagnostic(range, message, vscode.DiagnosticSeverity.Error);
			diagnostic.source = 'pyxforge-ld';

			addDiagnostic(diagnosticsMap, absoluteFile, diagnostic);
			continue;
		}

		// Matches: filename: undefined reference to 'symbol'
		const ldMatch = line.match(/^((?:[a-zA-Z]:)?[^:\n]+):\s*(undefined reference to.*|multiple definition of.*)$/i);
		if (ldMatch) {
			const file = ldMatch[1];
			const absoluteFile = path.isAbsolute(file) ? file : path.join(projectRoot, file);
			const message = ldMatch[2];

			const range = new vscode.Range(0, 0, 0, 1);
			const diagnostic = new vscode.Diagnostic(range, message, vscode.DiagnosticSeverity.Error);
			diagnostic.source = 'pyxforge-ld';

			addDiagnostic(diagnosticsMap, absoluteFile, diagnostic);
			continue;
		}
	}

	return diagnosticsMap;
}

function addDiagnostic(
	diagnosticsMap: Map<string, vscode.Diagnostic[]>,
	filePath: string,
	diagnostic: vscode.Diagnostic
) {
	// Normalize file path for VS Code key indexing
	let normalizedPath = filePath;
	try {
		normalizedPath = vscode.Uri.file(filePath).fsPath;
	} catch {
		// fallback
	}

	// Verify that the file actually exists on disk before publishing
	if (!fs.existsSync(normalizedPath)) {
		return;
	}

	const list = diagnosticsMap.get(normalizedPath) || [];
	// Avoid exact duplicates
	const isDuplicate = list.some(
		(d) =>
			d.message === diagnostic.message &&
			d.range.start.line === diagnostic.range.start.line &&
			d.range.start.character === diagnostic.range.start.character &&
			d.severity === diagnostic.severity
	);

	if (!isDuplicate) {
		list.push(diagnostic);
		diagnosticsMap.set(normalizedPath, list);
	}
}
