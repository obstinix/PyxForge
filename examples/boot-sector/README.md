# Boot sector example

The smallest program PyxForge builds, runs and debugs: 512 bytes that a PC BIOS loads at
`0x7c00` and runs in 16-bit real mode. It prints one line on the screen and on the serial
port, then halts.

Needs NASM and QEMU (`pyxforge doctor` checks both).

```sh
pyxforge build                  # assembles build/boot.bin and a listing, build/boot.lst
pyxforge run                    # builds, boots it in QEMU and prints the serial port
pyxforge inspect build/boot.bin --disasm   # signature, bytes used, hex, real-mode disassembly
```

`pyxforge run` prints `PyxForge boot sector OK`; stop it with Ctrl+C. In the desktop app, open this
folder and press Ctrl+Shift+B to build.

To debug, `pyxforge run --debug` starts QEMU paused with its GDB stub on port 1234 (from
`[qemu.debug]`). In another terminal:

```
gdb -ex "target remote localhost:1234" -ex "set architecture i8086"
(gdb) break *0x7c00
(gdb) continue
```

Connect first, then set the architecture: GDB takes QEMU's x86-64 register layout when it
connects and rejects it if i8086 was set before. GDB still disassembles with that layout, so use
`pyxforge inspect build/boot.bin --disasm` for a real-mode listing.
