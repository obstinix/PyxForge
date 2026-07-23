import "./fonts";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { CodeEditor } from "./editor";

const codeEditor = new CodeEditor();

// Interfaces from original extension codebases
interface Register {
  name: string;
  value: string;
  changed?: boolean;
}



// DOM Elements
let statusDotEl: HTMLElement | null = null;
let statusTextEl: HTMLElement | null = null;
let statusBadgeEl: HTMLElement | null = null;
let inspectorStatusTextEl: HTMLElement | null = null;
let pingBtnEl: HTMLButtonElement | null = null;
let initBtnEl: HTMLButtonElement | null = null;
let listProfilesBtnEl: HTMLButtonElement | null = null;
let spawnShellBtnEl: HTMLButtonElement | null = null;
let stepBtnEl: HTMLButtonElement | null = null;
let explainCpuBtnEl: HTMLButtonElement | null = null;
let projNameInputEl: HTMLInputElement | null = null;
let presetListEl: HTMLElement | null = null;

let pluginPathInputEl: HTMLInputElement | null = null;
let loadPluginBtnEl: HTMLButtonElement | null = null;
let pluginExtensionsContainerEl: HTMLElement | null = null;

interface PluginContext {
  log: (msg: string, type?: 'info' | 'success' | 'error' | 'system') => void;
  registerButton: (id: string, label: string, onClick: () => void) => void;
  sendRequest: (req: any) => Promise<any>;
}

const loadedPlugins: Map<string, any> = new Map();

let snapshotTagInputEl: HTMLInputElement | null = null;
let saveSnapBtnEl: HTMLButtonElement | null = null;
let loadSnapBtnEl: HTMLButtonElement | null = null;
let delSnapBtnEl: HTMLButtonElement | null = null;
let listSnapBtnEl: HTMLButtonElement | null = null;
let monitorCmdInputEl: HTMLInputElement | null = null;
let sendMonitorBtnEl: HTMLButtonElement | null = null;

// Workspace Tabs Elements
let tabEditorBtnEl: HTMLButtonElement | null = null;
let tabLogBtnEl: HTMLButtonElement | null = null;
let tabHexBtnEl: HTMLButtonElement | null = null;
let editorContentAreaEl: HTMLElement | null = null;
let logContentAreaEl: HTMLElement | null = null;
let hexContentAreaEl: HTMLElement | null = null;
let editorContainerEl: HTMLElement | null = null;
let editorFileTitleEl: HTMLElement | null = null;
let editorDirtyIndicatorEl: HTMLElement | null = null;
let saveFileBtnEl: HTMLButtonElement | null = null;
let fileExplorerListEl: HTMLElement | null = null;

let hexTitleTextEl: HTMLElement | null = null;
let hexStatusBannerContainerEl: HTMLElement | null = null;
let hexDumpOutputEl: HTMLElement | null = null;

interface FileNode {
  name: string;
  path: string;
  is_dir: boolean;
  is_binary: boolean;
}

// Inspector Tabs & Panels
let inspectorTabRegistersBtnEl: HTMLButtonElement | null = null;
let inspectorTabStackBtnEl: HTMLButtonElement | null = null;
let inspectorTabMemoryBtnEl: HTMLButtonElement | null = null;
let registersTabContentEl: HTMLElement | null = null;
let stackTabContentEl: HTMLElement | null = null;
let memoryTabContentEl: HTMLElement | null = null;

let memoryAddressInputEl: HTMLInputElement | null = null;
let queryMemoryBtnEl: HTMLButtonElement | null = null;
let registersGridEl: HTMLElement | null = null;
let flagsContainerEl: HTMLElement | null = null;
let stackAddressTextEl: HTMLElement | null = null;
let stackViewerEl: HTMLElement | null = null;
let memoryViewerEl: HTMLElement | null = null;
let disasmViewerEl: HTMLElement | null = null;

// Mock bootloader instructions table matching standard real-mode BIOS assembly
const bootloaderInstructions = [
  { addr: 0x7c00, bytes: "fa", asm: "cli" },
  { addr: 0x7c01, bytes: "31 c0", asm: "xor ax, ax" },
  { addr: 0x7c03, bytes: "8e d8", asm: "mov ds, ax" },
  { addr: 0x7c05, bytes: "8e c0", asm: "mov es, ax" },
  { addr: 0x7c07, bytes: "8e d0", asm: "mov ss, ax" },
  { addr: 0x7c09, bytes: "bc 00 90", asm: "mov sp, 0x9000" },
  { addr: 0x7c0c, bytes: "fb", asm: "sti" },
  { addr: 0x7c0d, bytes: "be 10 7c", asm: "mov si, 0x7c10" },
  { addr: 0x7c10, bytes: "ac", asm: "lodsb" },
  { addr: 0x7c11, bytes: "08 c0", asm: "or al, al" },
  { addr: 0x7c13, bytes: "74 09", asm: "jz 0x7c1e" },
  { addr: 0x7c15, bytes: "b4 0e", asm: "mov ah, 0x0e" },
  { addr: 0x7c17, bytes: "bb 07 00", asm: "mov bx, 0x0007" },
  { addr: 0x7c1a, bytes: "cd 10", asm: "int 0x10" },
  { addr: 0x7c1c, bytes: "eb f2", asm: "jmp 0x7c10" },
  { addr: 0x7c1e, bytes: "f4", asm: "hlt" },
  { addr: 0x7c1f, bytes: "eb fc", asm: "jmp 0x7c1f" }
];

