// Package crash contains failures (owner review, Phase 7.5). A panic in a worker goroutine
// would end the whole app, losing unsaved work and leaving no trace; Go runs f with a recover
// that writes the panic and its stack to a log file and reports it, and the app goes on.
// Logs live in PyxForge's own folder (%APPDATA%\PyxForge\logs on Windows,
// $XDG_CONFIG_HOME/pyxforge/logs elsewhere); the newest keep files are kept.
package crash

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/obstinix/PyxForge/internal/buildinfo"
)

// keep is how many crash logs are kept.
const keep = 20

// Dir is where crash logs go; tests set it.
var Dir = defaultDir()

func defaultDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "PyxForge", "logs")
	}
	name := "pyxforge"
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		name = "PyxForge"
	}
	return filepath.Join(base, name, "logs")
}

// Report is told of each contained panic, with what was running, the panic's text and the
// log's path ("" when it could not be written). The shell sets it to show a toast; it is
// called on the goroutine that panicked.
var Report func(what, message, log string)

var mu sync.Mutex

// Write records a panic and its stack in a new log file and returns its path.
func Write(what string, recovered any, stack []byte) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(Dir, 0o755); err != nil {
		return "", err
	}
	now := time.Now()
	name := fmt.Sprintf("crash-%s-%s.log", now.Format("20060102-150405.000"), slug(what))
	var b strings.Builder
	fmt.Fprintf(&b, "PyxForge %s crashed in %s\n", buildinfo.Version, what)
	fmt.Fprintf(&b, "Time: %s\nSystem: %s/%s, %s\n\n", now.Format(time.RFC3339), runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Fprintf(&b, "panic: %v\n\n%s", recovered, stack)
	p := filepath.Join(Dir, name)
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	prune()
	return p, nil
}

// prune removes all but the newest keep logs.
func prune() {
	logs, _ := filepath.Glob(filepath.Join(Dir, "crash-*.log"))
	if len(logs) <= keep {
		return
	}
	sort.Strings(logs) // the names start with the time
	for _, p := range logs[:len(logs)-keep] {
		os.Remove(p)
	}
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
		if b.Len() >= 32 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

// Recover, deferred, contains a panic in the calling goroutine: it is logged and reported, and
// the goroutine ends normally.
func Recover(what string) {
	if r := recover(); r != nil {
		contain(what, r)
	}
}

// Handle logs and reports a panic already recovered, for a goroutine that has its own clean-up
// to do after one.
func Handle(what string, r any) { contain(what, r) }

func contain(what string, r any) {
	stack := debug.Stack()
	p, err := Write(what, r, stack)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyxforge: %s crashed: %v (the log could not be written: %v)\n%s", what, r, err, stack)
	}
	if Report != nil {
		Report(what, fmt.Sprint(r), p)
	}
}

// Go runs f on a new goroutine, contained.
func Go(what string, f func()) {
	go func() {
		defer Recover(what)
		f()
	}()
}
