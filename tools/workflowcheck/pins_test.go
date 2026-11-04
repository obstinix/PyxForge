package workflowcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// usesLine matches a step's "uses:" value, with or without a leading "- ".
var usesLine = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*(\S+)(.*)$`)

// pinned is owner/repo[/path]@<40 hex digits>.
var pinned = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)

// versionComment is the "# v1.2.3" Dependabot keeps next to a pin.
var versionComment = regexp.MustCompile(`^\s*#\s*v\d+(\.\d+)*\S*\s*$`)

func workflowFiles(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "..", ".github", "workflows")
	var files []string
	for _, pattern := range []string{"*.yml", "*.yaml"} {
		m, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatalf("no workflows found in %s", dir)
	}
	return files
}

// TestActionsArePinnedByCommit checks that every action a workflow uses is pinned to a full
// commit SHA with its version in a comment (#32). A tag or a branch can be moved to other code
// by whoever controls the action's repository; a commit can't.
func TestActionsArePinnedByCommit(t *testing.T) {
	uses := 0
	for _, file := range workflowFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			m := usesLine.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			uses++
			ref := strings.Trim(m[1], `"'`)
			if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
				continue // an action in this repository, or an image pinned elsewhere
			}
			where := filepath.Base(file) + ":" + strconv.Itoa(i+1)
			if !pinned.MatchString(ref) {
				t.Errorf("%s: %q is not pinned to a commit SHA", where, ref)
				continue
			}
			if !versionComment.MatchString(m[2]) {
				t.Errorf("%s: %q has no version comment (such as # v5.1.0) for Dependabot", where, ref)
			}
		}
	}
	if uses == 0 {
		t.Fatal("found no uses: lines; the pattern no longer matches the workflows")
	}
}

func TestPinCheckRecognisesMutableRefs(t *testing.T) {
	for _, ref := range []string{"actions/checkout@v5", "dtolnay/rust-toolchain@stable", "a/b@1234567"} {
		if pinned.MatchString(ref) {
			t.Errorf("%q counted as pinned", ref)
		}
	}
	if !pinned.MatchString("actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09") {
		t.Error("a full commit SHA is not counted as pinned")
	}
	if !versionComment.MatchString(" # v4.6.2") || versionComment.MatchString(" # pinned") {
		t.Error("version comments are not recognised")
	}
}
