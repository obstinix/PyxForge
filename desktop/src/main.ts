import { invoke } from "@tauri-apps/api/core";

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
let clearLogBtnEl: HTMLButtonElement | null = null;
let stepBtnEl: HTMLButtonElement | null = null;
let explainCpuBtnEl: HTMLButtonElement | null = null;
let projNameInputEl: HTMLInputElement | null = null;
let presetListEl: HTMLElement | null = null;
let consoleLogEl: HTMLElement | null = null;
let themeSelectorEl: HTMLSelectElement | null = null;

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
  if (consoleLogEl) {
    const entry = document.createElement("div");
    entry.className = "log-entry";
    
    const timeSpan = document.createElement("span");
    timeSpan.className = "log-time";
    timeSpan.textContent = `[${new Date().toLocaleTimeString()}]`;
    
    const contentSpan = document.createElement("span");
    contentSpan.className = `log-${type}`;
    contentSpan.textContent = message;
    
    entry.appendChild(timeSpan);
    entry.appendChild(contentSpan);
    consoleLogEl.appendChild(entry);
    consoleLogEl.scrollTop = consoleLogEl.scrollHeight;
  }
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

// Switch themes dynamically
function switchTheme(theme: string) {
  document.documentElement.setAttribute('data-theme', theme);
  log(`UI Theme switched to: ${theme}`, 'system');
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
    const box = document.createElement('div');
    box.className = 'register-box';
    
    // Check if value changed to apply flash-animation style
    const oldVal = lastRegisterValues[reg.name];
    if (oldVal !== reg.value) {
      box.className = 'register-box changed';
      setTimeout(() => {
        box.className = 'register-box';
      }, 1500);
    }

    const nameSpan = document.createElement('span');
    nameSpan.className = 'register-name';
    nameSpan.innerText = reg.name;

    const valSpan = document.createElement('span');
    valSpan.className = 'register-val';
    valSpan.innerText = reg.value;

    box.appendChild(nameSpan);
    box.appendChild(valSpan);
    gridEl.appendChild(box);

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

// Bootstrap Event Listeners
window.addEventListener("DOMContentLoaded", () => {
  statusDotEl = document.querySelector("#status-dot");
  statusTextEl = document.querySelector("#status-text");
  statusBadgeEl = document.querySelector("#statusBadge");
  inspectorStatusTextEl = document.querySelector("#statusText");
  
  pingBtnEl = document.querySelector("#ping-btn");
  initBtnEl = document.querySelector("#init-btn");
  listProfilesBtnEl = document.querySelector("#list-profiles-btn");
  clearLogBtnEl = document.querySelector("#clear-log-btn");
  
  stepBtnEl = document.querySelector("#step-btn");
  explainCpuBtnEl = document.querySelector("#explainCpuBtn");
  projNameInputEl = document.querySelector("#proj-name-input");
  presetListEl = document.querySelector("#preset-list");
  consoleLogEl = document.querySelector("#console-log");
  themeSelectorEl = document.querySelector("#theme-selector");

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

  // Wire Listeners
  pingBtnEl?.addEventListener("click", pingBackend);
  initBtnEl?.addEventListener("click", initializeProject);
  listProfilesBtnEl?.addEventListener("click", fetchProfiles);
  stepBtnEl?.addEventListener("click", simulateRegisterChange);
  explainCpuBtnEl?.addEventListener("click", explainCpuState);

  // Theme support
  themeSelectorEl?.addEventListener("change", (e) => {
    const select = e.target as HTMLSelectElement;
    switchTheme(select.value);
  });

  // Workspace tab triggers
  tabLogBtnEl?.addEventListener("click", () => switchWorkspaceTab('log'));
  tabHexBtnEl?.addEventListener("click", () => switchWorkspaceTab('hex'));

  // Inspector tab triggers
  inspectorTabRegistersBtnEl?.addEventListener("click", () => switchInspectorTab('registers'));
  inspectorTabStackBtnEl?.addEventListener("click", () => switchInspectorTab('stack'));
  inspectorTabMemoryBtnEl?.addEventListener("click", () => switchInspectorTab('memory'));

  queryMemoryBtnEl?.addEventListener("click", updateMemoryUI);

  clearLogBtnEl?.addEventListener("click", () => {
    if (consoleLogEl) consoleLogEl.innerHTML = "";
  });

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

  // Try initial backend ping
  setTimeout(pingBackend, 500);
});
