package neovim

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/nvim"
)

// TestSetupInstallsPinnedPlugins runs Setup offline against a local Git repository standing in
// for a plugin's upstream: it clones at the pinned commit, leaves an up-to-date plugin alone,
// moves a plugin at another commit back to the pin, and refuses a commit that does not exist.
func TestSetupInstallsPinnedPlugins(t *testing.T) {
	for _, tool := range []string{"git", "nvim"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com",
			"-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	upstream := t.TempDir()
	git(upstream, "init", "--quiet")
	_ = os.MkdirAll(filepath.Join(upstream, "lua"), 0o755)
	_ = os.WriteFile(filepath.Join(upstream, "lua", "plugin.lua"), []byte("return 1\n"), 0o644)
	git(upstream, "add", ".")
	git(upstream, "commit", "--quiet", "-m", "one")
	first := git(upstream, "rev-parse", "HEAD")
	_ = os.WriteFile(filepath.Join(upstream, "lua", "plugin.lua"), []byte("return 2\n"), 0o644)
	git(upstream, "commit", "--quiet", "-am", "two")
	second := git(upstream, "rev-parse", "HEAD")

	xdg := t.TempDir()
	var env []string
	for _, d := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
		env = append(env, "XDG_"+d+"_HOME="+filepath.Join(xdg, strings.ToLower(d)))
	}
	init, err := nvim.Install(filepath.Join(xdg, "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	opts := func(rev string) SetupOptions {
		return SetupOptions{Config: init, Env: env, Out: &bytes.Buffer{},
			Lock: nvim.Lockfile{Plugins: []nvim.Plugin{{Name: "demo", URL: upstream, Rev: rev}}}}
	}
	ctx := context.Background()

	if err := Setup(ctx, opts(first)); err != nil {
		t.Fatal(err)
	}
	var plugin string
	_ = filepath.WalkDir(xdg, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == "demo" && strings.Contains(p, filepath.Join("pack", "pyxforge", "opt")) {
			plugin = p
		}
		return nil
	})
	if plugin == "" {
		t.Fatal("plugin not installed under PyxForge's pack folder")
	}
	if got := git(plugin, "rev-parse", "HEAD"); got != first {
		t.Errorf("installed %s, pinned %s", got, first)
	}

	o := opts(first)
	if err := Setup(ctx, o); err != nil || !strings.Contains(o.Out.(*bytes.Buffer).String(), "up to date") {
		t.Errorf("second run: %v %q", err, o.Out)
	}
	if err := Setup(ctx, opts(second)); err != nil || git(plugin, "rev-parse", "HEAD") != second {
		t.Errorf("moving to a new pin: %v", err)
	}
	if err := Setup(ctx, opts(strings.Repeat("0", 40))); err == nil {
		t.Error("a commit that does not exist was accepted")
	}
}
