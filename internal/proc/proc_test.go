package proc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary plays roles chosen by PYX_PROC: "parent" contains its children, starts a
// "child" and prints its process ID, and with PYX_PID_FILE also starts a "detached" child,
// which writes its process ID there; "child" and "detached" sleep.
func TestMain(m *testing.M) {
	switch os.Getenv("PYX_PROC") {
	case "child":
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case "detached":
		_ = os.WriteFile(os.Getenv("PYX_PID_FILE"), []byte(strconv.Itoa(os.Getpid())), 0o644)
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case "parent":
		if err := ContainChildren(); err != nil {
			fmt.Println("error", err)
			os.Exit(1)
		}
		c := exec.Command(os.Args[0])
		c.Env = append(os.Environ(), "PYX_PROC=child")
		Bind(c)
		if err := c.Start(); err != nil {
			fmt.Println("error", err)
			os.Exit(1)
		}
		if os.Getenv("PYX_PID_FILE") != "" {
			os.Setenv("PYX_PROC", "detached")
			if err := StartDetached(os.Args[0]); err != nil {
				fmt.Println("error", err)
				os.Exit(1)
			}
		}
		fmt.Println("child", c.Process.Pid)
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestChildrenDieWithAKilledParent kills the parent the hard way (TerminateProcess, SIGKILL),
// so none of its own cleanup runs, and expects its child to end too.
func TestChildrenDieWithAKilledParent(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("no kernel support for tying children to their parent on " + runtime.GOOS)
	}
	pidFile := filepath.Join(t.TempDir(), "detached.pid")
	parent := exec.Command(os.Args[0])
	parent.Env = append(os.Environ(), "PYX_PROC=parent", "PYX_PID_FILE="+pidFile)
	out, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "child ") {
		_ = parent.Process.Kill()
		t.Fatalf("parent said %q, %v", line, err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "child ")))
	if !Alive(pid) {
		t.Fatalf("child %d is not running", pid)
	}
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	for deadline := time.Now().Add(10 * time.Second); Alive(pid); {
		if time.Now().After(deadline) {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
			t.Fatalf("child %d outlived its killed parent", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// A process started with StartDetached (another PyxForge window) is not taken along.
	var detached int
	for deadline := time.Now().Add(10 * time.Second); detached == 0 && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		data, _ := os.ReadFile(pidFile)
		detached, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	if detached == 0 {
		t.Fatal("the detached child did not start")
	}
	if !Alive(detached) {
		t.Error("the detached child ended with its parent")
	}
	if p, err := os.FindProcess(detached); err == nil {
		_ = p.Kill()
	}
}

func TestAlive(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Error("this process is not alive")
	}
	c := exec.Command(os.Args[0])
	c.Env = append(os.Environ(), "PYX_PROC=none")
	c.Args = append(c.Args, "-test.run=^$")
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	if Alive(c.Process.Pid) {
		t.Error("an exited process is alive")
	}
}
