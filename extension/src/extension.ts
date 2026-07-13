import * as vscode from 'vscode';
import * as path from 'path';
import * as cp from 'child_process';

// ---------------------------------------------------------------------------
// Core binary helper
// ---------------------------------------------------------------------------

function getCoreBinaryPath(context: vscode.ExtensionContext): string {
	// TODO(phase-N): The following path is hardcoded for development and must
	// be replaced by production packaging logic later.
	const binaryName = process.platform === 'win32' ? 'pyxforge-core.exe' : 'pyxforge-core';
	return path.join(context.extensionPath, '..', 'core', 'target', 'x86_64-pc-windows-gnu', 'debug', binaryName);
}

/**
 * Send a JSON request to the core binary and return the parsed response.
 */
function callCore(
	coreBinaryPath: string,
	request: Record<string, unknown>
): Promise<{ status: string; [key: string]: unknown }> {
	return new Promise((resolve, reject) => {
		let stdoutData = '';
		let stderrData = '';

		try {
			const child = cp.spawn(coreBinaryPath, [], { stdio: ['pipe', 'pipe', 'pipe'] });

			child.stdout.on('data', (data) => {
				stdoutData += data.toString();
			});

			child.stderr.on('data', (data) => {
				stderrData += data.toString();
			});

			child.on('error', (err) => {
				reject(new Error(`Failed to spawn PyxForge Core binary at ${coreBinaryPath}: ${err.message}`));
			});

			child.on('close', (code) => {
				try {
					const trimmed = stdoutData.trim();
					if (trimmed) {
						const response = JSON.parse(trimmed);
						if (code === 0) {
							resolve(response);
						} else {
							reject(new Error(response.message || `Core exited with code ${code}`));
						}
					} else {
						reject(new Error(`Core exited with code ${code}. Stderr: ${stderrData || 'none'}`));
					}
				} catch (parseErr: any) {
					reject(new Error(`Failed to parse Core response: ${parseErr.message}. Raw: ${stdoutData}`));
				}
			});

			child.stdin.write(JSON.stringify(request) + '\n');
			child.stdin.end();
		} catch (err: any) {
			reject(new Error(`Unexpected exception: ${err.message}`));
		}
	});
}

// ---------------------------------------------------------------------------
// Output channel & state
// ---------------------------------------------------------------------------

let outputChannel: vscode.OutputChannel;
let activeQemuPid: number | null = null;
let activeQemuPort: number = 0;
let qemuStatusBarItem: vscode.StatusBarItem | null = null;

function getOutputChannel(): vscode.OutputChannel {
	if (!outputChannel) {
		outputChannel = vscode.window.createOutputChannel('PyxForge');
	}
	return outputChannel;
}

function updateQemuStatusBar() {
	if (!qemuStatusBarItem) {
		qemuStatusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
	}
	if (activeQemuPid !== null) {
		qemuStatusBarItem.text = `$(play) QEMU: Running (PID ${activeQemuPid})`;
		qemuStatusBarItem.tooltip = `QEMU is running. GDB Port: ${activeQemuPort}. Click to stop.`;
		qemuStatusBarItem.command = 'pyxforge.stop';
		qemuStatusBarItem.show();
	} else {
		qemuStatusBarItem.hide();
	}
}

// ---------------------------------------------------------------------------
// Activation
// ---------------------------------------------------------------------------

