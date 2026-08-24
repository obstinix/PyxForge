#![allow(dead_code)]

use std::path::{Path, PathBuf};
use chrono::Local;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ActiveScreen {
    Workspace,
    DebugInspect,
    BuildDiagnostics,
    QemuControl,
    HexExplorer,
    NewProject,
    ThemeGallery,
}

#[derive(Debug, Clone)]
pub struct FileItem {
    pub name: String,
    pub path: PathBuf,
    pub is_dir: bool,
    pub is_binary: bool,
    pub children: Vec<FileItem>,
    pub is_expanded: bool,
}

#[derive(Debug, Clone)]
pub struct OpenFile {
    pub name: String,
    pub path: PathBuf,
    pub content: String,
    pub is_dirty: bool,
    pub language: String,
}

#[derive(Debug, Clone)]
pub struct LogEntry {
    pub timestamp: String,
    pub category: String,
    pub message: String,
    pub is_error: bool,
}

#[derive(Debug, Clone)]
pub struct RegisterValue {
    pub name: String,
    pub value: u32,
    pub previous_value: u32,
    pub changed: bool,
}

#[derive(Debug, Clone)]
pub struct StackSlot {
    pub offset: String,
    pub value: u32,
    pub symbol: Option<String>,
}

#[derive(Debug, Clone)]
pub struct DisasmLine {
    pub address: u32,
    pub hex_bytes: String,
    pub instruction: String,
    pub is_current_ip: bool,
    pub has_breakpoint: bool,
}

#[derive(Debug, Clone)]
pub struct SnapshotItem {
    pub tag: String,
    pub vm_clock: String,
    pub date: String,
    pub size: String,
}

#[derive(Debug, Clone)]
pub struct DiagnosticItem {
    pub severity: String, // "ERROR", "WARNING", "INFO"
    pub file: String,
    pub line: u32,
    pub column: u32,
    pub message: String,
    pub fix_suggestion: Option<String>,
}

#[derive(Debug, Clone)]
pub struct BuildProfileItem {
    pub name: String,
    pub target: String,
    pub tool: String,
    pub description: String,
}

pub struct AppState {
    pub active_screen: ActiveScreen,
    pub workspace_root: PathBuf,
    pub file_tree: Vec<FileItem>,
    pub open_files: Vec<OpenFile>,
    pub active_file_idx: Option<usize>,
    
    // Terminal & Logs
    pub logs: Vec<LogEntry>,
    pub log_filter: String,
    pub terminal_input: String,

    // QEMU & Debugger
    pub qemu_running: bool,
    pub qemu_pid: Option<u32>,
    pub gdb_port: u16,
    pub qemu_architecture: String,
    pub qemu_memory: String,
    
    // CPU Registers & Stack & Disassembly
    pub registers: Vec<RegisterValue>,
    pub stack_slots: Vec<StackSlot>,
    pub disassembly: Vec<DisasmLine>,
    pub current_eip: u32,
    
    // Memory Inspector & Hex Explorer
    pub memory_address_input: String,
    pub memory_dump_lines: Vec<String>,
    pub hex_dump_lines: Vec<String>,
    pub hex_file_path: String,
    pub is_boot_signature_valid: bool,

    // QEMU Snapshots & HMP
    pub snapshot_tag_input: String,
    pub snapshots: Vec<SnapshotItem>,
    pub hmp_input: String,
    pub hmp_output: String,

    // Build Profiles & Diagnostics
    pub profiles: Vec<BuildProfileItem>,
    pub selected_profile: String,
    pub diagnostics: Vec<DiagnosticItem>,
    pub last_build_status: Option<bool>,

    // New Project Scaffolding
    pub scaffold_project_name: String,
    pub scaffold_template: String,
    pub scaffold_output_dir: PathBuf,
    pub scaffold_status_msg: Option<String>,

    // Status bar notification
    pub status_message: String,
}

