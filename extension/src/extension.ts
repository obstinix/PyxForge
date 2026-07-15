import * as vscode from 'vscode';
import * as path from 'path';
import * as cp from 'child_process';
import { PyxForgeDebugTrackerFactory } from './debugTracker';
import { PyxForgeInspectorPanel } from './inspectorPanel';
import { PyxForgeHexPanel } from './hexPanel';


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
let statusInterval: NodeJS.Timeout | null = null;

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
		qemuStatusBarItem.tooltip = activeQemuPort > 0
			? `QEMU is running (Debug Mode). GDB Port: ${activeQemuPort}. Click to stop.`
			: `QEMU is running (Normal Mode). Click to stop.`;
		qemuStatusBarItem.command = 'pyxforge.stop';
		qemuStatusBarItem.show();
	} else {
		qemuStatusBarItem.hide();
	}
}

function startQemuStatusPolling(coreBinaryPath: string) {
	if (statusInterval) {
		clearInterval(statusInterval);
	}
	statusInterval = setInterval(async () => {
		if (activeQemuPid === null) {
			stopQemuStatusPolling();
			return;
		}

		try {
			const response = await callCore(coreBinaryPath, {
				cmd: 'qemu-status',
				pid: activeQemuPid,
			});
			const isAlive = (response.data as any)?.alive;
			if (!isAlive) {
				const out = getOutputChannel();
				out.appendLine(`[PyxForge] QEMU process terminated externally (PID ${activeQemuPid})`);
				activeQemuPid = null;
				activeQemuPort = 0;
				updateQemuStatusBar();
				stopQemuStatusPolling();
			}
		} catch (err: any) {
			const out = getOutputChannel();
			out.appendLine(`[PyxForge] Error checking QEMU status: ${err.message}`);
		}
	}, 2000);
}

function stopQemuStatusPolling() {
	if (statusInterval) {
		clearInterval(statusInterval);
		statusInterval = null;
	}
}

// ---------------------------------------------------------------------------
// Activation
// ---------------------------------------------------------------------------

