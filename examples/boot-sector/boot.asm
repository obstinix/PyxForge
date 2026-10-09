; boot.asm: a BIOS boot sector. The BIOS loads these 512 bytes at 0x7c00 and jumps here in
; 16-bit real mode. It prints a line through the BIOS (the screen) and COM1 (QEMU's
; -serial stdio), then halts. Build: pyxforge build. Break at 0x7c00 to step through it.
bits 16
org 0x7c00

start:
    cli
    xor ax, ax              ; flat segments: data and stack in the first 64 KiB
    mov ds, ax
    mov es, ax
    mov ss, ax
    mov sp, 0x7c00          ; the stack grows down from just below this code
    sti

    mov si, message
.next:
    lodsb                   ; al = [ds:si], si += 1
    test al, al
    jz halt
    mov ah, 0x0e            ; BIOS teletype output
    int 0x10
    call serial_put
    jmp .next

; serial_put writes al to COM1 once its transmit buffer is empty.
serial_put:
    push ax
    mov dx, 0x3fd           ; line status register
.wait:
    in al, dx
    test al, 0x20           ; transmitter holding register empty?
    jz .wait
    pop ax
    mov dx, 0x3f8           ; data register
    out dx, al
    ret

halt:
    cli
    hlt
    jmp halt

message: db "PyxForge boot sector OK", 13, 10, 0

    times 510 - ($ - $$) db 0
    dw 0xaa55               ; boot signature: the BIOS boots only sectors that end with it
