package askpass

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// TestMain makes the test binary an askpass helper when Git starts it as one, as PyxForge's
// executable is.
func TestMain(m *testing.M) {
	if Requested() && len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-test.") {
		os.Exit(Client(os.Args[1:], os.Stdout))
	}
	os.Exit(m.Run())
}

// TestGitAsksThroughPyxForge: with the server's environment, real Git asks for a username and
// password through the helper, and gets the answers the user gave; a cancelled question gives
// Git nothing; a helper without the token is not answered.
func TestGitAsksThroughPyxForge(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var asked []Question
	cancel := false
	s, err := Start(func(q Question) (string, bool) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, q)
		if cancel {
			return "", false
		}
		if q.Secret {
			return "s3cret pass", true
		}
		return "obstinix", true
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fill := func(env []string) (string, error) {
		cmd := exec.Command("git", "-c", "credential.helper=", "credential", "fill")
		cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull), env...)
		cmd.Stdin = strings.NewReader("protocol=https\nhost=git.example.invalid\n\n")
		out, err := cmd.Output()
		return string(out), err
	}
	out, err := fill(s.Env(exe))
	if err != nil {
		t.Fatalf("git credential fill: %v\n%s", err, out)
	}
	if !strings.Contains(out, "username=obstinix\n") || !strings.Contains(out, "password=s3cret pass\n") {
		t.Errorf("git got:\n%s", out)
	}
	if len(asked) != 2 || asked[0].Secret || !asked[1].Secret || !strings.Contains(asked[0].Text, "Username") {
		t.Errorf("questions: %+v", asked)
	}

	cancel = true
	if out, err := fill(s.Env(exe)); err == nil || strings.Contains(out, "password=") {
		t.Errorf("a cancelled question still gave git %q (%v)", out, err)
	}

	// A helper with the wrong token: the server does not ask the user.
	mu.Lock()
	n := len(asked)
	mu.Unlock()
	env := s.Env(exe)
	env[0] = strings.Fields(env[0])[0] + " not-the-token"
	_, _ = fill(env)
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != n {
		t.Errorf("a question with the wrong token reached the user: %+v", asked[n:])
	}
}

// TestClassify: ssh's and git's prompts.
func TestClassify(t *testing.T) {
	for prompt, want := range map[string]Question{
		"Enter passphrase for key '/home/u/.ssh/id_ed25519': ":                                 {Secret: true},
		"Password for 'https://u@github.com': ":                                                {Secret: true},
		"Username for 'https://github.com': ":                                                  {},
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ":                {YesNo: true},
		"The authenticity of host 'h' can't be established.\nAre you sure (yes/no)? something": {YesNo: true},
	} {
		got := Classify(prompt)
		if got.Secret != want.Secret || got.YesNo != want.YesNo || got.Text != prompt {
			t.Errorf("Classify(%q) = %+v", prompt, got)
		}
	}
}
