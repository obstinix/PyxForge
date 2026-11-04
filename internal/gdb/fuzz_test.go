package gdb

import (
	"strings"
	"testing"
)

// FuzzParseRecord feeds the GDB/MI parser arbitrary lines: GDB prints the program's and the
// target's output into the same stream, so any text can arrive (#43; #17 was a panic found
// this way).
func FuzzParseRecord(f *testing.F) {
	for _, s := range []string{
		`12^done,bkpt={number="1",type="breakpoint",addr="0x00007c00",thread-groups=["i1"]}`,
		`*stopped,reason="breakpoint-hit",disp="keep",bkptno="1",frame={addr="0x00007c00",func="??",args=[]},thread-id="1",stopped-threads="all"`,
		`^done,stack=[frame={level="0",addr="0x7c03",func="??"},frame={level="1",addr="0x0"}]`,
		`~"Remote debugging using localhost:1234\n"`,
		`&"warning: \"no symbol\"\n"`,
		`=thread-group-added,id="i1"`,
		`^error,msg="No symbol table is loaded.  Use the \"file\" command."`,
		`(gdb)`,
		`5`,
		`^done,a=[[],{}],b="\\\"",c={d=[e="1"]}`,
		`^done,bkpt={number="1",type="breakpoint",addr="0x00007c00",file="boot.asm",line="7"}`,
		`=breakpoint-modified,bkpt={number="2",times="1"}`,
		`^done,register-values=[{number="0",value="0x0"},{number="1",value="0x1"}]`,
		"*,=[",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		r, err := ParseRecord(line)
		if err != nil {
			return
		}
		if !strings.ContainsRune("^*+=~@&", rune(r.Kind)) {
			t.Fatalf("record kind %q from %q", r.Kind, line)
		}
		if r.Token < -1 {
			t.Fatalf("token %d from %q", r.Token, line)
		}
		checkValues(t, r.Results)
	})
}

// checkValues fails unless every value is a string, a tuple or a list of those.
func checkValues(t *testing.T, v any) {
	t.Helper()
	switch v := v.(type) {
	case nil, string:
	case map[string]any:
		for _, x := range v {
			checkValues(t, x)
		}
	case []any:
		for _, x := range v {
			checkValues(t, x)
		}
	default:
		t.Fatalf("a %T in a record", v)
	}
}
