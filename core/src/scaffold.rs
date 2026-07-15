use std::fs;
use std::path::Path;

pub fn generate_scaffold(project_name: &str, project_root: &Path) -> Result<(), String> {
    if !project_root.exists() {
        return Err(format!(
            "Project root directory '{}' does not exist.",
            project_root.display()
        ));
    }

    // Define files and their contents
    let pyxforge_toml = format!(
        r#"[project]
name = "{}"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the bootloader (stage 1)"
args = ["-f", "bin", "boot.asm", "-o", "build/boot.bin"]

[profiles.kernel]
tool = "nasm"
description = "Assemble the stage 2 kernel"
args = ["-f", "bin", "kernel.asm", "-o", "build/kernel.bin"]
depends_on = ["bootloader"]

[profiles.kernel.gdb]
architecture = "i386"

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
boot_image = "build/os-image.bin"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i8086"
"#,
        project_name
    );

    let boot_asm = r#"; PyxForge boot.asm - 2-Stage BIOS Bootloader
org 0x7c00
bits 16

start:
    cli
    xor ax, ax
    mov ds, ax
    mov es, ax
    mov ss, ax
    mov sp, 0x7c00
    sti

    mov si, msg_loading
    call print_string

    ; Read 1 sector (the kernel) from hard drive (dl=0x80) to 0x1000:0000
    mov ax, 0x1000
    mov es, ax
    xor bx, bx

    mov ah, 0x02    ; BIOS read sectors
    mov al, 1       ; Read 1 sector
    mov ch, 0       ; Cylinder 0
    mov cl, 2       ; Sector 2
    mov dh, 0       ; Head 0
    int 0x13
    jc disk_error

    ; Jump to stage 2 kernel
    jmp 0x1000:0000

disk_error:
    mov si, msg_error
    call print_string
hang:
    hlt
    jmp hang

print_string:
    lodsb
    or al, al
    jz .done
    mov ah, 0x0e
    int 0x10
    jmp print_string
.done:
    ret

msg_loading db "Loading stage 2 kernel...", 0x0d, 0x0a, 0
msg_error db "Disk read error!", 0x0d, 0x0a, 0

times 510-($-$$) db 0
dw 0xaa55
"#;

    let kernel_asm = r#"; PyxForge kernel.asm - Stage 2 Kernel Entry Point
org 0x0000
bits 16

start:
    ; Set segment registers to match 0x1000
    mov ax, cs
    mov ds, ax
    mov es, ax

    mov si, msg_kernel
    call print_string

hang:
    hlt
    jmp hang

print_string:
    lodsb
    or al, al
    jz .done
    mov ah, 0x0e
    int 0x10
    jmp print_string
.done:
    ret

msg_kernel db "Hello from Stage 2 Kernel!", 0x0d, 0x0a, 0

times 512-($-$$) db 0   ; Pad to exactly 512 bytes (1 sector)
"#;

    let makefile = r#".PHONY: all clean run debug

all:
	@mkdir -p build
	nasm -f bin boot.asm -o build/boot.bin
	nasm -f bin kernel.asm -o build/kernel.bin
	cat build/boot.bin build/kernel.bin > build/os-image.bin

clean:
	rm -rf build

run: all
	qemu-system-x86_64 -drive format=raw,file=build/os-image.bin

debug: all
	qemu-system-x86_64 -drive format=raw,file=build/os-image.bin -s -S
"#;

    let tasks_json = r#"{
	"version": "2.0.0",
	"tasks": [
		{
			"type": "shell",
			"label": "PyxForge: Build Bootloader",
			"command": "${command:pyxforge.build}",
			"group": {
				"kind": "build",
				"isDefault": true
			},
			"problemMatcher": []
		}
	]
}
"#;

    let launch_json = r#"{
	"version": "0.2.0",
	"configurations": [
		{
			"type": "gdb",
			"request": "attach",
			"name": "PyxForge: GDB Debug",
			"executable": "",
			"remote": true,
			"target": ":1234",
			"cwd": "${workspaceFolder}",
			"gdbpath": "gdb",
			"autorun": [
				"set architecture i8086"
			],
			"valuesFormatting": "parseText"
		}
	]
}
"#;

    // Create directories
    let vscode_dir = project_root.join(".vscode");
    fs::create_dir_all(&vscode_dir).map_err(|e| {
        format!(
            "Failed to create .vscode directory at '{}': {}",
            vscode_dir.display(),
            e
        )
    })?;

    // Write files
    let write_file = |path: &Path, content: &str| -> Result<(), String> {
        fs::write(path, content)
            .map_err(|e| format!("Failed to write file '{}': {}", path.display(), e))
    };

    write_file(&project_root.join("pyxforge.toml"), &pyxforge_toml)?;
    write_file(&project_root.join("boot.asm"), boot_asm)?;
    write_file(&project_root.join("kernel.asm"), kernel_asm)?;
    write_file(&project_root.join("Makefile"), makefile)?;
    write_file(&vscode_dir.join("tasks.json"), tasks_json)?;
    write_file(&vscode_dir.join("launch.json"), launch_json)?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_scaffold() {
        let test_dir = Path::new("target/test-scaffold-dir");
        if test_dir.exists() {
            let _ = fs::remove_dir_all(test_dir);
        }
        fs::create_dir_all(test_dir).unwrap();

        let result = generate_scaffold("my-test-os", test_dir);
        assert!(result.is_ok());

        assert!(test_dir.join("pyxforge.toml").exists());
        assert!(test_dir.join("boot.asm").exists());
        assert!(test_dir.join("kernel.asm").exists());
        assert!(test_dir.join("Makefile").exists());
        assert!(test_dir.join(".vscode/tasks.json").exists());
        assert!(test_dir.join(".vscode/launch.json").exists());

        // Check if pyxforge.toml parses correctly
        let content = fs::read_to_string(test_dir.join("pyxforge.toml")).unwrap();
        assert!(content.contains("name = \"my-test-os\""));

        // Cleanup
        let _ = fs::remove_dir_all(test_dir);
    }
}