impl Default for AppState {
    fn default() -> Self {
        let current_dir = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
        let mut state = Self {
            active_screen: ActiveScreen::Workspace,
            workspace_root: current_dir.clone(),
            file_tree: Vec::new(),
            open_files: Vec::new(),
            active_file_idx: None,
            logs: Vec::new(),
            log_filter: "ALL".to_string(),
            terminal_input: String::new(),
            qemu_running: false,
            qemu_pid: None,
            gdb_port: 1234,
            qemu_architecture: "i386 (x86 Real Mode)".to_string(),
            qemu_memory: "128M".to_string(),
            registers: Vec::new(),
            stack_slots: Vec::new(),
            disassembly: Vec::new(),
            current_eip: 0x7C00,
            memory_address_input: "0x7C00".to_string(),
            memory_dump_lines: Vec::new(),
            hex_dump_lines: Vec::new(),
            hex_file_path: "boot.bin".to_string(),
            is_boot_signature_valid: true,
            snapshot_tag_input: "boot_init".to_string(),
            snapshots: Vec::new(),
            hmp_input: "info registers".to_string(),
            hmp_output: "Ready for QEMU HMP Monitor commands.".to_string(),
            profiles: Vec::new(),
            selected_profile: "x86_realmode".to_string(),
            diagnostics: Vec::new(),
            last_build_status: None,
            scaffold_project_name: "my_os_bootloader".to_string(),
            scaffold_template: "x86-realmode".to_string(),
            scaffold_output_dir: current_dir.clone(),
            scaffold_status_msg: None,
            status_message: "PyxForge Native Desktop Ready.".to_string(),
        };

        state.init_mock_data();
        state.scan_workspace();
        state
    }
}

impl AppState {
    pub fn add_log(&mut self, category: &str, message: &str, is_error: bool) {
        let now = Local::now().format("%H:%M:%S").to_string();
        self.logs.push(LogEntry {
            timestamp: now,
            category: category.to_string(),
            message: message.to_string(),
            is_error,
        });
    }

