import "./fonts";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

// Interfaces from original extension codebases
interface Register {
  name: string;
  value: string;
  changed?: boolean;
}

interface HexDumpLine {
  offset: number;
  hex_bytes: string[];
  ascii: string;
}

interface HexDumpData {
  is_boot_sector: boolean;
  has_boot_signature: boolean;
  file_size: number;
  lines: HexDumpLine[];
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
let tabLogBtnEl: HTMLButtonElement | null = null;
let tabHexBtnEl: HTMLButtonElement | null = null;
let logContentAreaEl: HTMLElement | null = null;
let hexContentAreaEl: HTMLElement | null = null;
let hexTitleTextEl: HTMLElement | null = null;
let hexStatusBannerContainerEl: HTMLElement | null = null;
let hexDumpOutputEl: HTMLElement | null = null;

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
      disasmHtml += `<span style="color: #cba6f7; font-weight: bold;">${lineText}</span>\n`;
    } else {
      disasmHtml += `<span style="color: #64748b;">${lineText}</span>\n`;
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
function switchWorkspaceTab(activeTab: 'log' | 'hex') {
  if (tabLogBtnEl && tabHexBtnEl && logContentAreaEl && hexContentAreaEl) {
    if (activeTab === 'log') {
      tabLogBtnEl.classList.add('active');
      tabHexBtnEl.classList.remove('active');
      logContentAreaEl.style.display = 'flex';
      hexContentAreaEl.style.display = 'none';
    } else {
      tabLogBtnEl.classList.remove('active');
      tabHexBtnEl.classList.add('active');
      logContentAreaEl.style.display = 'none';
      hexContentAreaEl.style.display = 'flex';
    }
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

// Genuinely ported hex view formatting (matches extension/src/hexPanel.ts layout exactly)
function renderHexDump(fileName: string, data: HexDumpData) {
  if (!hexTitleTextEl || !hexStatusBannerContainerEl || !hexDumpOutputEl) return;

  hexTitleTextEl.innerText = `Hex: ${fileName}`;

  // 1. Render Status Banner
  let bannerHtml = '';
  if (data.is_boot_sector) {
    if (data.has_boot_signature) {
      bannerHtml = `
        <div style="background: rgba(166, 227, 161, 0.1); border: 1px solid #a6e3a1; color: #a6e3a1; padding: 10px 14px; border-radius: 6px; display: flex; gap: 10px; align-items: center; font-size: 0.85rem;">
          <span style="font-size: 1.2rem; font-weight: bold;">✔</span>
          <div>
            <strong>Valid BIOS Boot Sector</strong>
            <div style="opacity: 0.8; font-size: 0.75rem; margin-top: 2px;">Exactly 512 bytes with bootloader signature (0xAA55) detected.</div>
          </div>
        </div>
      `;
    } else {
      bannerHtml = `
        <div style="background: rgba(249, 226, 175, 0.1); border: 1px solid #f9e2af; color: #f9e2af; padding: 10px 14px; border-radius: 6px; display: flex; gap: 10px; align-items: center; font-size: 0.85rem;">
          <span style="font-size: 1.2rem; font-weight: bold;">⚠</span>
          <div>
            <strong>Invalid Boot Sector</strong>
            <div style="opacity: 0.8; font-size: 0.75rem; margin-top: 2px;">File size is 512 bytes, but the 0xAA55 boot signature is missing! It will not boot.</div>
          </div>
        </div>
      `;
    }
  } else {
    bannerHtml = `
      <div style="background: rgba(56, 189, 248, 0.1); border: 1px solid #38bdf8; color: #38bdf8; padding: 10px 14px; border-radius: 6px; display: flex; gap: 10px; align-items: center; font-size: 0.85rem;">
        <span style="font-size: 1.2rem; font-weight: bold;">🛈</span>
        <div>
          <strong>Raw Binary File</strong>
          <div style="opacity: 0.8; font-size: 0.75rem; margin-top: 2px;">Size: ${data.file_size} bytes. (Not a standard 512-byte BIOS boot sector).</div>
        </div>
      </div>
    `;
  }
  hexStatusBannerContainerEl.innerHTML = bannerHtml;

  // 2. Render Hex Lines
  let linesHtml = '';
  data.lines.forEach((line) => {
    const offsetStr = line.offset.toString(16).padStart(8, '0');

    // Format bytes spans
    const byteSpans = line.hex_bytes.map((byte, idx) => {
      const globalIdx = line.offset + idx;
      const isSig = data.is_boot_sector && (globalIdx === 510 || globalIdx === 511);
      const style = isSig ? 'style="color: #f9e2af; font-weight: bold;"' : '';
      return `<span ${style}>${byte.toUpperCase()}</span>`;
    });

    // Group in pairs
    let byteGroups = [];
    for (let j = 0; j < byteSpans.length; j += 2) {
      byteGroups.push(byteSpans.slice(j, j + 2).join(' '));
    }
    const hexBytesStr = byteGroups.join('  ');

    // Escape ASCII characters safely
    const asciiSafe = line.ascii
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');

    linesHtml += `<div style="margin-bottom: 2px;"><span style="color: #64748b;">${offsetStr}</span>  <span>${hexBytesStr.padEnd(48, ' ')}</span>  <span style="color: #a78bfa;">|${asciiSafe}|</span></div>`;
  });

  hexDumpOutputEl.innerHTML = linesHtml;
  switchWorkspaceTab('hex');
}

// Generate structured mockup data to exercise the genuinely ported hexPanel formats
function loadMockupHexFile(fileName: string) {
  log(`Loading workspace file for hex dump: ${fileName}`, 'system');

  let isBootSector = false;
  let hasBootSignature = false;
  let fileSize = 128;
  const lines: HexDumpLine[] = [];

  if (fileName === 'boot.bin' || fileName === 'boot.asm') {
    isBootSector = true;
    hasBootSignature = true;
    fileSize = 512;
  } else if (fileName === 'invalid-boot.bin') {
    isBootSector = true;
    hasBootSignature = false;
    fileSize = 512;
  }

  // Populate mock hex lines
  const linesCount = Math.ceil(fileSize / 16);
  for (let i = 0; i < linesCount; i++) {
    const offset = i * 16;
    const hexBytes: string[] = [];
    let ascii = '';

    for (let b = 0; b < 16; b++) {
      const currentByteIndex = offset + b;
      
      // Inject boot signature 0xAA55 at indexes 510, 511 if valid boot sector
      if (isBootSector && currentByteIndex === 510 && hasBootSignature) {
        hexBytes.push('55');
        ascii += 'U';
      } else if (isBootSector && currentByteIndex === 511 && hasBootSignature) {
        hexBytes.push('aa');
        ascii += 'ª';
      } else {
        // Pseudo random bytes
        const charCode = (32 + Math.floor(Math.random() * 95));
        hexBytes.push(charCode.toString(16).padStart(2, '0'));
        ascii += String.fromCharCode(charCode);
      }
    }

    lines.push({
      offset,
      hex_bytes: hexBytes,
      ascii
    });
  }

  renderHexDump(fileName, {
    is_boot_sector: isBootSector,
    has_boot_signature: hasBootSignature,
    file_size: fileSize,
    lines
  });
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

  // Render a simulated stack grid layout
  let stackHtml = '';
  const baseAddr = parseInt(espVal, 16);
  for (let i = 0; i < 4; i++) {
    const offset = baseAddr + (i * 16);
    const hex = [];
    let ascii = '';
    for (let b = 0; b < 16; b++) {
      const byteVal = Math.floor(Math.random() * 256);
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
  
  // Render simulated memory grid layout
  let memHtml = '';
  const baseAddr = parseInt(addrStr, 16) || 0x7c00;
  for (let i = 0; i < 6; i++) {
    const offset = baseAddr + (i * 16);
    const hex = [];
    let ascii = '';
    for (let b = 0; b < 16; b++) {
      const byteVal = Math.floor(Math.random() * 256);
      hex.push(byteVal.toString(16).padStart(2, '0').toUpperCase());
      ascii += (byteVal >= 32 && byteVal <= 126) ? String.fromCharCode(byteVal) : '.';
    }
    memHtml += `${offset.toString(16).padStart(8, '0')}  ${hex.join(' ')}  |${ascii}|\n`;
  }
  memoryViewerEl.innerText = memHtml;
}

function simulateRegisterChange() {
  if (currentStatus === 'disconnected') {
    setConnectionStatus('stopped');
  }

  log("Simulating CPU step and register updates...", "system");
  
  const hex = (digits: number) => {
    let val = Math.floor(Math.random() * Math.pow(16, digits)).toString(16);
    return "0x" + val.padStart(digits, "0");
  };

  const oldEflags = parseInt(lastRegisterValues['EFLAGS'], 16);
  // Toggle some flags bits randomly
  const newEflags = "0x" + (oldEflags ^ (Math.random() > 0.5 ? 0x0040 : 0x0001)).toString(16).padStart(8, "0");

  lastRegisterValues['EAX'] = hex(8);
  lastRegisterValues['EBX'] = hex(8);
  lastRegisterValues['ECX'] = hex(8);
  lastRegisterValues['EDX'] = hex(8);
  lastRegisterValues['CS'] = hex(4);
  lastRegisterValues['DS'] = hex(4);
  lastRegisterValues['SS'] = hex(4);
  
  const currentEip = parseInt(lastRegisterValues['EIP'], 16);
  lastRegisterValues['EIP'] = "0x" + (currentEip + 4).toString(16).padStart(8, "0");
  lastRegisterValues['ESP'] = hex(8);
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
  tabLogBtnEl = document.querySelector("#tab-log-btn");
  tabHexBtnEl = document.querySelector("#tab-hex-btn");
  logContentAreaEl = document.querySelector("#log-content-area");
  hexContentAreaEl = document.querySelector("#hex-content-area");
  hexTitleTextEl = document.querySelector("#hex-title-text");
  hexStatusBannerContainerEl = document.querySelector("#hex-status-banner-container");
  hexDumpOutputEl = document.querySelector("#hex-dump-output");

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
  tabLogBtnEl?.addEventListener("click", () => switchWorkspaceTab('log'));
  tabHexBtnEl?.addEventListener("click", () => switchWorkspaceTab('hex'));

  // Inspector tab triggers
  inspectorTabRegistersBtnEl?.addEventListener("click", () => switchInspectorTab('registers'));
  inspectorTabStackBtnEl?.addEventListener("click", () => switchInspectorTab('stack'));
  inspectorTabMemoryBtnEl?.addEventListener("click", () => switchInspectorTab('memory'));

  queryMemoryBtnEl?.addEventListener("click", updateMemoryUI);

  // Virtual file click listener for Hex Viewer
  document.querySelectorAll(".file-item").forEach(item => {
    item.addEventListener("click", (e) => {
      const card = e.currentTarget as HTMLElement;
      const filename = card.getAttribute("data-filename") || "raw.bin";
      loadMockupHexFile(filename);
    });
  });

  // Initialize UI displays
  setConnectionStatus('disconnected');
  updateRegistersUI();

  // Try initial backend ping and spawn shell session
  setTimeout(() => {
    pingBackend();
    spawnPtySession();
  }, 500);
});