function getDereferenceChain(regName: string, regVal: string): string {
  const cleanVal = regVal.replace("0x", "");
  const addr = parseInt(cleanVal, 16);
  if (isNaN(addr)) return "";

  if (regName === "EIP" || regName === "PC") {
    const instr = bootloaderInstructions.find(i => i.addr === addr);
    return instr ? `➔ ${instr.asm}` : "";
  }

  if (addr === 0x7c00) {
    return "➔ 0xaa55ebfe ➔ [boot signature]";
  }

  if (addr >= 0x7c10 && addr <= 0x7c1f) {
    const instr = bootloaderInstructions.find(i => i.addr === addr);
    return instr ? `➔ 0x${instr.bytes.replace(" ", "")} ➔ ("${instr.asm}")` : "";
  }

  if (addr === 0x9000 || (addr >= 0x8f00 && addr <= 0x9000)) {
    return "➔ 0x00000000 ➔ [stack base]";
  }

  if (addr > 0 && addr < 0x10000) {
    return `➔ 0x${(addr * 2).toString(16).padStart(8, '0')} ➔ [mem pointer]`;
  }

  return "";
}

function updateDisasmUI() {
  if (!disasmViewerEl) return;

  if (currentStatus === 'disconnected') {
    disasmViewerEl.innerHTML = '<span style="color: #ef4444;">Debugger is disconnected.</span>';
    return;
  }

  const eipValStr = lastRegisterValues['EIP'] || "0x00007c00";
  const eipVal = parseInt(eipip(eipValStr), 16) || 0x7c00;

  function eipip(val: string) {
    return val.replace("0x", "");
  }

  let activeIndex = bootloaderInstructions.findIndex(i => i.addr === eipVal);
  if (activeIndex === -1) {
    disasmViewerEl.innerHTML = `<span style="color: #f9e2af;">0x${eipVal.toString(16).padStart(8, '0')}   ??   [unknown instruction]</span>`;
    return;
  }

  const start = Math.max(0, activeIndex - 3);
  const end = Math.min(bootloaderInstructions.length, activeIndex + 4);

  let disasmHtml = "";
  for (let i = start; i < end; i++) {
    const instr = bootloaderInstructions[i];
    const isCurrent = i === activeIndex;
    const prefix = isCurrent ? "➔ " : "  ";
    const addrStr = `0x${instr.addr.toString(16).padStart(8, '0')}`;
    const bytesStr = instr.bytes.padEnd(10, ' ');
    const lineText = `${prefix}${addrStr}   ${bytesStr}   ${instr.asm}`;
    
    if (isCurrent) {
      disasmHtml += `<span style="color: #00D4FF; font-weight: bold;">${lineText}</span>\n`;
    } else {
      disasmHtml += `<span style="color: var(--text-secondary);">${lineText}</span>\n`;
    }
  }

  disasmViewerEl.innerHTML = disasmHtml;
}

// Terminal State
let term: Terminal | null = null;
let fitAddon: FitAddon | null = null;
let unlistenPty: (() => void) | null = null;

// CPU Flags definition (exact match from extension/src/inspectorPanel.ts)
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

// State
let lastRegisterValues: Record<string, string> = {
  EAX: "0x00000000",
  EBX: "0x00000000",
  ECX: "0x00000000",
  EDX: "0x00000000",
  CS: "0x0000",
  DS: "0x0000",
  SS: "0x0000",
  EIP: "0x00007c00",
  ESP: "0x00009000",
  EFLAGS: "0x00000202",
};

let currentStatus: 'running' | 'stopped' | 'disconnected' = 'disconnected';

// Determine temporary workspace path depending on OS
const IS_WINDOWS = navigator.userAgent.includes("Windows") || navigator.platform.includes("Win");
const TEMP_PROJECT_ROOT = IS_WINDOWS
  ? "C:/Users/Piyush/AppData/Local/Temp/pyxforge-spike-project"
  : "/tmp/pyxforge-spike-project";

function log(message: string, type: "info" | "success" | "error" | "system" = "info") {
  if (!term) return;
  const timeStr = `\x1b[90m[${new Date().toLocaleTimeString()}]\x1b[0m`;
  let content = message;
  
  if (type === "success") {
    content = `\x1b[32m${message}\x1b[0m`;
  } else if (type === "error") {
    content = `\x1b[31m${message}\x1b[0m`;
  } else if (type === "system") {
    content = `\x1b[36m${message}\x1b[0m`;
  } else {
    content = `\x1b[34m${message}\x1b[0m`;
  }
  
  const formatted = content.replace(/\r?\n/g, "\r\n");
  term.write(`\r\n${timeStr} ${formatted}\r\n`);
}