    pub fn init_mock_data(&mut self) {
        // Init CPU Registers
        self.registers = vec![
            RegisterValue { name: "EAX".to_string(), value: 0x0000_0000, previous_value: 0x0000_0000, changed: false },
            RegisterValue { name: "EBX".to_string(), value: 0x0000_7C00, previous_value: 0x0000_0000, changed: true },
            RegisterValue { name: "ECX".to_string(), value: 0x0000_0020, previous_value: 0x0000_0020, changed: false },
            RegisterValue { name: "EDX".to_string(), value: 0x0000_0080, previous_value: 0x0000_0080, changed: false },
            RegisterValue { name: "ESI".to_string(), value: 0x0000_7C24, previous_value: 0x0000_7C00, changed: true },
            RegisterValue { name: "EDI".to_string(), value: 0x0000_B800, previous_value: 0x0000_0000, changed: true },
            RegisterValue { name: "ESP".to_string(), value: 0x0000_7C00, previous_value: 0x0000_7C00, changed: false },
            RegisterValue { name: "EBP".to_string(), value: 0x0000_0000, previous_value: 0x0000_0000, changed: false },
            RegisterValue { name: "EIP".to_string(), value: 0x0000_7C05, previous_value: 0x0000_7C00, changed: true },
            RegisterValue { name: "EFLAGS".to_string(), value: 0x0000_0202, previous_value: 0x0000_0202, changed: false },
            RegisterValue { name: "CR0".to_string(), value: 0x0000_0010, previous_value: 0x0000_0010, changed: false },
            RegisterValue { name: "CR3".to_string(), value: 0x0000_0000, previous_value: 0x0000_0000, changed: false },
        ];

        // Init Stack slots
        self.stack_slots = vec![
            StackSlot { offset: "ESP+0x00".to_string(), value: 0x0000_AA55, symbol: Some("BootSig".to_string()) },
            StackSlot { offset: "ESP+0x04".to_string(), value: 0x0000_7C00, symbol: Some("Stage1_Entry".to_string()) },
            StackSlot { offset: "ESP+0x08".to_string(), value: 0x0000_0000, symbol: None },
            StackSlot { offset: "ESP+0x0C".to_string(), value: 0x0000_0080, symbol: Some("DriveNumber".to_string()) },
            StackSlot { offset: "ESP+0x10".to_string(), value: 0x0000_9000, symbol: Some("GDT_Base".to_string()) },
        ];

        // Init Disassembly instructions
        self.disassembly = vec![
            DisasmLine { address: 0x7C00, hex_bytes: "FA".to_string(), instruction: "cli".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C01, hex_bytes: "31 C0".to_string(), instruction: "xor ax, ax".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C03, hex_bytes: "8E D8".to_string(), instruction: "mov ds, ax".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C05, hex_bytes: "8E C0".to_string(), instruction: "mov es, ax".to_string(), is_current_ip: true, has_breakpoint: true },
            DisasmLine { address: 0x7C07, hex_bytes: "8E D0".to_string(), instruction: "mov ss, ax".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C09, hex_bytes: "BC 00 7C".to_string(), instruction: "mov sp, 0x7c00".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C0C, hex_bytes: "FB".to_string(), instruction: "sti".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C0D, hex_bytes: "E8 0A 00".to_string(), instruction: "call print_welcome_msg".to_string(), is_current_ip: false, has_breakpoint: false },
            DisasmLine { address: 0x7C10, hex_bytes: "EB FE".to_string(), instruction: "jmp $".to_string(), is_current_ip: false, has_breakpoint: false },
        ];

        // Init Snapshots
        self.snapshots = vec![
            SnapshotItem { tag: "boot_init".to_string(), vm_clock: "00:00:00.124".to_string(), date: "2026-08-24 14:10".to_string(), size: "128 KB".to_string() },
            SnapshotItem { tag: "pre_protected_mode".to_string(), vm_clock: "00:00:02.481".to_string(), date: "2026-08-24 14:15".to_string(), size: "512 KB".to_string() },
        ];

        // Init Build Profiles
        self.profiles = vec![
            BuildProfileItem {
                name: "x86_realmode".to_string(),
                target: "i8086-unknown-none".to_string(),
                tool: "nasm".to_string(),
                description: "16-bit Master Boot Record (MBR) bootloader binary".to_string(),
            },
            BuildProfileItem {
                name: "x86_protected".to_string(),
                target: "i386-unknown-elf".to_string(),
                tool: "gcc/ld".to_string(),
                description: "32-bit Protected Mode Kernel with GDT and IDT setup".to_string(),
            },
            BuildProfileItem {
                name: "x86_64_longmode".to_string(),
                target: "x86_64-unknown-none".to_string(),
                tool: "rustc/cargo".to_string(),
                description: "64-bit Freestanding Rust Microkernel".to_string(),
            },
            BuildProfileItem {
                name: "arm_cortex_m4".to_string(),
                target: "thumbv7em-none-eabihf".to_string(),
                tool: "arm-none-eabi-gcc".to_string(),
                description: "ARM Bare-metal firmware for QEMU lm3s6965evb board".to_string(),
            },
        ];

        // Init Diagnostics
        self.diagnostics = vec![
            DiagnosticItem {
                severity: "WARNING".to_string(),
                file: "boot.asm".to_string(),
                line: 42,
                column: 8,
                message: "A20 line fast gate enabled without keyboard controller verification".to_string(),
                fix_suggestion: Some("Add A20 test loop to check 0x000000 vs 0x100000 wraparound".to_string()),
            },
            DiagnosticItem {
                severity: "INFO".to_string(),
                file: "pyxforge.toml".to_string(),
                line: 12,
                column: 1,
                message: "QMP socket enabled on 127.0.0.1:4444".to_string(),
                fix_suggestion: None,
            },
        ];

        // Init default open file
        let sample_asm = r#"; ==============================================================================
; PyxForge Bare-Metal Bootloader (x86 16-bit Real Mode)
; Target: IBM PC Compatible (BIOS 0x7C00)
; ==============================================================================

[BITS 16]
[ORG 0x7C00]

start:
    cli                     ; Disable maskable hardware interrupts
    xor ax, ax              ; Zero out AX register
    mov ds, ax              ; Data Segment = 0x0000
    mov es, ax              ; Extra Segment = 0x0000
    mov ss, ax              ; Stack Segment = 0x0000
    mov sp, 0x7C00          ; Stack Pointer grows down from 0x7C00
    sti                     ; Re-enable interrupts

    ; Print Blueprint Welcome Message to BIOS Teletype
    mov si, msg_welcome
    call print_string

    ; Check Boot Sector Signature
    mov ax, [0x7DFE]
    cmp ax, 0xAA55
    jne boot_error

    jmp $                   ; Halt / infinite loop

print_string:
    lodsb
    or al, al
    jz .done
    mov ah, 0x0E            ; BIOS Teletype function
    int 0x10
    jmp print_string
.done:
    ret

boot_error:
    mov si, msg_error
    call print_string
    hlt

msg_welcome: db ">> PyxForge OS Engine v0.1.0 Loaded <<", 0x0D, 0x0A, 0
msg_error:   db "Error: Invalid Boot Sector Signature!", 0x0D, 0x0A, 0

; Boot Sector Padding & Signature
times 510 - ($ - $$) db 0
dw 0xAA55                   ; MBR Magic Boot Signature
"#;

        self.open_files.push(OpenFile {
            name: "boot.asm".to_string(),
            path: PathBuf::from("boot.asm"),
            content: sample_asm.to_string(),
            is_dirty: false,
            language: "assembly".to_string(),
        });
        self.active_file_idx = Some(0);

        self.add_log("System", "PyxForge Native Blueprint Core initialized.", false);
        self.add_log("Workspace", "Loaded project config: pyxforge.toml", false);
    }