export function activate(context: vscode.ExtensionContext) {
	console.log('PyxForge extension is now active!');

	const coreBinaryPath = getCoreBinaryPath(context);

	// -- ping ---------------------------------------------------------------
	const pingDisposable = vscode.commands.registerCommand('pyxforge.ping', async () => {
		try {
			const response = await callCore(coreBinaryPath, { cmd: 'ping' });
			vscode.window.showInformationMessage(
				`PyxForge Core Ping Successful! Status: ${response.status}, Version: ${response.version}, Message: ${response.message}`
			);
		} catch (err: any) {
			vscode.window.showErrorMessage(`PyxForge Ping Failed: ${err.message}`);
		}
	});

	// -- build --------------------------------------------------------------
	const buildDisposable = vscode.commands.registerCommand('pyxforge.build', async () => {
		const workspaceFolders = vscode.workspace.workspaceFolders;
		if (!workspaceFolders || workspaceFolders.length === 0) {
			vscode.window.showErrorMessage('PyxForge: No workspace folder is open.');
			return;
		}

		const projectRoot = workspaceFolders[0].uri.fsPath;
		const out = getOutputChannel();

		// First, list available profiles so the user can pick one.
		try {
			const listResponse = await callCore(coreBinaryPath, {
				cmd: 'list-profiles',
				project_root: projectRoot,
			});

			const profiles: { name: string; tool: string; description?: string }[] =
				(listResponse.data as any)?.profiles || [];

			if (profiles.length === 0) {
				vscode.window.showWarningMessage('PyxForge: No build profiles found in pyxforge.toml.');
				return;
			}

			// Show a quick pick for the user to select a profile.
			const items: vscode.QuickPickItem[] = profiles.map((p) => ({
				label: p.name,
				description: p.tool,
				detail: p.description,
			}));

			const selected = await vscode.window.showQuickPick(items, {
				placeHolder: 'Select a build profile',
				title: 'PyxForge: Build',
			});

			if (!selected) {
				return; // User cancelled.
			}

			// Execute the build.
			out.show(true);
			out.appendLine(`[PyxForge] Building profile: ${selected.label}`);
			out.appendLine('---');

			const buildResponse = await callCore(coreBinaryPath, {
				cmd: 'build',
				profile: selected.label,
				project_root: projectRoot,
			});

			const buildData = buildResponse.data as any;
			if (buildData?.stdout) {
				out.appendLine(buildData.stdout);
			}
			if (buildData?.stderr) {
				out.appendLine(buildData.stderr);
			}
			out.appendLine(`[PyxForge] Build '${selected.label}' succeeded (exit code ${buildData?.exit_code ?? 0})`);
			vscode.window.showInformationMessage(`PyxForge: Build '${selected.label}' succeeded.`);

		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Build failed: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge Build Failed: ${err.message}`);
		}
	});

	// -- launch QEMU --------------------------------------------------------
	const launchDisposable = vscode.commands.registerCommand('pyxforge.launch', async () => {
		const workspaceFolders = vscode.workspace.workspaceFolders;
		if (!workspaceFolders || workspaceFolders.length === 0) {
			vscode.window.showErrorMessage('PyxForge: No workspace folder is open.');
			return;
		}

		if (activeQemuPid !== null) {
			const choice = await vscode.window.showWarningMessage(
				`QEMU is already running (PID ${activeQemuPid}). Do you want to restart it?`,
				'Yes',
				'No'
			);
			if (choice === 'Yes') {
				await vscode.commands.executeCommand('pyxforge.stop');
			} else {
				return;
			}
		}

		const projectRoot = workspaceFolders[0].uri.fsPath;
		const out = getOutputChannel();

		try {
			out.show(true);
			out.appendLine('[PyxForge] Launching QEMU...');

			const response = await callCore(coreBinaryPath, {
				cmd: 'launch',
				project_root: projectRoot,
				debug: true, // Default to true per PRD
			});

			const launchData = response.data as any;
			activeQemuPid = launchData.pid;
			activeQemuPort = launchData.port;

			out.appendLine(`[PyxForge] QEMU launched successfully (PID ${activeQemuPid})`);
			if (launchData.args_used) {
				out.appendLine(`[PyxForge] Arguments: ${launchData.args_used.join(' ')}`);
			}
			if (activeQemuPort > 0) {
				out.appendLine(`[PyxForge] GDB remote debugging enabled on port ${activeQemuPort}`);
			}

			updateQemuStatusBar();
			vscode.window.showInformationMessage(`PyxForge: QEMU launched successfully (PID ${activeQemuPid}).`);

		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Launch failed: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge Launch Failed: ${err.message}`);
		}
	});

	// -- stop QEMU ----------------------------------------------------------
	const stopDisposable = vscode.commands.registerCommand('pyxforge.stop', async () => {
		if (activeQemuPid === null) {
			vscode.window.showWarningMessage('PyxForge: No QEMU instance is running.');
			return;
		}

		const out = getOutputChannel();
		const pidToStop = activeQemuPid;

		try {
			out.show(true);
			out.appendLine(`[PyxForge] Stopping QEMU (PID ${pidToStop})...`);

			await callCore(coreBinaryPath, {
				cmd: 'stop',
				pid: pidToStop,
			});

			out.appendLine(`[PyxForge] QEMU stopped (PID ${pidToStop})`);
			vscode.window.showInformationMessage(`PyxForge: QEMU stopped (PID ${pidToStop}).`);
		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Failed to stop QEMU: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge: Failed to stop QEMU: ${err.message}`);
		} finally {
			activeQemuPid = null;
			activeQemuPort = 0;
			updateQemuStatusBar();
		}
	});

	context.subscriptions.push(pingDisposable, buildDisposable, launchDisposable, stopDisposable);
}

export function deactivate() {
	if (activeQemuPid !== null) {
		try {
			if (process.platform === 'win32') {
				cp.execSync(`taskkill /F /PID ${activeQemuPid}`);
			} else {
				cp.execSync(`kill -9 ${activeQemuPid}`);
			}
		} catch (e) {
			// ignore
		}
	}
	if (outputChannel) {
		outputChannel.dispose();
	}
	if (qemuStatusBarItem) {
		qemuStatusBarItem.dispose();
	}
}
