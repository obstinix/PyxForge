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
