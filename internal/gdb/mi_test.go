package gdb

import "testing"

// TestParseRecordTruncated: a record cut off anywhere is an error, never a panic. The reader
// shares GDB's output with the debugged program's, so any line can arrive here.
func TestParseRecordTruncated(t *testing.T) {
	full := `^done,stack=[frame={level="0",addr="0x7c00",func="??"},frame={level="1"}],args=["a\"b",{x="1"}]`
	for i := range len(full) + 1 {
		_, _ = ParseRecord(full[:i])
	}
	for _, line := range []string{"*,=[", "^done,a=[", `^done,a=["x",`, "^done,a={", "^done,a={b=", `^done,a="x`, `~"\`, "=x,a=[[", "+y,a=[{"} {
		if _, err := ParseRecord(line); err == nil {
			t.Errorf("%q parsed without an error", line)
		}
	}
}

// FuzzParseRecord: no input makes the parser panic. go test -fuzz FuzzParseRecord ./internal/gdb
func FuzzParseRecord(f *testing.F) {
	for _, s := range []string{
		`^done,bkpt={number="1",type="breakpoint",addr="0x00007c00",file="boot.asm",line="7"}`,
		`*stopped,reason="breakpoint-hit",frame={addr="0x7c00",args=[]},thread-id="1"`,
		`=breakpoint-modified,bkpt={number="2",times="1"}`,
		`^done,register-values=[{number="0",value="0x0"},{number="1",value="0x1"}]`,
		`~"Reading symbols\n"`,
		`12^error,msg="No symbol table is loaded."`,
		"*,=[",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { _, _ = ParseRecord(s) })
}
