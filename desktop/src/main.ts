import { invoke } from "@tauri-apps/api/core";

// DOM Elements
let statusDotEl: HTMLElement | null;
let statusTextEl: HTMLElement | null;
let pingBtnEl: HTMLButtonElement | null;
let initBtnEl: HTMLButtonElement | null;
let listProfilesBtnEl: HTMLButtonElement | null;
let clearLogBtnEl: HTMLButtonElement | null;
let stepBtnEl: HTMLButtonElement | null;
let projNameInputEl: HTMLInputElement | null;
let presetListEl: HTMLElement | null;
let consoleLogEl: HTMLElement | null;

// Registers DOM Elements
const registerEls: Record<string, HTMLElement | null> = {};

// State
let lastRegisterValues: Record<string, string> = {
  eax: "0x00000000",
  ebx: "0x00000000",
  ecx: "0x00000000",
  edx: "0x00000000",
  cs: "0x0000",
  ds: "0x0000",
  ss: "0x0000",
  eip: "0x00007c00",
  esp: "0x00009000",
  eflags: "0x00000002",
};

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

async function pingBackend() {
  try {
    const resp = await sendRequest({ cmd: "ping" });
    if (statusDotEl && statusTextEl) {
      statusDotEl.className = "status-dot connected";
      statusTextEl.textContent = `CONNECTED (v${resp.version || "0.1.0"})`;
    }
  } catch {
    if (statusDotEl && statusTextEl) {
      statusDotEl.className = "status-dot";
      statusTextEl.textContent = "DISCONNECTED";
    }
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
      presetListEl.innerHTML = "";
      profiles.forEach((profile: any) => {
        const card = document.createElement("div");
        card.className = "preset-card";
        card.innerHTML = `
          <div class="preset-name">${profile.name}</div>
          <div class="preset-desc">${profile.description || `Tool: ${profile.tool}`}</div>
        `;
        card.addEventListener("click", () => buildProfile(profile.name));
        presetListEl.appendChild(card);
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

function updateRegister(name: string, value: string) {
  const el = registerEls[name];
  if (el) {
    const oldVal = lastRegisterValues[name];
    if (oldVal !== value) {
      el.textContent = value;
      el.className = "register-value changed";
      // Clear the highlighting after animation completes
      setTimeout(() => {
        el.className = "register-value";
      }, 1000);
      lastRegisterValues[name] = value;
    }
  }
}

function simulateRegisterChange() {
  log("Simulating CPU step and register state updates...", "system");
  
  // Random hex generator
  const hex = (digits: number) => {
    let val = Math.floor(Math.random() * Math.pow(16, digits)).toString(16);
    return "0x" + val.padStart(digits, "0");
  };

  updateRegister("eax", hex(8));
  updateRegister("ebx", hex(8));
  updateRegister("ecx", hex(8));
  updateRegister("edx", hex(8));
  
  updateRegister("cs", hex(4));
  updateRegister("ds", hex(4));
  updateRegister("ss", hex(4));
  
  // Increment EIP by step
  const currentEip = parseInt(lastRegisterValues["eip"], 16);
  const nextEip = "0x" + (currentEip + Math.floor(Math.random() * 8) + 2).toString(16).padStart(8, "0");
  updateRegister("eip", nextEip);
  
  updateRegister("esp", hex(8));
}

// Bootstrap Event Listeners
window.addEventListener("DOMContentLoaded", () => {
  statusDotEl = document.querySelector("#status-dot");
  statusTextEl = document.querySelector("#status-text");
  pingBtnEl = document.querySelector("#ping-btn");
  initBtnEl = document.querySelector("#init-btn");
  listProfilesBtnEl = document.querySelector("#list-profiles-btn");
  clearLogBtnEl = document.querySelector("#clear-log-btn");
  stepBtnEl = document.querySelector("#step-btn");
  projNameInputEl = document.querySelector("#proj-name-input");
  presetListEl = document.querySelector("#preset-list");
  consoleLogEl = document.querySelector("#console-log");

  // Registers DOM bindings
  const registers = ["eax", "ebx", "ecx", "edx", "cs", "ds", "ss", "eip", "esp", "eflags"];
  registers.forEach((reg) => {
    registerEls[reg] = document.querySelector(`#reg-${reg}`);
  });

  // Attach Listeners
  pingBtnEl?.addEventListener("click", pingBackend);
  initBtnEl?.addEventListener("click", initializeProject);
  listProfilesBtnEl?.addEventListener("click", fetchProfiles);
  stepBtnEl?.addEventListener("click", simulateRegisterChange);
  
  clearLogBtnEl?.addEventListener("click", () => {
    if (consoleLogEl) consoleLogEl.innerHTML = "";
  });

  // Do initial ping check
  setTimeout(pingBackend, 500);
});
