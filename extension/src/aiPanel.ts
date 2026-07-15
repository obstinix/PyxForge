import * as vscode from 'vscode';
import * as path from 'path';

export class PyxForgeAiPanel {
	public static currentPanel: PyxForgeAiPanel | undefined;
	private readonly panel: vscode.WebviewPanel;
	private disposables: vscode.Disposable[] = [];
	private rawContent: string = '';

	public static createOrShow(extensionUri: vscode.Uri) {
		const column = vscode.window.activeTextEditor
			? vscode.window.activeTextEditor.viewColumn
			: undefined;

		if (PyxForgeAiPanel.currentPanel) {
			PyxForgeAiPanel.currentPanel.panel.reveal(column);
			PyxForgeAiPanel.currentPanel.clear();
			return PyxForgeAiPanel.currentPanel;
		}

		const panel = vscode.window.createWebviewPanel(
			'pyxforgeAi',
			'PyxForge AI Assistant',
			column || vscode.ViewColumn.Three,
			{
				enableScripts: true,
				retainContextWhenHidden: true,
			}
		);

		PyxForgeAiPanel.currentPanel = new PyxForgeAiPanel(panel);
		return PyxForgeAiPanel.currentPanel;
	}

	private constructor(panel: vscode.WebviewPanel) {
		this.panel = panel;
		this.panel.webview.html = this.getHtmlContent();
		this.panel.onDidDispose(() => this.dispose(), null, this.disposables);
	}

	public appendChunk(text: string) {
		this.rawContent += text;
		this.panel.webview.postMessage({
			type: 'chunk',
			raw: this.rawContent,
		});
	}

	public clear() {
		this.rawContent = '';
		this.panel.webview.postMessage({
			type: 'clear',
		});
	}

	public dispose() {
		PyxForgeAiPanel.currentPanel = undefined;
		this.panel.dispose();

		while (this.disposables.length) {
			const x = this.disposables.pop();
			if (x) {
				x.dispose();
			}
		}
	}

	private getHtmlContent(): string {
		return `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>PyxForge AI Assistant</title>
	<!-- Include Marked.js for markdown parsing -->
	<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
	<style>
		:root {
			--background-color: #1e1e2e;
			--card-background: #252538;
			--text-color: #cdd6f4;
			--accent-color: #cba6f7;
			--border-color: #45475a;
			--header-background: #11111b;
		}

		body {
			background-color: var(--background-color);
			color: var(--text-color);
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			margin: 0;
			padding: 15px;
			line-height: 1.5;
		}

		.header {
			background-color: var(--header-background);
			padding: 12px 15px;
			border-radius: 8px;
			border: 1px solid var(--border-color);
			display: flex;
			align-items: center;
			gap: 10px;
			margin-bottom: 15px;
		}

		.header h2 {
			margin: 0;
			font-size: 1rem;
			color: var(--accent-color);
		}

		.header .sparkle {
			font-size: 1.2rem;
		}

		.content-container {
			background-color: rgba(0, 0, 0, 0.15);
			border: 1px solid var(--border-color);
			border-radius: 8px;
			padding: 15px;
			min-height: 200px;
			overflow-y: auto;
		}

		pre {
			background-color: rgba(0, 0, 0, 0.3);
			padding: 12px;
			border-radius: 6px;
			border: 1px solid var(--border-color);
			overflow-x: auto;
		}

		code {
			font-family: "SFMono-Regular", Consolas, monospace;
			font-size: 0.85em;
			background-color: rgba(255, 255, 255, 0.05);
			padding: 2px 4px;
			border-radius: 3px;
		}

		pre code {
			background-color: transparent;
			padding: 0;
		}

		a {
			color: var(--accent-color);
			text-decoration: none;
		}

		a:hover {
			text-decoration: underline;
		}

		.cursor {
			display: inline-block;
			width: 6px;
			height: 14px;
			background-color: var(--accent-color);
			margin-left: 3px;
			animation: blink 0.8s infinite;
			vertical-align: middle;
		}

		@keyframes blink {
			0%, 100% { opacity: 0; }
			50% { opacity: 1; }
		}
	</style>
</head>
<body>
	<div class="header">
		<span class="sparkle">✨</span>
		<h2>PyxForge AI Assistant</h2>
	</div>

	<div class="content-container" id="content">
		<div style="opacity: 0.5; font-style: italic;">Awaiting explanation stream...</div>
	</div>

	<script>
		let isStreaming = false;

		window.addEventListener('message', event => {
			const message = event.data;
			const container = document.getElementById('content');

			if (message.type === 'clear') {
				container.innerHTML = '<div style="opacity: 0.5; font-style: italic;">Awaiting explanation stream...</div>';
				isStreaming = false;
			} else if (message.type === 'chunk') {
				isStreaming = true;
				// Parse markdown using marked.js
				const parsedHtml = marked.parse(message.raw);
				container.innerHTML = parsedHtml + '<span class="cursor"></span>';
				
				// Scroll to bottom
				window.scrollTo({
					top: document.body.scrollHeight,
					behavior: 'smooth'
				});
			}
		});
	</script>
</body>
</html>`;
	}
}