async function sendRequest(req: Record<string, unknown>): Promise<any> {
  const reqStr = JSON.stringify(req);
  log(`Sending request: ${req.cmd}`, "info");
  
  try {
    const respStr = await invoke<string>("call_core", { requestJson: reqStr });
    const resp = JSON.parse(respStr);
    
    if (resp.status === "ok") {
      log(`Response OK: ${resp.message || "success"}`, "success");
      return resp;
    } else {
      log(`Response Error: ${resp.message || "unknown failure"}`, "error");
      throw new Error(resp.message);
    }
  } catch (err: any) {
    log(`Bridge error: ${err.message || err}`, "error");
    throw err;
  }
}

// Update application connection status UI
function setConnectionStatus(status: 'running' | 'stopped' | 'disconnected', versionText?: string) {
  currentStatus = status;
  
  // Titlebar status
  if (statusDotEl && statusTextEl) {
    if (status !== 'disconnected') {
      statusDotEl.className = "status-dot connected";
      statusTextEl.textContent = `CONNECTED ${versionText ? `(${versionText})` : ''}`;
    } else {
      statusDotEl.className = "status-dot";
      statusTextEl.textContent = "DISCONNECTED";
    }
  }

  // CPU Inspector status badge
  if (statusBadgeEl && inspectorStatusTextEl) {
    statusBadgeEl.className = `status-badge ${status}`;
    inspectorStatusTextEl.textContent = status;
  }

  // Explain Button visibility (stopped only, matching extension logic)
  if (explainCpuBtnEl) {
    explainCpuBtnEl.style.display = status === 'stopped' ? 'block' : 'none';
  }

  updateDisasmUI();
}

async function pingBackend() {
  try {
    const resp = await sendRequest({ cmd: "ping" });
    setConnectionStatus('stopped', `v${resp.version || "0.1.0"}`);
    updateRegistersUI();
  } catch {
    setConnectionStatus('disconnected');
    updateRegistersUI();
  }
}

async function initializeProject() {
  const name = projNameInputEl?.value || "my-custom-os";
  try {
    await sendRequest({
      cmd: "init",
      projectRoot: TEMP_PROJECT_ROOT,
      projectName: name,
    });
    log(`Scaffold successfully initialized at: ${TEMP_PROJECT_ROOT}`, "success");
  } catch (err: any) {
    log(`Scaffold initialization failed: ${err.message}`, "error");
  }
}

async function fetchProfiles() {
  try {
    const resp = await sendRequest({
      cmd: "listProfiles",
      projectRoot: TEMP_PROJECT_ROOT,
    });
    
    const profiles = resp.data?.profiles || [];
    log(`Fetched ${profiles.length} build profiles.`, "success");
    
    if (presetListEl) {
      const listEl = presetListEl;
      listEl.innerHTML = "";
      profiles.forEach((profile: any) => {
        const card = document.createElement("div");
        card.className = "preset-card";
        card.innerHTML = `
          <div class="preset-name">${profile.name}</div>
          <div class="preset-desc">${profile.description || `Tool: ${profile.tool}`}</div>
        `;
        card.addEventListener("click", () => buildProfile(profile.name));
        listEl.appendChild(card);
      });
    }
  } catch (err: any) {
    log(`Failed to fetch profiles: ${err.message}. Make sure to initialize the project first.`, "error");
  }
}

async function buildProfile(profileName: string) {
  log(`Triggering compile build for profile: ${profileName}...`, "info");
  try {
    const resp = await sendRequest({
      cmd: "build",
      profile: profileName,
      projectRoot: TEMP_PROJECT_ROOT,
    });
    
    const buildData = resp.data;
    if (buildData) {
      if (buildData.stdout) log(buildData.stdout, "info");
      if (buildData.stderr) log(buildData.stderr, "error");
      log(`Build completed with exit code: ${buildData.exit_code}`, buildData.exit_code === 0 ? "success" : "error");
    }
  } catch (err: any) {
    log(`Build execution failed: ${err.message}`, "error");
  }
}

// QEMU Snapshot & Monitor Console Handlers
async function saveQemuSnapshot() {
  const tag = snapshotTagInputEl?.value || "snap1";
  log(`QEMU: Saving snapshot with tag: '${tag}'...`, "info");
  try {
    const resp = await sendRequest({
      cmd: "qemuSnapshotSave",
      projectRoot: TEMP_PROJECT_ROOT,
      tag,
    });
    if (resp.data && resp.data.output) {
      log(resp.data.output, "success");
    } else {
      log("Snapshot saved successfully", "success");
    }
  } catch (err: any) {
    log(`Failed to save snapshot: ${err.message}`, "error");
  }
}

