import * as fs from 'fs';
import * as path from 'path';

export interface Preset {
	name: string;
	description: string;
	tomlTemplate: (projectName: string) => string;
}

const registry: Preset[] = [];

/**
 * Registers a new build profile configuration preset into the global extension registry.
 *
 * @param p The preset object containing the name, description, and template generator.
 */
export function registerPreset(p: Preset) {
	registry.push(p);
}

/**
 * Retrieves the complete array of currently registered build profile presets.
 */
export function getPresets(): Preset[] {
	return registry;
}

// Register all presets at module load
registerPreset({
	name: 'Bootloader',
	description: 'Assemble BIOS bootloader using nasm (Real Mode i8086 target)',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the BIOS bootloader"
source_dir = "."
output_dir = "build"
args = ["-f", "bin", "boot.asm", "-o", "build/boot.bin"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
boot_image = "build/boot.bin"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i8086"
`
});

registerPreset({
	name: 'Kernel Debug',
	description: 'Compile freestanding C kernel with debug symbols and no optimization',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.kernel_debug]
tool = "gcc"
description = "Compile the kernel with debug symbols"
source_dir = "."
output_dir = "build"
args = ["-g", "-O0", "-ffreestanding", "-c", "kernel.c", "-o", "build/kernel.o"]
env = { DEBUG = "1" }

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
kernel = "build/kernel.bin"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i386:x86-64"
`
});

registerPreset({
	name: 'Kernel Release',
	description: 'Compile freestanding C kernel with optimizations and debug symbols removed',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.kernel_release]
tool = "gcc"
description = "Compile the kernel with optimizations"
source_dir = "."
output_dir = "build"
args = ["-O2", "-ffreestanding", "-c", "kernel.c", "-o", "build/kernel.o"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
kernel = "build/kernel.bin"

[qemu.debug]
enabled = false
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i386:x86-64"
`
});

registerPreset({
	name: 'Rust Application',
	description: 'Build native bare-metal Rust kernel/app using Cargo',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.rust_app]
tool = "cargo"
description = "Build the Rust application"
source_dir = "."
output_dir = "target"
args = ["build"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
kernel = "target/debug/${projectName}"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i386:x86-64"
`
});

registerPreset({
	name: 'C Application',
	description: 'Compile standard host or embedded C application using gcc',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.c_app]
tool = "gcc"
description = "Compile C application"
source_dir = "."
output_dir = "build"
args = ["-g", "main.c", "-o", "build/app"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
kernel = "build/app"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "auto"
`
});

registerPreset({
	name: 'C++ Application',
	description: 'Compile standard host or embedded C++ application using g++',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.cpp_app]
tool = "g++"
description = "Compile C++ application"
source_dir = "."
output_dir = "build"
args = ["-g", "main.cpp", "-o", "build/app"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
kernel = "build/app"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "auto"
`
});

registerPreset({
	name: 'Embedded',
	description: 'Compile and emulate for ARM Cortex-M4 (QEMU lm3s6965evb target)',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.embedded]
tool = "arm-none-eabi-gcc"
description = "Compile for ARM Cortex-M4"
source_dir = "."
output_dir = "build"
args = ["-g", "-O1", "-mcpu=cortex-m4", "-mthumb", "main.c", "-o", "build/embedded.elf"]

[qemu]
executable = "qemu-system-arm"
machine = "lm3s6965evb"
memory = "64M"
kernel = "build/embedded.elf"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb-multiarch"
architecture = "arm"
`
});

registerPreset({
	name: 'Bare Metal',
	description: 'Assemble bare-metal stage binary using nasm (x86 Protected Mode)',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.bare_metal]
tool = "nasm"
description = "Assemble bare-metal kernel stage"
source_dir = "."
output_dir = "build"
args = ["-f", "bin", "kernel.asm", "-o", "build/kernel.bin"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
boot_image = "build/kernel.bin"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i386"
`
});

registerPreset({
	name: 'Custom',
	description: 'Skeleton Makefile/Build configurations customizable by the user',
	tomlTemplate: (projectName) => `[project]
name = "${projectName}"

[profiles.custom]
tool = "make"
description = "Custom build configuration"
source_dir = "."
output_dir = "build"
args = ["all"]
env = {}

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
architecture = "auto"
`
});

/**
 * Extracts the project name from the TOML string, or returns a fallback if not found.
 */
export function extractProjectName(tomlContent: string, fallback: string): string {
	const lines = tomlContent.split(/\r?\n/);
	let inProjectSection = false;

	for (const line of lines) {
		const trimmed = line.trim();
		if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
			const section = trimmed.substring(1, trimmed.length - 1).trim().toLowerCase();
			inProjectSection = (section === 'project');
			continue;
		}

		if (inProjectSection) {
			const match = trimmed.match(/^name\s*=\s*"(.*?)"/);
			if (match) {
				return match[1];
			}
		}
	}

	const secondaryMatch = tomlContent.match(/name\s*=\s*"(.*?)"/);
	if (secondaryMatch) {
		return secondaryMatch[1];
	}

	return fallback;
}
