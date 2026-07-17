import * as assert from 'assert';
import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { parseBuildOutput } from '../diagnostics';
import { getPresets, extractProjectName } from '../presets';

/**
 * Helper utility to parse stdout and stderr streams from composite build log formats.
 */
function testExtractStdoutStderr(msg: string): { stdout: string; stderr: string } {
	const stdoutMarker = '--- stdout ---\n';
	const stderrMarker = '--- stderr ---\n';
	const stdoutIndex = msg.indexOf(stdoutMarker);
	const stderrIndex = msg.indexOf(stderrMarker);

	let stdout = '';
	let stderr = '';

	if (stdoutIndex !== -1 && stderrIndex !== -1) {
		stdout = msg.substring(stdoutIndex + stdoutMarker.length, stderrIndex).trim();
		stderr = msg.substring(stderrIndex + stderrMarker.length).trim();
	} else if (stdoutIndex !== -1) {
		stdout = msg.substring(stdoutIndex + stdoutMarker.length).trim();
	} else if (stderrIndex !== -1) {
		stderr = msg.substring(stderrIndex + stderrMarker.length).trim();
	} else {
		stderr = msg;
	}

	return { stdout, stderr };
}

suite('PyxForge Extension Test Suite', () => {

	suiteSetup(async () => {
		// Wait for the extension to activate
		const ext = vscode.extensions.getExtension('obstinix.pyxforge');
		if (ext) {
			await ext.activate();
		}
	});

	test('PyxForge command registrations', async () => {
		const commands = await vscode.commands.getCommands(true);
		const expectedCommands = [
			'pyxforge.ping',
			'pyxforge.build',
			'pyxforge.launch',
			'pyxforge.launchNoDebug',
			'pyxforge.stop',
			'pyxforge.debug',
			'pyxforge.showInspector',
			'pyxforge.init',
			'pyxforge.openHex',
			'pyxforge.explainAsm',
			'pyxforge.explainBuild',
			'pyxforge.selectPreset'
		];

		for (const cmd of expectedCommands) {
			assert.ok(commands.includes(cmd), `Command ${cmd} should be registered`);
		}
	});

	test('Build output parsing - GCC/Clang format', () => {
		// Mock temporary file in the workspace to make sure fs.existsSync checks pass
		const tempFile = path.join(__dirname, 'mock_file.c');
		fs.writeFileSync(tempFile, 'int main() {}', 'utf8');

		try {
			const stdout = '';
			const stderr = `${tempFile}:10:5: error: expected ';' before '}' token\n${tempFile}:15: warning: unused variable 'x'`;
			
			const diags = parseBuildOutput(stdout, stderr, __dirname);
			
			assert.ok(diags.has(tempFile), 'Should parse diagnostics for the mock file');
			const fileDiags = diags.get(tempFile)!;
			assert.strictEqual(fileDiags.length, 2, 'Should have 2 diagnostics');

			assert.strictEqual(fileDiags[0].severity, vscode.DiagnosticSeverity.Error);
			assert.strictEqual(fileDiags[0].range.start.line, 9);
			assert.strictEqual(fileDiags[0].range.start.character, 4);
			assert.strictEqual(fileDiags[0].message, "expected ';' before '}' token");

			assert.strictEqual(fileDiags[1].severity, vscode.DiagnosticSeverity.Warning);
			assert.strictEqual(fileDiags[1].range.start.line, 14);
			assert.strictEqual(fileDiags[1].message, "unused variable 'x'");
		} finally {
			if (fs.existsSync(tempFile)) {
				fs.unlinkSync(tempFile);
			}
		}
	});

	test('Build output parsing - MSVC format', () => {
		const tempFile = path.join(__dirname, 'mock_file.cpp');
		fs.writeFileSync(tempFile, 'int main() {}', 'utf8');

		try {
			const stdout = '';
			const stderr = `${tempFile}(12) : error C2143: syntax error : missing ';' before '}'\n${tempFile} : warning C4101: 'x' : unreferenced local variable`;

			const diags = parseBuildOutput(stdout, stderr, __dirname);

			assert.ok(diags.has(tempFile));
			const fileDiags = diags.get(tempFile)!;
			assert.strictEqual(fileDiags.length, 2);

			assert.strictEqual(fileDiags[0].severity, vscode.DiagnosticSeverity.Error);
			assert.strictEqual(fileDiags[0].range.start.line, 11);
			assert.strictEqual(fileDiags[0].code, 'C2143');

			assert.strictEqual(fileDiags[1].severity, vscode.DiagnosticSeverity.Warning);
			assert.strictEqual(fileDiags[1].range.start.line, 0);
			assert.strictEqual(fileDiags[1].code, 'C4101');
		} finally {
			if (fs.existsSync(tempFile)) {
				fs.unlinkSync(tempFile);
			}
		}
	});

	test('Build output parsing - Rustc human-readable format', () => {
		const tempFile = path.join(__dirname, 'mock_file.rs');
		fs.writeFileSync(tempFile, 'fn main() {}', 'utf8');

		try {
			const stdout = '';
			const stderr = `error[E0308]: mismatched types\n  --> ${tempFile}:5:10\n   |\n5 | let x = 5;\n   |         ^ expected u32`;

			const diags = parseBuildOutput(stdout, stderr, __dirname);

			assert.ok(diags.has(tempFile));
			const fileDiags = diags.get(tempFile)!;
			assert.strictEqual(fileDiags.length, 1);
			assert.strictEqual(fileDiags[0].severity, vscode.DiagnosticSeverity.Error);
			assert.strictEqual(fileDiags[0].range.start.line, 4);
			assert.strictEqual(fileDiags[0].range.start.character, 9);
			assert.strictEqual(fileDiags[0].code, 'E0308');
			assert.strictEqual(fileDiags[0].message, 'mismatched types');
		} finally {
			if (fs.existsSync(tempFile)) {
				fs.unlinkSync(tempFile);
			}
		}
	});

	test('Build output parsing - Cargo JSON format', () => {
		const tempFile = path.join(__dirname, 'mock_file.rs');
		fs.writeFileSync(tempFile, 'fn main() {}', 'utf8');

		try {
			const jsonLine = JSON.stringify({
				reason: 'compiler-message',
				package_id: 'test 0.1.0',
				target: { kind: ['bin'], name: 'test' },
				message: {
					message: 'mismatched types',
					code: { code: 'E0308', explanation: 'type mismatch' },
					level: 'error',
					spans: [{
						file_name: tempFile,
						line_start: 5,
						line_end: 5,
						column_start: 10,
						column_end: 15,
						is_primary: true
					}]
				}
			});

			const diags = parseBuildOutput('', jsonLine, __dirname);

			assert.ok(diags.has(tempFile));
			const fileDiags = diags.get(tempFile)!;
			assert.strictEqual(fileDiags.length, 1);
			assert.strictEqual(fileDiags[0].severity, vscode.DiagnosticSeverity.Error);
			assert.strictEqual(fileDiags[0].range.start.line, 4);
			assert.strictEqual(fileDiags[0].range.start.character, 9);
			assert.strictEqual(fileDiags[0].range.end.line, 4);
			assert.strictEqual(fileDiags[0].range.end.character, 14);
			assert.strictEqual(fileDiags[0].code, 'E0308');
			assert.strictEqual(fileDiags[0].message, 'mismatched types');
		} finally {
			if (fs.existsSync(tempFile)) {
				fs.unlinkSync(tempFile);
			}
		}
	});

	test('Build output parsing - GNU Linker format', () => {
		const tempFile = path.join(__dirname, 'main.o');
		fs.writeFileSync(tempFile, '', 'utf8');

		try {
			const stderr = `${tempFile}:10: undefined reference to 'kernel_main'\n${tempFile}: multiple definition of 'foo'`;
			const diags = parseBuildOutput('', stderr, __dirname);

			assert.ok(diags.has(tempFile));
			const fileDiags = diags.get(tempFile)!;
			assert.strictEqual(fileDiags.length, 2);

			assert.strictEqual(fileDiags[0].severity, vscode.DiagnosticSeverity.Error);
			assert.strictEqual(fileDiags[0].range.start.line, 9);
			assert.ok(fileDiags[0].message.includes('undefined reference'));

			assert.strictEqual(fileDiags[1].severity, vscode.DiagnosticSeverity.Error);
			assert.strictEqual(fileDiags[1].range.start.line, 0);
			assert.ok(fileDiags[1].message.includes('multiple definition'));
		} finally {
			if (fs.existsSync(tempFile)) {
				fs.unlinkSync(tempFile);
			}
		}
	});

	test('Presets parsing & project name extraction', () => {
		const toml = `
[project]
name = "my-special-os"
description = "A custom operating system"
`;
		const projName = extractProjectName(toml, 'fallback');
		assert.strictEqual(projName, 'my-special-os');

		// Missing project name fallback
		const badToml = `
[settings]
key = "value"
`;
		const fallbackName = extractProjectName(badToml, 'my-fallback');
		assert.strictEqual(fallbackName, 'my-fallback');

		// Validate preset templates exist and are valid TOML format
		for (const p of getPresets()) {
			assert.ok(p.name);
			assert.ok(p.description);
			const generated = p.tomlTemplate('test-project');
			assert.ok(generated.includes('name = "test-project"'), `Preset ${p.name} template should embed project name`);
		}
	});

	test('Extracting stdout/stderr from core build error logs', () => {
		const msg = `Build 'bootloader' failed (exit code 1)
--- stdout ---
Assembly compilation started.
Done.
--- stderr ---
nasm: error: file boot.asm not found`;

		const { stdout, stderr } = testExtractStdoutStderr(msg);
		assert.strictEqual(stdout, 'Assembly compilation started.\nDone.');
		assert.strictEqual(stderr, 'nasm: error: file boot.asm not found');

		// Empty/Fallback case
		const fallbackMsg = 'Failed to execute command: executable not found';
		const res = testExtractStdoutStderr(fallbackMsg);
		assert.strictEqual(res.stdout, '');
		assert.strictEqual(res.stderr, fallbackMsg);
	});

	test('Theme configurations validation', () => {
		const config = vscode.workspace.getConfiguration('pyxforge');
		const activeTheme = config.get<string>('theme');
		assert.ok(activeTheme === undefined || typeof activeTheme === 'string', 'Theme setting should be a valid string or undefined');
	});
});