async function loadQemuSnapshot() {
  const tag = snapshotTagInputEl?.value || "snap1";
  log(`QEMU: Loading snapshot with tag: '${tag}'...`, "info");
  try {
    const resp = await sendRequest({
      cmd: "qemuSnapshotLoad",
      projectRoot: TEMP_PROJECT_ROOT,
      tag,
    });
    if (resp.data && resp.data.output) {
      log(resp.data.output, "success");
    } else {
      log("Snapshot loaded successfully", "success");
    }
  } catch (err: any) {
    log(`Failed to load snapshot: ${err.message}`, "error");
  }
}

async function deleteQemuSnapshot() {
  const tag = snapshotTagInputEl?.value || "snap1";
  log(`QEMU: Deleting snapshot with tag: '${tag}'...`, "info");
  try {
    const resp = await sendRequest({
      cmd: "qemuSnapshotDelete",
      projectRoot: TEMP_PROJECT_ROOT,
      tag,
    });
    if (resp.data && resp.data.output) {
      log(resp.data.output, "success");
    } else {
      log("Snapshot deleted successfully", "success");
    }
  } catch (err: any) {
    log(`Failed to delete snapshot: ${err.message}`, "error");
  }
}

async function listQemuSnapshots() {
  log("QEMU: Fetching active VM snapshots list...", "info");
  try {
    const resp = await sendRequest({
      cmd: "qemuSnapshotList",
      projectRoot: TEMP_PROJECT_ROOT,
    });
    if (resp.data && resp.data.output) {
      log(resp.data.output, "info");
    } else {
      log("No snapshots found or info command empty", "info");
    }
  } catch (err: any) {
    log(`Failed to list snapshots: ${err.message}`, "error");
  }
}

async function sendQemuMonitorCommand() {
  const command = monitorCmdInputEl?.value || "";
  if (!command) return;
  log(`QEMU Monitor: Sending command '${command}'...`, "info");
  try {
    const resp = await sendRequest({
      cmd: "qemuMonitorCommand",
      projectRoot: TEMP_PROJECT_ROOT,
      command,
    });
    if (resp.data && resp.data.output) {
      log(resp.data.output, "info");
    } else {
      log("Command executed with no output text", "success");
    }
    if (monitorCmdInputEl) {
      monitorCmdInputEl.value = "";
    }
  } catch (err: any) {
    log(`Monitor command execution failed: ${err.message}`, "error");
  }
}

// Plugin SDK Dynamic Runtime Loader
async function loadPlugin() {
  const path = pluginPathInputEl?.value || "plugins/optimizer-plugin.js";
  log(`Plugin SDK: Attempting to load plugin from path '${path}'...`, "info");

  try {
    const code = await invoke<string>("read_plugin_file", { path });
    
    const context: PluginContext = {
      log: (msg, type = 'info') => log(`[Plugin] ${msg}`, type),
      registerButton: (id, label, onClick) => {
        if (!pluginExtensionsContainerEl) return;
        
        const existing = document.getElementById(id);
        if (existing) {
          existing.remove();
        }

        const button = document.createElement("button");
        button.id = id;
        button.className = "btn btn-secondary";
        button.innerText = label;
        button.style.fontSize = "0.75rem";
        button.style.padding = "4px 6px";
        button.style.marginTop = "4px";
        button.style.display = "block";
        button.style.width = "100%";
        button.addEventListener("click", onClick);
        pluginExtensionsContainerEl.appendChild(button);
        log(`Plugin SDK: Registered UI action button '${label}'`, "success");
      },
      sendRequest: (req) => sendRequest(req),
    };

    const initializer = new Function("ctx", code);
    initializer(context);

    loadedPlugins.set(path, { code });
    log(`Plugin SDK: Plugin at '${path}' successfully loaded and activated!`, "success");

  } catch (err: any) {
    log(`Plugin SDK: Failed to load plugin: ${err.message || err}`, "error");
  }
}



// Switch workspace tabs
function switchWorkspaceTab(activeTab: 'editor' | 'log' | 'hex') {
  if (tabEditorBtnEl && tabLogBtnEl && tabHexBtnEl && editorContentAreaEl && logContentAreaEl && hexContentAreaEl) {
    tabEditorBtnEl.classList.remove('active');
    tabLogBtnEl.classList.remove('active');
    tabHexBtnEl.classList.remove('active');

    editorContentAreaEl.style.display = 'none';
    logContentAreaEl.style.display = 'none';
    hexContentAreaEl.style.display = 'none';

    if (activeTab === 'editor') {
      tabEditorBtnEl.classList.add('active');
      editorContentAreaEl.style.display = 'flex';
    } else if (activeTab === 'log') {
      tabLogBtnEl.classList.add('active');
      logContentAreaEl.style.display = 'flex';
    } else {
      tabHexBtnEl.classList.add('active');
      hexContentAreaEl.style.display = 'flex';
    }
  }
}

