import * as vscode from 'vscode';
import { PyxForgeAiPanel } from './aiPanel';

export async function explainAssembly(code: string, panel: PyxForgeAiPanel) {
	const systemPrompt = `You are PyxForge AI, an expert systems programmer and BIOS/kernel developer.
Explain the selected x86 assembly code line-by-line for a student learning OS development.
Point out syntax, registers used, memory locations, interrupts, and segment usage if any.
Be concise but extremely informative. Format your response in clean Markdown.`;

	const userPrompt = `Please explain this selected assembly code:
\`\`\`nasm
${code}
\`\`\``;

	await streamModelResponse(systemPrompt, userPrompt, panel);
}

export async function explainRegisters(registers: { name: string; value: string }[], panel: PyxForgeAiPanel) {
	const systemPrompt = `You are PyxForge AI, an expert systems programmer and BIOS/kernel developer.
Analyze the current CPU registers and flags state and explain what the CPU is currently doing.
Focus on standard registers (EAX, EBX, EIP, ESP, EFLAGS) and segment registers (CS, DS, ES, SS).
Determine if the CPU is in real mode, protected mode, or long mode.
Identify potential operations, stack pointers, or program counter locations.
Be concise but extremely informative. Format your response in clean Markdown.`;

	const formattedRegs = registers.map(r => `${r.name}: ${r.value}`).join('\n');
	const userPrompt = `Here is the current CPU registers state:
\`\`\`
${formattedRegs}
\`\`\``;

	await streamModelResponse(systemPrompt, userPrompt, panel);
}

export async function explainBuildError(errorLog: string, panel: PyxForgeAiPanel) {
	const systemPrompt = `You are PyxForge AI, an expert systems programmer and BIOS/kernel developer.
Analyze this compilation/assembly error log and explain what went wrong.
Suggest clear, concrete steps to fix the issue.
Be concise but extremely informative. Format your response in clean Markdown.`;

	const userPrompt = `Here is the build error output:
\`\`\`
${errorLog}
\`\`\``;

	await streamModelResponse(systemPrompt, userPrompt, panel);
}

async function streamModelResponse(systemPrompt: string, userPrompt: string, panel: PyxForgeAiPanel) {
	panel.clear();
	panel.appendChunk('Searching for active Language Model...\n');

	try {
		// Try selecting Gemini first, then Copilot, then any model
		let models = await vscode.lm.selectChatModels({ family: 'gemini-1.5-pro' });
		if (models.length === 0) {
			models = await vscode.lm.selectChatModels({ family: 'copilot-gpt-4o' });
		}
		if (models.length === 0) {
			models = await vscode.lm.selectChatModels();
		}

		if (models.length === 0) {
			panel.clear();
			panel.appendChunk('❌ Error: No language model is active in this VS Code workspace. Make sure Copilot or Gemini is installed and authorized.');
			return;
		}

		const model = models[0];
		panel.clear();
		panel.appendChunk(`🤖 *Using model: ${model.name} (${model.vendor})*\n\n---\n\n`);

		const combinedPrompt = `${systemPrompt}\n\n---\n\nUser Input:\n${userPrompt}`;
		const messages = [
			new vscode.LanguageModelChatMessage(vscode.LanguageModelChatMessageRole.User, combinedPrompt),
		];

		const request = await model.sendRequest(messages, {}, new vscode.CancellationTokenSource().token);
		
		for await (const chunk of request.text) {
			panel.appendChunk(chunk);
		}
	} catch (err: any) {
		panel.appendChunk(`\n\n❌ Error streaming from Language Model: ${err.message}`);
	}
}
