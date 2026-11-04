package inspect

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Fuzz targets for the binary inspector, which reads whatever file the user opens (#43). The
// seeds run with the ordinary tests; `go test -fuzz` explores from them (the Fuzz workflow
// does so every night).

func FuzzReadELF(f *testing.F) {
	kernel, err := os.ReadFile(filepath.Join("testdata", "kernel.elf"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(kernel)
	f.Add(kernel[:64]) // the header alone
	f.Add(kernel[:len(kernel)/2])
	f.Add([]byte("\x7fELF\x01\x01\x01"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := ReadELF(bytes.NewReader(data))
		if err != nil {
			if e != nil {
				t.Fatal("ReadELF returned an error and a result")
			}
			return
		}
		for i := 1; i < len(e.Symbols); i++ {
			if e.Symbols[i].Addr < e.Symbols[i-1].Addr {
				t.Fatalf("symbols are not in address order: %#x after %#x", e.Symbols[i].Addr, e.Symbols[i-1].Addr)
			}
		}
		if e.Mode != 0 && e.Mode != 32 && e.Mode != 64 {
			t.Fatalf("mode %d", e.Mode)
		}
	})
}

func FuzzDisassemble(f *testing.F) {
	f.Add([]byte{0xfa, 0x31, 0xc0, 0x8e, 0xd8, 0xeb, 0xfe}, uint64(0x7c00), uint8(16), false)
	f.Add([]byte{0x55, 0x48, 0x89, 0xe5, 0xc3}, uint64(0x401000), uint8(64), true)
	f.Add([]byte{0x66, 0x67, 0xf0}, uint64(0xffffffffffffffff), uint8(32), false) // prefixes only, at the top of memory
	f.Add([]byte{0x0f}, uint64(0), uint8(32), true)
	f.Fuzz(func(t *testing.T, code []byte, addr uint64, mode uint8, intel bool) {
		m := []Mode{16, 32, 64}[int(mode)%3]
		ins := Disassemble(code, addr, m, intel)
		// The listing covers the code exactly, one instruction after another.
		pc, total := addr, 0
		for _, in := range ins {
			if in.Addr != pc {
				t.Fatalf("an instruction at %#x, want %#x", in.Addr, pc)
			}
			if len(in.Bytes) == 0 || in.Text == "" {
				t.Fatalf("an empty instruction at %#x: %+v", in.Addr, in)
			}
			if !bytes.Equal(in.Bytes, code[total:total+len(in.Bytes)]) {
				t.Fatalf("the bytes at %#x are not the code's", in.Addr)
			}
			pc += uint64(len(in.Bytes))
			total += len(in.Bytes)
			_ = in.String()
		}
		if total != len(code) {
			t.Fatalf("the listing covers %d of %d bytes", total, len(code))
		}
	})
}