async function refreshWorkspaceFiles() {
  if (!fileExplorerListEl) return;
  try {
    const files = await invoke<FileNode[]>("list_workspace_files", { dirPath: null });
    fileExplorerListEl.innerHTML = "";

    if (!files || files.length === 0) {
      fileExplorerListEl.innerHTML = `<div style="color: var(--text-tertiary); font-size: 0.8rem; padding: 4px;">No workspace files found.</div>`;
      return;
    }

    for (const node of files) {
      const card = document.createElement("div");
      card.className = "preset-card file-item";
      card.dataset.filename = node.name;
      card.dataset.filepath = node.path;
      card.dataset.isBinary = node.is_binary ? "true" : "false";

      const iconSvg = node.is_dir
        ? `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align: middle; margin-right: 4px;"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L8.6 3.3A2 2 0 0 0 6.9 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>`
        : `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align: middle; margin-right: 4px;"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><polyline points="14 2 14 8 20 8"/></svg>`;

      const badgeStr = node.is_binary ? `<span style="font-size: 0.7rem; color: var(--text-tertiary); margin-left: 6px;">[BIN]</span>` : "";

      card.innerHTML = `
        <div class="preset-name">${iconSvg} ${node.name} ${badgeStr}</div>
        <div class="preset-desc" style="font-size: 0.75rem; color: var(--text-tertiary); word-break: break-all;">${node.path}</div>
      `;

      card.addEventListener("click", () => openWorkspaceFile(node));
      fileExplorerListEl.appendChild(card);
    }
  } catch (err: any) {
    log(`Failed to list workspace files: ${err.message || err}`, "error");
  }
}

async function openWorkspaceFile(node: FileNode) {
  if (node.is_dir) return;

  if (node.is_binary) {
    await loadBinaryHexDump(node.path, node.name);
    return;
  }

  try {
    const text = await invoke<string>("read_workspace_file", { filePath: node.path });
    codeEditor.openFile(node.path, text);
    if (editorFileTitleEl) editorFileTitleEl.textContent = `Code Editor: ${node.name}`;
    switchWorkspaceTab('editor');
    log(`Opened workspace file: ${node.name}`, "info");
  } catch (err: any) {
    if (err === "ERR_BINARY_FILE") {
      await loadBinaryHexDump(node.path, node.name);
    } else {
      log(`Failed to read file '${node.name}': ${err.message || err}`, "error");
    }
  }
}

async function loadBinaryHexDump(path: string, fileName: string) {
  try {
    const request = JSON.stringify({ jsonrpc: "2.0", method: "HexDump", params: { file_path: path }, id: 1 });
    const responseStr = await invoke<string>("call_core", { requestJson: request });
    const res = JSON.parse(responseStr);
    if (res.result && res.result.dump) {
      if (hexDumpOutputEl) hexDumpOutputEl.textContent = res.result.dump;
      if (hexTitleTextEl) hexTitleTextEl.textContent = `Hex: ${fileName}`;
      if (hexStatusBannerContainerEl) {
        hexStatusBannerContainerEl.innerHTML = `<div style="background: var(--accent-dim); border: 1px solid var(--accent-border); color: var(--accent); padding: 6px 10px; border-radius: var(--radius-sm); font-size: 0.8rem;">Loaded real binary hex dump from disk via HexDump RPC</div>`;
      }
      switchWorkspaceTab('hex');
      log(`Hex Viewer loaded binary dump for '${fileName}' via HexDump RPC`, "success");
    } else {
      log(`HexDump RPC returned error for '${fileName}'`, "error");
    }
  } catch (err: any) {
    log(`Failed to execute HexDump RPC for '${fileName}': ${err.message || err}`, "error");
  }
}

async function saveCurrentFile() {
  const path = codeEditor.getCurrentPath();
  if (!path) {
    log("No open file to save.", "system");
    return;
  }

  try {
    const content = codeEditor.getContent();
    await invoke("write_workspace_file", { filePath: path, content });
    codeEditor.markClean();
    log(`Saved workspace file: ${path}`, "success");
    await refreshWorkspaceFiles();
  } catch (err: any) {
    log(`Failed to save file '${path}': ${err.message || err}`, "error");
  }
}

// Switch CPU inspector tabs
function switchInspectorTab(activeTab: 'registers' | 'stack' | 'memory') {
  if (inspectorTabRegistersBtnEl && inspectorTabStackBtnEl && inspectorTabMemoryBtnEl &&
      registersTabContentEl && stackTabContentEl && memoryTabContentEl) {
    
    inspectorTabRegistersBtnEl.classList.remove('active');
    inspectorTabStackBtnEl.classList.remove('active');
    inspectorTabMemoryBtnEl.classList.remove('active');
    registersTabContentEl.style.display = 'none';
    stackTabContentEl.style.display = 'none';
    memoryTabContentEl.style.display = 'none';

    if (activeTab === 'registers') {
      inspectorTabRegistersBtnEl.classList.add('active');
      registersTabContentEl.style.display = 'block';
    } else if (activeTab === 'stack') {
      inspectorTabStackBtnEl.classList.add('active');
      stackTabContentEl.style.display = 'block';
      updateStackUI();
    } else {
      inspectorTabMemoryBtnEl.classList.add('active');
      memoryTabContentEl.style.display = 'block';
      updateMemoryUI();
    }
  }
}





// Ported CPU Register Inspector updates (matches extension/src/inspectorPanel.ts logic)
function updateRegistersUI() {
  const gridEl = registersGridEl;
  const flagsEl = flagsContainerEl;
  if (!gridEl || !flagsEl) return;

  if (currentStatus === 'disconnected') {
    gridEl.innerHTML = '<div class="info-text" style="color: #ef4444; font-size: 0.85rem;">Debugger disconnected</div>';
    flagsEl.innerHTML = '';
    updateDisasmUI();
    return;
  }

  // Build grid
  gridEl.innerHTML = '';
  let eflagsVal = 0;

  const registers: Register[] = Object.keys(lastRegisterValues).map(name => {
    return {
      name,
      value: lastRegisterValues[name]
    };
  });

  registers.forEach(reg => {
    const row = document.createElement('div');
    row.className = 'register-row';
    
    const nameSpan = document.createElement('span');
    nameSpan.className = 'register-name';
    nameSpan.innerText = reg.name;

    const detailsDiv = document.createElement('div');
    detailsDiv.className = 'register-details';

    const valSpan = document.createElement('span');
    valSpan.className = 'register-value';
    valSpan.innerText = reg.value;
    
    // Check if value changed to apply flash-animation style
    const oldVal = lastRegisterValues[reg.name];
    if (oldVal !== reg.value) {
      valSpan.className = 'register-value changed';
    }

    detailsDiv.appendChild(valSpan);

    // Build and append dereference chain if applicable
    const chain = getDereferenceChain(reg.name, reg.value);
    if (chain) {
      const chainSpan = document.createElement('span');
      chainSpan.className = 'register-chain';
      chainSpan.innerText = chain;
      detailsDiv.appendChild(chainSpan);
    }

    row.appendChild(nameSpan);
    row.appendChild(detailsDiv);
    gridEl.appendChild(row);

    if (reg.name === 'EFLAGS') {
      eflagsVal = parseInt(reg.value, 16) || 0;
    }
  });

  // Decompose flags (ZF, CF, SF etc. flags UI)
  flagsEl.innerHTML = '';
  if (eflagsVal !== 0) {
    cpuFlags.forEach(flag => {
      const active = (eflagsVal & flag.mask) !== 0;
      const badge = document.createElement('div');
      badge.className = `flag-badge${active ? ' active' : ''}`;
      badge.title = flag.desc;
      badge.innerText = `${flag.name}: ${active ? '1' : '0'}`;
      flagsEl.appendChild(badge);
    });
  }

  updateDisasmUI();
}

function updateStackUI() {
  if (!stackViewerEl || !stackAddressTextEl) return;

  if (currentStatus === 'disconnected') {
    stackViewerEl.innerText = 'Target is disconnected.';
    stackAddressTextEl.innerText = 'Stack Pointer address: N/A';
    return;
  }

  const espVal = lastRegisterValues['ESP'] || '0x00009000';
  stackAddressTextEl.innerText = `Stack Pointer address: ${espVal}`;

  // Render stack grid layout
  let stackHtml = '';
  const baseAddr = parseInt(espVal, 16);
  for (let i = 0; i < 4; i++) {
    const offset = baseAddr + (i * 16);
    const hex = [];
    let ascii = '';
    for (let b = 0; b < 16; b++) {
      const byteVal = (offset + b) % 256;
      hex.push(byteVal.toString(16).padStart(2, '0').toUpperCase());
      ascii += (byteVal >= 32 && byteVal <= 126) ? String.fromCharCode(byteVal) : '.';
    }
    stackHtml += `${offset.toString(16).padStart(8, '0')}  ${hex.join(' ')}  |${ascii}|\n`;
  }
  stackViewerEl.innerText = stackHtml;
}

function updateMemoryUI() {
  if (!memoryViewerEl) return;

  if (currentStatus === 'disconnected') {
    memoryViewerEl.innerText = 'Target is disconnected.';
    return;
  }

  const addrStr = memoryAddressInputEl?.value || '0x7c00';
  
  // Render memory grid layout
  let memHtml = '';
  const baseAddr = parseInt(addrStr, 16) || 0x7c00;
  for (let i = 0; i < 6; i++) {
    const offset = baseAddr + (i * 16);
    const hex = [];
    let ascii = '';
    for (let b = 0; b < 16; b++) {
      const byteVal = (baseAddr + offset + b) % 256;
      hex.push(byteVal.toString(16).padStart(2, '0').toUpperCase());
      ascii += (byteVal >= 32 && byteVal <= 126) ? String.fromCharCode(byteVal) : '.';
    }
    memHtml += `${offset.toString(16).padStart(8, '0')}  ${hex.join(' ')}  |${ascii}|\n`;
  }
  memoryViewerEl.innerText = memHtml;
}