    pub fn scan_workspace(&mut self) {
        if let Ok(entries) = std::fs::read_dir(&self.workspace_root) {
            let mut items = Vec::new();
            for entry in entries.flatten() {
                let path = entry.path();
                let name = path.file_name().unwrap_or_default().to_string_lossy().into_owned();
                
                // Skip hidden folders like .git, node_modules, target
                if name.starts_with('.') || name == "target" || name == "node_modules" {
                    continue;
                }

                let is_dir = path.is_dir();
                let is_binary = path.extension().map_or(false, |ext| ext == "bin" || ext == "img" || ext == "o");
                
                items.push(FileItem {
                    name,
                    path,
                    is_dir,
                    is_binary,
                    children: Vec::new(),
                    is_expanded: false,
                });
            }
            items.sort_by(|a, b| b.is_dir.cmp(&a.is_dir).then(a.name.cmp(&b.name)));
            self.file_tree = items;
        }
    }

    pub fn open_file(&mut self, path: &Path) {
        // If already open, switch to it
        if let Some(idx) = self.open_files.iter().position(|f| f.path == path) {
            self.active_file_idx = Some(idx);
            return;
        }

        // Read file from disk
        if let Ok(content) = std::fs::read_to_string(path) {
            let name = path.file_name().unwrap_or_default().to_string_lossy().into_owned();
            let ext = path.extension().unwrap_or_default().to_string_lossy().to_string();
            let language = match ext.as_str() {
                "asm" | "s" | "S" => "assembly".to_string(),
                "rs" => "rust".to_string(),
                "c" | "h" => "c".to_string(),
                "toml" => "toml".to_string(),
                _ => "text".to_string(),
            };

            self.open_files.push(OpenFile {
                name,
                path: path.to_path_buf(),
                content,
                is_dirty: false,
                language,
            });
            self.active_file_idx = Some(self.open_files.len() - 1);
            self.add_log("Editor", &format!("Opened {}", path.display()), false);
        }
    }

    pub fn save_active_file(&mut self) {
        if let Some(idx) = self.active_file_idx {
            if let Some(file) = self.open_files.get_mut(idx) {
                if file.is_dirty {
                    if let Ok(_) = std::fs::write(&file.path, &file.content) {
                        file.is_dirty = false;
                        self.status_message = format!("Saved {}", file.name);
                    }
                }
            }
        }
    }

    pub fn step_debugger(&mut self) {
        self.current_eip += 2;
        for reg in &mut self.registers {
            if reg.name == "EIP" {
                reg.previous_value = reg.value;
                reg.value = self.current_eip;
                reg.changed = true;
            } else if reg.name == "EAX" {
                reg.previous_value = reg.value;
                reg.value = (reg.value + 1) & 0xFFFF;
                reg.changed = true;
            } else {
                reg.changed = false;
            }
        }
        self.add_log("GDB", &format!("Single step executed. EIP -> 0x{:04X}", self.current_eip), false);
    }
}
