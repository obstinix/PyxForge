; kernel.asm: the source of kernel.elf, a tiny 32-bit ELF for the inspect tests.
; Built on Linux: nasm -f elf32 kernel.asm -o kernel.o && ld -m elf_i386 -Ttext 0x100000 -e kmain -o kernel.elf kernel.o
bits 32
section .text
global kmain
kmain:
    mov esp, stack_top
    call clear
.hang:
    hlt
    jmp .hang

clear:
    mov edi, 0xb8000
    mov ecx, 80*25
    mov ax, 0x0720
    rep stosw
    ret

section .bss
    resb 4096
stack_top:
