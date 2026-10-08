import * as vscode from 'vscode';

export class PyxForgeDebugTracker implements vscode.DebugAdapterTracker {
	constructor(
		private session: vscode.DebugSession,
		private onStopCallback: (session: vscode.DebugSession) => void
	) {}

	onDidSendMessage(message: any): void {
		// Capture "stopped" event which indicates the debugger has paused execution (e.g. hit breakpoint or step)
		if (message.type === 'event' && message.event === 'stopped') {
			this.onStopCallback(this.session);
		}
	}
}

export class PyxForgeDebugTrackerFactory implements vscode.DebugAdapterTrackerFactory {
	constructor(private onStopCallback: (session: vscode.DebugSession) => void) {}

	createDebugAdapterTracker(session: vscode.DebugSession): vscode.ProviderResult<vscode.DebugAdapterTracker> {
		// Only track GDB debug sessions
		if (session.type === 'gdb') {
			return new PyxForgeDebugTracker(session, this.onStopCallback);
		}
		return undefined;
	}
}
