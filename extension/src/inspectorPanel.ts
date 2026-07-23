import * as vscode from 'vscode';

/**
 * Represents a single CPU register state.
 */
export interface Register {
	name: string;
	value: string;
	changed?: boolean;
}

/**
 * State payload sent to the CPU Inspector webview panel on debugger halts/steps.
 */
export interface InspectorState {
	status: 'running' | 'stopped' | 'disconnected';
	registers: Register[];
	stackAddress: string;
	stackDump: string;
	customAddress: string;
	customDump: string;
}

export class PyxForgeInspectorPanel {
	public static currentPanel: PyxForgeInspectorPanel | undefined;
	private readonly panel: vscode.WebviewPanel;
	private readonly extensionUri: vscode.Uri;
	private disposables: vscode.Disposable[] = [];
	private customAddress: string = '0x7c00';

	public static createOrShow(extensionUri: vscode.Uri) {
		const column = vscode.window.activeTextEditor
			? vscode.window.activeTextEditor.viewColumn
			: undefined;

		if (PyxForgeInspectorPanel.currentPanel) {
			PyxForgeInspectorPanel.currentPanel.panel.reveal(column);
			return;
		}

		const panel = vscode.window.createWebviewPanel(
			'pyxforgeInspector',
			'PyxForge: CPU & Memory Inspector',
			column || vscode.ViewColumn.Two,
			{
				enableScripts: true,
				retainContextWhenHidden: true,
			}
		);

		PyxForgeInspectorPanel.currentPanel = new PyxForgeInspectorPanel(panel, extensionUri);
	}

	private constructor(panel: vscode.WebviewPanel, extensionUri: vscode.Uri) {
		this.panel = panel;
		this.extensionUri = extensionUri;

		this.updateHtml();

		this.panel.onDidDispose(() => this.dispose(), null, this.disposables);

		// Handle messages from the Webview
		this.panel.webview.onDidReceiveMessage(
			async (message) => {
				switch (message.command) {
					case 'readMemory':
						this.customAddress = message.address;
						await this.triggerFetch();
						break;
					case 'explainRegisters':
						await vscode.commands.executeCommand('pyxforge.explainActiveRegisters');
						break;
				}
			},
			null,
			this.disposables
		);
	}

	public update(state: InspectorState) {
		this.panel.webview.postMessage({
			type: 'updateState',
			state: {
				...state,
				customAddress: this.customAddress,
			},
		});
	}

	public getCustomAddress(): string {
		return this.customAddress;
	}

	private async triggerFetch() {
		// Trigger an active debugging state fetch in extension.ts
		await vscode.commands.executeCommand('pyxforge.refreshInspector');
	}

	public dispose() {
		PyxForgeInspectorPanel.currentPanel = undefined;

		this.panel.dispose();

		while (this.disposables.length) {
			const x = this.disposables.pop();
			if (x) {
				x.dispose();
			}
		}
	}

	private updateHtml() {
		this.panel.webview.html = this.getHtmlContent();
	}

	public updateTheme(theme: string) {
		let activeTheme = theme;
		if (activeTheme === 'auto') {
			const kind = vscode.window.activeColorTheme.kind;
			if (kind === vscode.ColorThemeKind.HighContrast || kind === vscode.ColorThemeKind.HighContrastLight) {
				activeTheme = 'contrast';
			} else {
				activeTheme = 'hybrid';
			}
		}
		this.panel.webview.postMessage({
			type: 'updateTheme',
			theme: activeTheme,
		});
	}

