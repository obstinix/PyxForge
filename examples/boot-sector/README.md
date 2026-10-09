# Boot sector example

The smallest program PyxForge builds, runs and debugs: 512 bytes that a PC BIOS loads at
`0x7c00` and runs in 16-bit real mode. It prints one line on the screen and on the serial
port, then halts.

Needs NASM and QEMU (`pyxforge doctor` checks both).

```sh
pyxforge build                  # assembles build/boot.bin and a listing, build/boot.lst
qemu-system-x86_64 -drive format=raw,file=build/boot.bin -serial stdio -display none
```

QEMU prints `PyxForge boot sector OK`; stop it with Ctrl+C. In the desktop app, open this folder
and press Ctrl+Shift+B to build.

To debug, start QEMU paused with its GDB stub (`-s -S`), then in GDB:

```
set architecture i8086
target remote localhost:1234
break *0x7c00
continue
```

`pyxforge.toml` already describes this setup (`[qemu]`, `[qemu.debug]`, `[gdb]`) for PyxForge's
run and debug commands, which arrive with the QEMU integration.