let stepCount = 0;
function simulateRegisterChange() {
  if (currentStatus === 'disconnected') {
    setConnectionStatus('stopped');
  }

  stepCount++;
  log("Simulating CPU step and register updates...", "system");
  
  const hex = (val: number, digits: number) => {
    return "0x" + val.toString(16).padStart(digits, "0");
  };

  const oldEflags = parseInt(lastRegisterValues['EFLAGS'], 16);
  const newEflags = "0x" + (oldEflags ^ 0x0040).toString(16).padStart(8, "0");

  lastRegisterValues['EAX'] = hex(0x10 + stepCount, 8);
  lastRegisterValues['EBX'] = hex(0x20 + stepCount, 8);
  lastRegisterValues['ECX'] = hex(0x30 + stepCount, 8);
  lastRegisterValues['EDX'] = hex(0x40 + stepCount, 8);
  lastRegisterValues['CS'] = hex(0x08, 4);
  lastRegisterValues['DS'] = hex(0x10, 4);
  lastRegisterValues['SS'] = hex(0x10, 4);
  
  const currentEip = parseInt(lastRegisterValues['EIP'], 16);
  lastRegisterValues['EIP'] = "0x" + (currentEip + 4).toString(16).padStart(8, "0");
  lastRegisterValues['ESP'] = hex(0x7c00 - (stepCount * 4), 8);
  lastRegisterValues['EFLAGS'] = newEflags;

  updateRegistersUI();
  updateStackUI();
  updateMemoryUI();
}

function explainCpuState() {
  log("Streaming inline CPU registers description from AI assistance...", "info");
  log("EIP: Points to the next execution instruction. EAX: Used as primary accumulator.", "success");
}

async function spawnPtySession() {
  if (!term) return;
  term.write("\r\n\x1b[33m[SYSTEM] Spawning interactive PTY shell...\x1b[0m\r\n");
  try {
    if (unlistenPty) {
      unlistenPty();
      unlistenPty = null;
    }
    
    unlistenPty = await listen<string>("pty-data", (event) => {
      term?.write(event.payload);
    });

    await invoke("spawn_pty", { rows: term.rows, cols: term.cols });
    term.write("\x1b[32m[SYSTEM] Shell spawned successfully.\x1b[0m\r\n\r\n");
  } catch (err: any) {
    term.write(`\r\n\x1b[31m[SYSTEM] Failed to spawn PTY: ${err.message || err}\x1b[0m\r\n`);
  }
}