	private getHtmlContent(): string {
		const webview = this.panel.webview;
		const monoUri = webview.asWebviewUri(vscode.Uri.joinPath(this.extensionUri, 'themes', 'mono.css'));
		const contrastUri = webview.asWebviewUri(vscode.Uri.joinPath(this.extensionUri, 'themes', 'contrast.css'));
		const hybridUri = webview.asWebviewUri(vscode.Uri.joinPath(this.extensionUri, 'themes', 'hybrid.css'));

		let activeTheme = vscode.workspace.getConfiguration('pyxforge').get<string>('theme', 'mono');
		if (activeTheme === 'auto') {
			const kind = vscode.window.activeColorTheme.kind;
			if (kind === vscode.ColorThemeKind.HighContrast || kind === vscode.ColorThemeKind.HighContrastLight) {
				activeTheme = 'contrast';
			} else {
				activeTheme = 'hybrid';
			}
		}

		return `<!DOCTYPE html>
<html lang="en" data-theme="${activeTheme}">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>PyxForge Inspector</title>
	<link rel="stylesheet" href="${monoUri}">
	<link rel="stylesheet" href="${contrastUri}">
	<link rel="stylesheet" href="${hybridUri}">
	<style>
		:root {
			--background-color: #1e1e2e;
			--card-background: #252538;
			--text-color: #cdd6f4;
			--accent-color: #00D4FF;
			--border-color: #45475a;
			--header-background: #11111b;

			/* Decoupled semantic colors */
			--color-success: #10B981;
			--color-warning: #F59E0B;
			--color-error: #EF4444;
		}

		body {
			background-color: var(--background-color);
			color: var(--text-color);
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
			margin: 0;
			padding: 15px;
			user-select: none;
		}

		.header {
			background-color: var(--header-background);
			padding: 12px 15px;
			border-radius: 8px;
			border: 1px solid var(--border-color);
			display: flex;
			justify-content: space-between;
			align-items: center;
			margin-bottom: 15px;
		}

		.title-container h2 {
			margin: 0;
			font-size: 1.1rem;
			color: var(--accent-color);
			font-weight: 600;
		}

		.status-badge {
			padding: 4px 8px;
			border-radius: 4px;
			font-size: 0.8rem;
			font-weight: bold;
			text-transform: uppercase;
			display: flex;
			align-items: center;
			gap: 6px;
		}

		.status-badge.running {
			background-color: rgba(166, 227, 161, 0.15);
			color: var(--color-success);
			border: 1px solid var(--color-success);
		}

		.status-badge.stopped {
			background-color: rgba(249, 226, 175, 0.15);
			color: var(--color-warning);
			border: 1px solid var(--color-warning);
		}

		.status-badge.disconnected {
			background-color: rgba(243, 139, 168, 0.15);
			color: var(--color-error);
			border: 1px solid var(--color-error);
		}

		.status-dot {
			width: 8px;
			height: 8px;
			border-radius: 50%;
			background-color: currentColor;
			display: inline-block;
		}

		.tabs {
			display: flex;
			gap: 5px;
			margin-bottom: 15px;
			border-bottom: 1px solid var(--border-color);
			padding-bottom: 5px;
		}

		.tab {
			background: none;
			border: none;
			color: var(--text-color);
			opacity: 0.6;
			padding: 8px 16px;
			font-size: 0.9rem;
			font-weight: 500;
			cursor: pointer;
			border-radius: 4px 4px 0 0;
			transition: all 0.2s ease;
		}

		.tab:hover {
			opacity: 0.9;
			background-color: rgba(255, 255, 255, 0.05);
		}

		.tab.active {
			opacity: 1;
			color: var(--accent-color);
			border-bottom: 2px solid var(--accent-color);
		}

		.tab-content {
			display: none;
		}

		.tab-content.active {
			display: block;
		}

		.card {
			background-color: var(--card-background);
			border: 1px solid var(--border-color);
			border-radius: 8px;
			padding: 15px;
			box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		}

		.grid-container {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
			gap: 10px;
		}

		.register-box {
			background-color: rgba(0, 0, 0, 0.2);
			border: 1px solid var(--border-color);
			border-radius: 6px;
			padding: 8px 12px;
			display: flex;
			flex-direction: column;
			position: relative;
			overflow: hidden;
		}

		.register-box.changed {
			border-left: 3px solid var(--accent-color);
			animation: highlight-change 1.5s ease-out;
		}

		@keyframes highlight-change {
			0% { border-color: var(--color-warning); background-color: rgba(249, 226, 175, 0.2); }
			100% { border-color: var(--border-color); background-color: rgba(0, 0, 0, 0.2); }
		}

		.register-name {
			font-size: 0.75rem;
			color: var(--accent-color);
			text-transform: uppercase;
			font-weight: 600;
		}

		.register-val {
			font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
			font-size: 0.95rem;
			margin-top: 4px;
		}

		.flags-container {
			display: flex;
			flex-wrap: wrap;
			gap: 8px;
			margin-top: 15px;
			padding-top: 15px;
			border-top: 1px solid var(--border-color);
		}

		.flag-badge {
			padding: 4px 8px;
			border-radius: 4px;
			font-size: 0.75rem;
			font-weight: bold;
			font-family: monospace;
			border: 1px solid var(--border-color);
			background-color: rgba(0, 0, 0, 0.2);
			opacity: 0.4;
		}

		.flag-badge.active {
			opacity: 1;
			background-color: rgba(166, 227, 161, 0.15);
			color: var(--color-success);
			border-color: var(--color-success);
		}

		.hex-viewer {
			font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
			font-size: 0.85rem;
			white-space: pre;
			overflow-x: auto;
			background-color: rgba(0, 0, 0, 0.25);
			padding: 12px;
			border-radius: 6px;
			border: 1px solid var(--border-color);
			line-height: 1.4;
		}

		.search-bar {
			display: flex;
			gap: 8px;
			margin-bottom: 12px;
		}

		.search-input {
			flex: 1;
			background-color: rgba(0, 0, 0, 0.2);
			border: 1px solid var(--border-color);
			color: var(--text-color);
			padding: 6px 10px;
			border-radius: 4px;
			font-family: monospace;
			outline: none;
		}

		.search-input:focus {
			border-color: var(--accent-color);
		}

		.search-btn {
			background-color: var(--accent-color);
			color: var(--header-background);
			border: none;
			padding: 6px 15px;
			border-radius: 4px;
			font-weight: bold;
			cursor: pointer;
			transition: opacity 0.2s;
		}

		.search-btn:hover {
			opacity: 0.9;
		}

		.info-text {
			font-size: 0.85rem;
			opacity: 0.6;
			margin-bottom: 10px;
		}
	</style>
</head>
<body>
	<div class="header">
		<div class="title-container">
			<h2>PyxForge Inspector</h2>
		</div>
		<div id="statusBadge" class="status-badge disconnected">
			<span class="status-dot"></span>
			<span id="statusText">Disconnected</span>
		</div>
	</div>

	<div class="tabs">
		<button class="tab active" onclick="switchTab('registers')">Registers</button>
		<button class="tab" onclick="switchTab('stack')">Stack Viewer</button>
		<button class="tab" onclick="switchTab('memory')">Memory Explorer</button>
	</div>

	<div id="registersTab" class="tab-content active">
		<div class="card">
			<div style="display: flex; justify-content: flex-end; margin-bottom: 10px;">
				<button class="search-btn" id="explainCpuBtn" onclick="explainCpu()" style="display: none; background-color: var(--accent-color); color: var(--header-background);">✨ Explain CPU State</button>
			</div>
			<div class="grid-container" id="registersGrid">
				<!-- Registers loaded dynamically -->
			</div>
			<div class="flags-container" id="flagsContainer">
				<!-- Flags loaded dynamically -->
			</div>
		</div>
	</div>

	<div id="stackTab" class="tab-content">
		<div class="card">
			<div class="info-text" id="stackAddressText">Stack Pointer address: N/A</div>
			<div class="hex-viewer" id="stackViewer">Target is not stopped or GDB is detached.</div>
		</div>
	</div>

	<div id="memoryTab" class="tab-content">
		<div class="card">
			<div class="search-bar">
				<input type="text" id="memoryAddressInput" class="search-input" value="0x7c00" placeholder="Address (e.g. 0x7c00)">
				<button class="search-btn" onclick="queryMemory()">Read</button>
			</div>
			<div class="hex-viewer" id="memoryViewer">Target is not stopped or GDB is detached.</div>
		</div>
	</div>

	<script>
		const vscode = acquireVsCodeApi();
		let previousRegisters = {};

		// Define flags for decomposition
		const cpuFlags = [
			{ mask: 0x0001, name: 'CF', desc: 'Carry Flag' },
			{ mask: 0x0004, name: 'PF', desc: 'Parity Flag' },
			{ mask: 0x0010, name: 'AF', desc: 'Auxiliary Carry Flag' },
			{ mask: 0x0040, name: 'ZF', desc: 'Zero Flag' },
			{ mask: 0x0080, name: 'SF', desc: 'Sign Flag' },
			{ mask: 0x0100, name: 'TF', desc: 'Trap Flag' },
			{ mask: 0x0200, name: 'IF', desc: 'Interrupt Enable Flag' },
			{ mask: 0x0400, name: 'DF', desc: 'Direction Flag' },
			{ mask: 0x0800, name: 'OF', desc: 'Overflow Flag' }
		];

		function switchTab(tabId) {
			document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
			document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

			const event = window.event;
			if (event) {
				event.target.classList.add('active');
			}
			document.getElementById(tabId + 'Tab').classList.add('active');
		}

		function queryMemory() {
			const address = document.getElementById('memoryAddressInput').value.trim();
			if (address) {
				vscode.postMessage({
					command: 'readMemory',
					address: address
				});
			}
		}

		function explainCpu() {
			vscode.postMessage({
				command: 'explainRegisters'
			});
		}

		window.addEventListener('message', event => {
			const message = event.data;
			if (message.type === 'updateTheme') {
				document.documentElement.setAttribute('data-theme', message.theme);
			} else if (message.type === 'updateState') {
				const state = message.state;
				
				// Update Status Badge
				const badge = document.getElementById('statusBadge');
				const text = document.getElementById('statusText');
				badge.className = 'status-badge ' + state.status;
				text.innerText = state.status;

				// Update memory input field if changed by the backend
				if (state.customAddress) {
					document.getElementById('memoryAddressInput').value = state.customAddress;
				}

				if (state.status === 'disconnected') {
					document.getElementById('explainCpuBtn').style.display = 'none';
					document.getElementById('registersGrid').innerHTML = '<div class="info-text">Debugger disconnected</div>';
					document.getElementById('flagsContainer').innerHTML = '';
					document.getElementById('stackViewer').innerText = 'Target is disconnected.';
					document.getElementById('memoryViewer').innerText = 'Target is disconnected.';
					document.getElementById('stackAddressText').innerText = 'Stack Pointer address: N/A';
					previousRegisters = {};
					return;
				}

				if (state.status === 'running') {
					document.getElementById('explainCpuBtn').style.display = 'none';
					document.getElementById('stackViewer').innerText = 'Target is running...';
					document.getElementById('memoryViewer').innerText = 'Target is running...';
					return;
				}

				if (state.status === 'stopped') {
					document.getElementById('explainCpuBtn').style.display = 'block';
				}

				// Update Registers Grid
				const grid = document.getElementById('registersGrid');
				grid.innerHTML = '';
				
				let eflagsVal = 0;

				state.registers.forEach(reg => {
					const isChanged = !!reg.changed;
					
					const box = document.createElement('div');
					box.className = 'register-box' + (isChanged ? ' changed' : '');
					
					const name = document.createElement('span');
					name.className = 'register-name';
					name.innerText = reg.name;

					const val = document.createElement('span');
					val.className = 'register-val';
					val.innerText = reg.value;

					box.appendChild(name);
					box.appendChild(val);
					grid.appendChild(box);

					// Store previous registers
					previousRegisters[reg.name] = reg.value;

					// Track eflags value for decomp
					if (reg.name.toLowerCase() === 'eflags' || reg.name.toLowerCase() === 'flags') {
						eflagsVal = parseInt(reg.value, 16) || parseInt(reg.value, 10) || 0;
					}
				});

				// Update Flags
				const flagsContainer = document.getElementById('flagsContainer');
				flagsContainer.innerHTML = '';
				if (eflagsVal !== 0) {
					cpuFlags.forEach(flag => {
						const active = (eflagsVal & flag.mask) !== 0;
						const badge = document.createElement('div');
						badge.className = 'flag-badge' + (active ? ' active' : '');
						badge.title = flag.desc;
						badge.innerText = flag.name + ': ' + (active ? '1' : '0');
						flagsContainer.appendChild(badge);
					});
				}

				// Update Stack
				document.getElementById('stackAddressText').innerText = 'Stack Pointer address: ' + (state.stackAddress || 'N/A');
				document.getElementById('stackViewer').innerText = state.stackDump || 'No stack dump data available.';

				// Update Memory
				document.getElementById('memoryViewer').innerText = state.customDump || 'No memory dump data available.';
			}
		});
	</script>
</body>
</html>`;
	}
}
