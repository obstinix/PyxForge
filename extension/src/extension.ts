import * as vscode from 'vscode';
import * as path from 'path';
import * as cp from 'child_process';

export function activate(context: vscode.ExtensionContext) {
	console.log('PyxForge extension is now active!');

	const disposable = vscode.commands.registerCommand('pyxforge.ping', () => {
		// TODO(phase-N): The following path is hardcoded for development and must be replaced by production packaging logic later.
		const binaryName = process.platform === 'win32' ? 'pyxforge-core.exe' : 'pyxforge-core';
		const coreBinaryPath = path.join(context.extensionPath, '..', 'core', 'target', 'debug', binaryName);

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
				vscode.window.showErrorMessage(`Failed to spawn PyxForge Core binary at ${coreBinaryPath}: ${err.message}`);
			});

			child.on('close', (code) => {
				if (code !== 0) {
					vscode.window.showErrorMessage(`PyxForge Core exited with code ${code}. Error: ${stderrData || stdoutData || 'Unknown error'}`);
					return;
				}

				try {
					const response = JSON.parse(stdoutData.trim());
					if (response.status === 'ok') {
						vscode.window.showInformationMessage(
							`PyxForge Core Ping Successful! Status: ${response.status}, Version: ${response.version}, Message: ${response.message}`
						);
					} else {
						vscode.window.showErrorMessage(`PyxForge Core returned error response: ${response.message || stdoutData}`);
					}
				} catch (parseErr: any) {
					vscode.window.showErrorMessage(`Failed to parse PyxForge Core response JSON: ${parseErr.message}. Raw output: ${stdoutData}`);
				}
			});

			// Write the request to core stdin and close it
			child.stdin.write(JSON.stringify({ cmd: 'ping' }) + '\n');
			child.stdin.end();

		} catch (spawnErr: any) {
			vscode.window.showErrorMessage(`Unexpected exception spawning PyxForge Core: ${spawnErr.message}`);
		}
	});

	context.subscriptions.push(disposable);
}

export function deactivate() {}