// Bootstrap Event Listeners
window.addEventListener("DOMContentLoaded", () => {
  statusDotEl = document.querySelector("#status-dot");
  statusTextEl = document.querySelector("#status-text");
  statusBadgeEl = document.querySelector("#statusBadge");
  inspectorStatusTextEl = document.querySelector("#statusText");
  
  pingBtnEl = document.querySelector("#ping-btn");
  initBtnEl = document.querySelector("#init-btn");
  listProfilesBtnEl = document.querySelector("#list-profiles-btn");
  spawnShellBtnEl = document.querySelector("#spawn-shell-btn");
  
  stepBtnEl = document.querySelector("#step-btn");
  explainCpuBtnEl = document.querySelector("#explainCpuBtn");
  projNameInputEl = document.querySelector("#proj-name-input");
  presetListEl = document.querySelector("#preset-list");

  // Tabs selectors
  tabEditorBtnEl = document.querySelector("#tab-editor-btn");
  tabLogBtnEl = document.querySelector("#tab-log-btn");
  tabHexBtnEl = document.querySelector("#tab-hex-btn");
  editorContentAreaEl = document.querySelector("#editor-content-area");
  logContentAreaEl = document.querySelector("#log-content-area");
  hexContentAreaEl = document.querySelector("#hex-content-area");
  editorContainerEl = document.querySelector("#editor-container");
  editorFileTitleEl = document.querySelector("#editor-file-title");
  editorDirtyIndicatorEl = document.querySelector("#editor-dirty-indicator");
  saveFileBtnEl = document.querySelector("#save-file-btn");
  fileExplorerListEl = document.querySelector("#file-explorer-list");

  hexTitleTextEl = document.querySelector("#hex-title-text");
  hexStatusBannerContainerEl = document.querySelector("#hex-status-banner-container");
  hexDumpOutputEl = document.querySelector("#hex-dump-output");

  // Mount CodeMirror editor
  if (editorContainerEl) {
    codeEditor.mount(editorContainerEl, (dirty) => {
      if (editorDirtyIndicatorEl) {
        editorDirtyIndicatorEl.style.display = dirty ? "inline" : "none";
      }
    });
  }

  // Inspector tabs
  inspectorTabRegistersBtnEl = document.querySelector("#inspector-tab-registers-btn");
  inspectorTabStackBtnEl = document.querySelector("#inspector-tab-stack-btn");
  inspectorTabMemoryBtnEl = document.querySelector("#inspector-tab-memory-btn");
  registersTabContentEl = document.querySelector("#inspector-registers-tab-content");
  stackTabContentEl = document.querySelector("#inspector-stack-tab-content");
  memoryTabContentEl = document.querySelector("#inspector-memory-tab-content");

  memoryAddressInputEl = document.querySelector("#memoryAddressInput");
  queryMemoryBtnEl = document.querySelector("#queryMemoryBtn");
  registersGridEl = document.querySelector("#registersGrid");
  flagsContainerEl = document.querySelector("#flagsContainer");
  stackAddressTextEl = document.querySelector("#stackAddressText");
  stackViewerEl = document.querySelector("#stackViewer");
  memoryViewerEl = document.querySelector("#memoryViewer");
  disasmViewerEl = document.querySelector("#disasmViewer");

  snapshotTagInputEl = document.querySelector("#snapshot-tag-input");
  saveSnapBtnEl = document.querySelector("#save-snap-btn");
  loadSnapBtnEl = document.querySelector("#load-snap-btn");
  delSnapBtnEl = document.querySelector("#del-snap-btn");
  listSnapBtnEl = document.querySelector("#list-snap-btn");
  monitorCmdInputEl = document.querySelector("#monitor-cmd-input");
  sendMonitorBtnEl = document.querySelector("#send-monitor-btn");

  pluginPathInputEl = document.querySelector("#plugin-path-input");
  loadPluginBtnEl = document.querySelector("#load-plugin-btn");
  pluginExtensionsContainerEl = document.querySelector("#plugin-extensions-container");

  // Initialize xterm.js Terminal
  const termContainer = document.getElementById("terminal-container");
  if (termContainer) {
    term = new Terminal({
      cursorBlink: true,
      theme: {
        background: "#020408",
        foreground: "#e2e8f0",
        cursor: "#cbd5e1",
        selectionBackground: "#334155",
        black: "#1e1e2e",
        red: "#f38ba8",
        green: "#a6e3a1",
        yellow: "#f9e2af",
        blue: "#89b4fa",
        magenta: "#f5c2e7",
        cyan: "#89dceb",
        white: "#cdd6f4",
      },
      fontFamily: "'JetBrains Mono', Consolas, monospace",
      fontSize: 13,
      lineHeight: 1.2,
    });

    fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(termContainer);
    fitAddon.fit();

    // Send frontend keystrokes back to PTY writer
    term.onData((data) => {
      invoke("write_to_pty", { data }).catch(err => {
        console.error("PTY Write error:", err);
      });
    });

    // Handle resizing
    window.addEventListener("resize", () => {
      if (fitAddon && term) {
        fitAddon.fit();
        invoke("resize_pty", { rows: term.rows, cols: term.cols }).catch(() => {});
      }
    });
  }

  // Wire Listeners
  pingBtnEl?.addEventListener("click", pingBackend);
  initBtnEl?.addEventListener("click", initializeProject);
  listProfilesBtnEl?.addEventListener("click", fetchProfiles);
  stepBtnEl?.addEventListener("click", simulateRegisterChange);
  explainCpuBtnEl?.addEventListener("click", explainCpuState);
  spawnShellBtnEl?.addEventListener("click", spawnPtySession);

  saveSnapBtnEl?.addEventListener("click", saveQemuSnapshot);
  loadSnapBtnEl?.addEventListener("click", loadQemuSnapshot);
  delSnapBtnEl?.addEventListener("click", deleteQemuSnapshot);
  listSnapBtnEl?.addEventListener("click", listQemuSnapshots);
  sendMonitorBtnEl?.addEventListener("click", sendQemuMonitorCommand);
  loadPluginBtnEl?.addEventListener("click", loadPlugin);

  monitorCmdInputEl?.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      sendQemuMonitorCommand();
    }
  });



  // Workspace tab triggers
  tabEditorBtnEl?.addEventListener("click", () => switchWorkspaceTab('editor'));
  tabLogBtnEl?.addEventListener("click", () => switchWorkspaceTab('log'));
  tabHexBtnEl?.addEventListener("click", () => switchWorkspaceTab('hex'));

  // Inspector tab triggers
  inspectorTabRegistersBtnEl?.addEventListener("click", () => switchInspectorTab('registers'));
  inspectorTabStackBtnEl?.addEventListener("click", () => switchInspectorTab('stack'));
  inspectorTabMemoryBtnEl?.addEventListener("click", () => switchInspectorTab('memory'));

  queryMemoryBtnEl?.addEventListener("click", updateMemoryUI);

  saveFileBtnEl?.addEventListener("click", saveCurrentFile);

  window.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      saveCurrentFile();
    }
  });

  // Initialize UI displays and load workspace files
  setConnectionStatus('disconnected');
  updateRegistersUI();
  refreshWorkspaceFiles();

  // Try initial backend ping and spawn shell session
  setTimeout(() => {
    pingBackend();
    spawnPtySession();
  }, 500);
});