export function activate(context: vscode.ExtensionContext) {
	console.log('PyxForge extension is now active!');

	const coreBinaryPath = getCoreBinaryPath(context);

	// Register Debug Adapter Tracker Factory to capturestopped events on GDB
	const debugTrackerFactory = new PyxForgeDebugTrackerFactory((session) => {
		vscode.commands.executeCommand('pyxforge.refreshInspector');
	});
	const trackerDisposable = vscode.debug.registerDebugAdapterTrackerFactory('gdb', debugTrackerFactory);

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

	// -- launch QEMU (Debug Mode) -------------------------------------------
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
			out.appendLine('[PyxForge] Launching QEMU (Debug Mode)...');

			const response = await callCore(coreBinaryPath, {
				cmd: 'launch',
				project_root: projectRoot,
				debug: true,
			});

			const launchData = response.data as any;
			activeQemuPid = launchData.pid;
			activeQemuPort = launchData.port;

			out.appendLine(`[PyxForge] QEMU launched successfully in Debug Mode (PID ${activeQemuPid})`);
			if (launchData.args_used) {
				out.appendLine(`[PyxForge] Arguments: ${launchData.args_used.join(' ')}`);
			}
			if (activeQemuPort > 0) {
				out.appendLine(`[PyxForge] GDB remote debugging enabled on port ${activeQemuPort}`);
			}

			updateQemuStatusBar();
			startQemuStatusPolling(coreBinaryPath);
			vscode.window.showInformationMessage(`PyxForge: QEMU launched in Debug Mode (PID ${activeQemuPid}).`);

		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Launch failed: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge Launch Failed: ${err.message}`);
		}
	});

	// -- launch QEMU (No Debug) ---------------------------------------------
	const launchNoDebugDisposable = vscode.commands.registerCommand('pyxforge.launchNoDebug', async () => {
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
			out.appendLine('[PyxForge] Launching QEMU (No Debug)...');

			const response = await callCore(coreBinaryPath, {
				cmd: 'launch',
				project_root: projectRoot,
				debug: false,
			});

			const launchData = response.data as any;
			activeQemuPid = launchData.pid;
			activeQemuPort = 0; // No GDB port

			out.appendLine(`[PyxForge] QEMU launched successfully (PID ${activeQemuPid})`);
			if (launchData.args_used) {
				out.appendLine(`[PyxForge] Arguments: ${launchData.args_used.join(' ')}`);
			}

			updateQemuStatusBar();
			startQemuStatusPolling(coreBinaryPath);
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
			stopQemuStatusPolling();
		}
	});

	// -- debug (GDB attach) -------------------------------------------------
	const debugDisposable = vscode.commands.registerCommand('pyxforge.debug', async () => {
		const workspaceFolders = vscode.workspace.workspaceFolders;
		if (!workspaceFolders || workspaceFolders.length === 0) {
			vscode.window.showErrorMessage('PyxForge: No workspace folder is open.');
			return;
		}

		const projectRoot = workspaceFolders[0].uri.fsPath;
		const out = getOutputChannel();

		// Ensure QEMU is running in debug mode first.
		if (activeQemuPid === null) {
			out.show(true);
			out.appendLine('[PyxForge] No QEMU instance detected. Auto-launching in Debug Mode...');
			await vscode.commands.executeCommand('pyxforge.launch');
			// Give QEMU a moment to start up.
			await new Promise(resolve => setTimeout(resolve, 500));
			if (activeQemuPid === null) {
				vscode.window.showErrorMessage('PyxForge: Failed to auto-launch QEMU. Cannot attach GDB.');
				return;
			}
		}

		let selectedProfileName: string | undefined;
		try {
			const listResponse = await callCore(coreBinaryPath, {
				cmd: 'list-profiles',
				project_root: projectRoot,
			});
			const profiles: { name: string; tool: string; description?: string }[] =
				(listResponse.data as any)?.profiles || [];

			if (profiles.length > 0) {
				const items: vscode.QuickPickItem[] = profiles.map((p) => ({
					label: p.name,
					description: p.tool,
					detail: p.description,
				}));

				const selected = await vscode.window.showQuickPick(items, {
					placeHolder: 'Select a build profile to debug',
					title: 'PyxForge: Debug',
				});

				if (selected) {
					selectedProfileName = selected.label;
				} else {
					return; // User cancelled
				}
			}
		} catch (err: any) {
			out.appendLine(`[PyxForge] Failed to retrieve profiles: ${err.message}`);
		}

		try {
			out.show(true);
			out.appendLine('[PyxForge] Fetching debug configuration...');

			const response = await callCore(coreBinaryPath, {
				cmd: 'debug-config',
				project_root: projectRoot,
				profile: selectedProfileName,
			});

			const debugData = response.data as any;
			const gdbPath: string = debugData.gdb_executable;
			const target: string = debugData.target;
			const setupCommands: string[] = debugData.setup_commands || [];

			out.appendLine(`[PyxForge] GDB: ${gdbPath}`);
			out.appendLine(`[PyxForge] Architecture: ${debugData.architecture}`);
			out.appendLine(`[PyxForge] Target: ${target}`);
			out.appendLine(`[PyxForge] Setup commands: ${setupCommands.join('; ')}`);

			// Build the Native Debug launch configuration.
			const debugConfig: vscode.DebugConfiguration = {
				type: 'gdb',
				request: 'attach',
				name: 'PyxForge: GDB Attach',
				executable: '',
				remote: true,
				target: target,
				cwd: projectRoot,
				gdbpath: gdbPath,
				autorun: setupCommands,
				valuesFormatting: 'parseText',
			};

			out.appendLine('[PyxForge] Starting debug session...');

			const started = await vscode.debug.startDebugging(
				workspaceFolders[0],
				debugConfig
			);

			if (started) {
				vscode.window.showInformationMessage('PyxForge: GDB debug session started.');
			} else {
				vscode.window.showErrorMessage(
					'PyxForge: Failed to start debug session. Make sure the Native Debug extension (webfreak.debug) is installed.'
				);
			}
		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Debug session failed: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge Debug Failed: ${err.message}`);
		}
	});

	// -- initialize project -------------------------------------------------
	const initDisposable = vscode.commands.registerCommand('pyxforge.init', async () => {
		const workspaceFolders = vscode.workspace.workspaceFolders;
		if (!workspaceFolders || workspaceFolders.length === 0) {
			vscode.window.showErrorMessage('PyxForge: Please open a workspace folder first to initialize a project.');
			return;
		}

		const projectRoot = workspaceFolders[0].uri.fsPath;
		const defaultProjectName = path.basename(projectRoot);

		// Check if pyxforge.toml already exists
		const tomlPath = path.join(projectRoot, 'pyxforge.toml');
		try {
			await vscode.workspace.fs.stat(vscode.Uri.file(tomlPath));
			vscode.window.showErrorMessage('PyxForge: A pyxforge.toml already exists in this workspace. Aborting initialization.');
			return;
		} catch {
			// File does not exist, safe to continue
		}

		// Prompt user for a project name
		const projectName = await vscode.window.showInputBox({
			placeHolder: 'Enter your project name',
			value: defaultProjectName,
			title: 'PyxForge: Initialize Project',
			validateInput: (value) => {
				if (!value || value.trim().length === 0) {
					return 'Project name cannot be empty.';
				}
				return null;
			}
		});

		if (!projectName) {
			return; // User cancelled
		}

		const out = getOutputChannel();
		try {
			out.show(true);
			out.appendLine(`[PyxForge] Initializing project '${projectName}'...`);

			const response = await callCore(coreBinaryPath, {
				cmd: 'init',
				project_root: projectRoot,
				project_name: projectName
			});

			out.appendLine(`[PyxForge] ${response.message}`);
			vscode.window.showInformationMessage(`PyxForge: Project '${projectName}' initialized successfully.`);
		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Scaffolding failed: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge: Scaffolding failed: ${err.message}`);
		}
	});

	// -- open in Hex Viewer -------------------------------------------------
	const openHexDisposable = vscode.commands.registerCommand('pyxforge.openHex', async (uri?: vscode.Uri) => {
		let filePath: string | undefined;

		if (uri && uri.fsPath) {
			filePath = uri.fsPath;
		} else {
			// Prompt user to pick a file
			const fileUris = await vscode.window.showOpenDialog({
				canSelectMany: false,
				openLabel: 'Open in Hex Viewer',
				filters: {
					'Binary/Image Files': ['bin', 'img', 'iso', 'o', 'sys'],
					'All Files': ['*']
				}
			});

			if (fileUris && fileUris.length > 0) {
				filePath = fileUris[0].fsPath;
			}
		}

		if (!filePath) {
			return; // User cancelled
		}

		const out = getOutputChannel();
		try {
			out.show(true);
			out.appendLine(`[PyxForge] Loading hex dump for: ${filePath}`);

			const response = await callCore(coreBinaryPath, {
				cmd: 'hex-dump',
				file_path: filePath,
			});

			const hexData = response.data as any;
			PyxForgeHexPanel.createOrShow(context.extensionUri, filePath, hexData);
		} catch (err: any) {
			out.show(true);
			out.appendLine(`[PyxForge] Failed to dump hex data: ${err.message}`);
			vscode.window.showErrorMessage(`PyxForge Hex Dump Failed: ${err.message}`);
		}
	});

	// -- show CPU & Memory Inspector Panel ----------------------------------
	const showInspectorDisposable = vscode.commands.registerCommand('pyxforge.showInspector', () => {
		PyxForgeInspectorPanel.createOrShow(context.extensionUri);
		vscode.commands.executeCommand('pyxforge.refreshInspector');
	});

	// -- refresh CPU & Memory Inspector Panel state -------------------------
	const refreshInspectorDisposable = vscode.commands.registerCommand('pyxforge.refreshInspector', async () => {
		const session = vscode.debug.activeDebugSession;
		if (!session) {
			if (PyxForgeInspectorPanel.currentPanel) {
				PyxForgeInspectorPanel.currentPanel.update({
					status: 'disconnected',
					registers: [],
					stackAddress: 'N/A',
					stackDump: '',
					customAddress: '',
					customDump: ''
				});
			}
			return;
		}

		try {
			// Update status to running initially (we will change to stopped if we successfully query frame variables)
			if (PyxForgeInspectorPanel.currentPanel) {
				PyxForgeInspectorPanel.currentPanel.update({
					status: 'running',
					registers: [],
					stackAddress: 'N/A',
					stackDump: '',
					customAddress: PyxForgeInspectorPanel.currentPanel.getCustomAddress(),
					customDump: ''
				});
			}

			// 1. Get threads
			const threadsResponse = await session.customRequest('threads');
			const threadId = threadsResponse?.threads?.[0]?.id;
			if (threadId === undefined) {
				return;
			}

			// 2. Get top stack frame
			const stackResponse = await session.customRequest('stackTrace', { threadId, levels: 1 });
			const frameId = stackResponse?.stackFrames?.[0]?.id;
			if (frameId === undefined) {
				return;
			}

			// 3. Get scopes
			const scopesResponse = await session.customRequest('scopes', { frameId });
			const scopes = scopesResponse?.scopes || [];

			// 4. Find Registers scope
			const regScope = scopes.find((s: any) => s.name.toLowerCase() === 'registers');
			let registers: any[] = [];
			let spValue = 'N/A';
			if (regScope) {
				const varsResponse = await session.customRequest('variables', { variablesReference: regScope.variablesReference });
				registers = (varsResponse?.variables || []).map((v: any) => ({
					name: v.name,
					value: v.value
				}));

				const spReg = registers.find(r => ['esp', 'sp', 'rsp'].includes(r.name.toLowerCase()));
				if (spReg) {
					spValue = spReg.value;
				}
			}

			// 5. Get Stack dump
			let stackDump = '';
			if (spValue !== 'N/A') {
				try {
					const evalResponse = await session.customRequest('evaluate', {
						expression: `-exec x/16x ${spValue}`,
						frameId
					});
					stackDump = evalResponse?.result || '';
				} catch (e: any) {
					stackDump = `Failed to read stack at ${spValue}: ${e.message}`;
				}
			} else {
				stackDump = 'Stack pointer not found.';
			}

			// 6. Get custom Memory dump
			let customAddress = '0x7c00';
			if (PyxForgeInspectorPanel.currentPanel) {
				customAddress = PyxForgeInspectorPanel.currentPanel.getCustomAddress() || '0x7c00';
			}
			let customDump = '';
			try {
				const evalResponse = await session.customRequest('evaluate', {
					expression: `-exec x/16x ${customAddress}`,
					frameId
				});
				customDump = evalResponse?.result || '';
			} catch (e: any) {
				customDump = `Failed to read memory at ${customAddress}: ${e.message}`;
			}

			if (PyxForgeInspectorPanel.currentPanel) {
				PyxForgeInspectorPanel.currentPanel.update({
					status: 'stopped',
					registers,
					stackAddress: spValue,
					stackDump,
					customAddress,
					customDump
				});
			}
		} catch (err: any) {
			console.error('Error refreshing inspector:', err);
		}
	});

	// Track debugger lifecycle events to keep the inspector state in sync
	const sessionStartDisposable = vscode.debug.onDidStartDebugSession(() => {
		if (PyxForgeInspectorPanel.currentPanel) {
			PyxForgeInspectorPanel.currentPanel.update({
				status: 'running',
				registers: [],
				stackAddress: 'N/A',
				stackDump: '',
				customAddress: PyxForgeInspectorPanel.currentPanel.getCustomAddress(),
				customDump: ''
			});
		}
	});

	const sessionTerminateDisposable = vscode.debug.onDidTerminateDebugSession((session) => {
		if (session.type === 'gdb' && PyxForgeInspectorPanel.currentPanel) {
			PyxForgeInspectorPanel.currentPanel.update({
				status: 'disconnected',
				registers: [],
				stackAddress: 'N/A',
				stackDump: '',
				customAddress: '',
				customDump: ''
			});
		}
	});

	const activeSessionChangeDisposable = vscode.debug.onDidChangeActiveDebugSession(() => {
		vscode.commands.executeCommand('pyxforge.refreshInspector');
	});

	context.subscriptions.push(
		pingDisposable,
		buildDisposable,
		launchDisposable,
		launchNoDebugDisposable,
		stopDisposable,
		debugDisposable,
		initDisposable,
		openHexDisposable,
		showInspectorDisposable,
		refreshInspectorDisposable,
		trackerDisposable,
		sessionStartDisposable,
		sessionTerminateDisposable,
		activeSessionChangeDisposable
	);
}

export function deactivate() {
	stopQemuStatusPolling();
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
