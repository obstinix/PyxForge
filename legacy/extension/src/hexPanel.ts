import * as vscode from 'vscode';
import * as path from 'path';

/**
 * Represents a single formatted line in a hex dump view panel.
 */
export interface HexDumpLine {
	offset: number;
	hex_bytes: string[];
	ascii: string;
}

/**
 * Encapsulates the complete hex dump information payload returned from the core backend.
 */
export interface HexDumpData {
	is_boot_sector: boolean;
	has_boot_signature: boolean;
	file_size: number;
	lines: HexDumpLine[];
}

export class PyxForgeHexPanel {
	public static currentPanels: Map<string, PyxForgeHexPanel> = new Map();
	private readonly panel: vscode.WebviewPanel;
	private readonly filePath: string;
	private readonly extensionUri: vscode.Uri;
	private disposables: vscode.Disposable[] = [];

	public static createOrShow(extensionUri: vscode.Uri, filePath: string, data: HexDumpData) {
		const normalizedPath = path.normalize(filePath);
		const existingPanel = PyxForgeHexPanel.currentPanels.get(normalizedPath);

		if (existingPanel) {
			existingPanel.panel.reveal();
			existingPanel.update(data);
			return;
		}

		const fileName = path.basename(filePath);
		const panel = vscode.window.createWebviewPanel(
			'pyxforgeHex',
			`Hex: ${fileName}`,
			vscode.ViewColumn.One,
			{
				enableScripts: true,
				retainContextWhenHidden: true,
			}
		);

		const newPanel = new PyxForgeHexPanel(panel, normalizedPath, data, extensionUri);
		PyxForgeHexPanel.currentPanels.set(normalizedPath, newPanel);
	}

	private constructor(panel: vscode.WebviewPanel, filePath: string, data: HexDumpData, extensionUri: vscode.Uri) {
		this.panel = panel;
		this.filePath = filePath;
		this.extensionUri = extensionUri;

		this.panel.onDidDispose(() => this.dispose(), null, this.disposables);
		this.update(data);
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

	public update(data: HexDumpData) {
		const fileName = path.basename(this.filePath);
		this.panel.webview.html = this.getHtmlContent(fileName, data);
	}

	public dispose() {
		PyxForgeHexPanel.currentPanels.delete(this.filePath);
		this.panel.dispose();

		while (this.disposables.length) {
			const x = this.disposables.pop();
			if (x) {
				x.dispose();
			}
		}
	}

	private getHtmlContent(fileName: string, data: HexDumpData): string {
		let statusHtml = '';
		if (data.is_boot_sector) {
			if (data.has_boot_signature) {
				statusHtml = `
					<div class="status-banner valid">
						<span class="status-icon">✔</span>
						<div>
							<strong>Valid BIOS Boot Sector</strong>
							<div class="status-desc">Exactly 512 bytes with bootloader signature (0xAA55) detected.</div>
						</div>
					</div>
				`;
			} else {
				statusHtml = `
					<div class="status-banner invalid">
						<span class="status-icon">⚠</span>
						<div>
							<strong>Invalid Boot Sector</strong>
							<div class="status-desc">File size is 512 bytes, but the 0xAA55 boot signature is missing! It will not boot.</div>
						</div>
					</div>
				`;
			}
		} else {
			statusHtml = `
				<div class="status-banner info">
					<span class="status-icon">🛈</span>
					<div>
						<strong>Raw Binary File</strong>
						<div class="status-desc">Size: ${data.file_size} bytes. (Not a standard 512-byte BIOS boot sector).</div>
					</div>
				</div>
			`;
		}

		let hexLinesHtml = '';
		data.lines.forEach((line) => {
			const offsetStr = line.offset.toString(16).padStart(8, '0');
			
			// Format hex bytes
			const byteSpans = line.hex_bytes.map((byte, idx) => {
				const globalIdx = line.offset + idx;
				const isSig = data.is_boot_sector && (globalIdx === 510 || globalIdx === 511);
				const cls = isSig ? 'class="sig-byte" title="Boot Signature Byte (0xAA55)"' : '';
				return `<span ${cls}>${byte.toUpperCase()}</span>`;
			});

			// Group into pairs for readability
			let byteGroups = [];
			for (let j = 0; j < byteSpans.length; j += 2) {
				const pair = byteSpans.slice(j, j + 2).join(' ');
				byteGroups.push(pair);
			}
			const hexBytesStr = byteGroups.join('  ');

			// Sanitize ASCII view
			const asciiSafe = line.ascii
				.replace(/&/g, '&amp;')
				.replace(/</g, '&lt;')
				.replace(/>/g, '&gt;');

			hexLinesHtml += `<div class="hex-row"><span class="offset">${offsetStr}</span>  <span class="bytes">${hexBytesStr}</span>  <span class="ascii">|${asciiSafe}|</span></div>`;
		});

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
	<title>Hex: ${fileName}</title>
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
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			margin: 0;
			padding: 15px;
			user-select: text;
		}

		.header {
			background-color: var(--header-background);
			padding: 15px;
			border-radius: 8px;
			border: 1px solid var(--border-color);
			margin-bottom: 15px;
		}

		.header h2 {
			margin: 0 0 6px 0;
			font-size: 1.1rem;
			color: var(--accent-color);
		}

		.header .meta {
			font-size: 0.85rem;
			opacity: 0.65;
			font-family: monospace;
			word-break: break-all;
		}

		.status-banner {
			display: flex;
			gap: 12px;
			align-items: center;
			padding: 12px 15px;
			border-radius: 6px;
			margin-bottom: 15px;
			font-size: 0.9rem;
			border: 1px solid;
		}

		.status-banner.valid {
			background-color: rgba(166, 227, 161, 0.12);
			border-color: var(--color-success);
			color: var(--color-success);
		}

		.status-banner.invalid {
			background-color: rgba(249, 226, 175, 0.12);
			border-color: var(--color-error);
			color: var(--color-error);
		}

		.status-banner.info {
			background-color: rgba(137, 180, 250, 0.12);
			border-color: var(--border-color);
			color: var(--text-color);
		}

		.status-icon {
			font-size: 1.5rem;
			font-weight: bold;
		}

		.status-desc {
			font-size: 0.8rem;
			margin-top: 2px;
			opacity: 0.85;
		}

		.viewer-container {
			background-color: rgba(0, 0, 0, 0.25);
			border: 1px solid var(--border-color);
			border-radius: 8px;
			padding: 15px;
			overflow-x: auto;
		}

		.hex-row {
			font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
			font-size: 0.85rem;
			line-height: 1.45;
			display: flex;
			white-space: pre;
		}

		.offset {
			color: #585b70;
			margin-right: 15px;
		}

		.bytes {
			color: var(--text-color);
			margin-right: 25px;
		}

		.ascii {
			color: #a6adc8;
		}

		.sig-byte {
			background-color: rgba(249, 226, 175, 0.2);
			color: var(--color-warning);
			border-bottom: 2px solid var(--color-warning);
			font-weight: bold;
			padding: 0 1px;
			border-radius: 2px;
		}
	</style>
</head>
<body>
	<div class="header">
		<h2>Binary Hex Explorer</h2>
		<div class="meta">File: ${this.filePath}</div>
	</div>

	${statusHtml}

	<div class="viewer-container">
		${hexLinesHtml}
	</div>
	<script>
		window.addEventListener('message', event => {
			const message = event.data;
			if (message.type === 'updateTheme') {
				document.documentElement.setAttribute('data-theme', message.theme);
			}
		});
	</script>
</body>
</html>`;
	}
}
