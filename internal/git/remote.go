package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// Remotes and the network (PXF-DEF-030). Fetch, pull and push run git like the rest of the
// package, with the same rule (decision D14): what the repository's own configuration names
// as a program to run is refused, here the programs Git runs to reach a remote. Nothing ever
// waits for a typed answer: Git does not prompt for a user name or password (a credential
// helper, such as Git Credential Manager, may show its own window), and ssh runs in batch mode
// unless the user chose an ssh command. A failure says what kind it is (RemoteError).

// NetworkTimeout is how long a fetch, pull or push may take when the caller sets no deadline.
const NetworkTimeout = 5 * time.Minute

// networkProgramKeys are the settings naming a program Git runs to reach a remote, or letting a
// remote's URL run one (the ext:: transport, which Git allows only when told to).
const networkProgramKeys = `^(core\.sshcommand|core\.askpass|core\.gitproxy|credential\.(.+\.)?helper|remote\..+\.(uploadpack|receivepack)|protocol\.(ext\.)?allow)$`

// Remote is a repository this one exchanges commits with.
type Remote struct {
	Name string
	URL  string // where fetches come from
}

// Remotes lists the remotes, in the order Git keeps them.
func (r *Repo) Remotes(ctx context.Context) ([]Remote, error) {
	out, err := r.run(ctx, nil, "remote", "-v")
	if err != nil {
		return nil, err
	}
	var list []Remote
	for _, line := range strings.Split(out, "\n") {
		name, rest, ok := strings.Cut(strings.TrimRight(line, "\r"), "\t")
		if url, fetch := strings.CutSuffix(rest, " (fetch)"); ok && fetch {
			list = append(list, Remote{Name: name, URL: url})
		}
	}
	return list, nil
}

// AddRemote adds a remote by name and URL. The name may not start with "-".
func (r *Repo) AddRemote(ctx context.Context, name, url string) error {
	name, url = strings.TrimSpace(name), strings.TrimSpace(url)
	switch {
	case name == "" || url == "":
		return errors.New("a remote needs a name and a URL")
	case strings.HasPrefix(name, "-"):
		return errors.New("a remote's name cannot start with -")
	}
	_, err := r.run(ctx, nil, "remote", "add", "--", name, url)
	return err
}

// Init makes dir a new, empty repository (git init) and opens it. The user's own Git
// configuration decides the first branch's name and the template.
func Init(ctx context.Context, dir string) (*Repo, error) {
	r := &Repo{Root: dir}
	if _, err := r.exec(ctx, nil, nil, "init", "--quiet"); err != nil {
		return nil, r.explain([]string{"init"}, err)
	}
	return Open(ctx, dir)
}

// Fetch downloads what the current branch's remote (or origin) has that this repository does
// not. Branches and the working tree do not change; Status then shows how far behind they are.
func (r *Repo) Fetch(ctx context.Context) error {
	remotes, err := r.Remotes(ctx)
	if err != nil {
		return classify(ctx, "fetch", err)
	}
	if len(remotes) == 0 { // git fetch says nothing and succeeds
		return &RemoteError{Op: "fetch", Problem: ProblemNoRemote, err: errors.New("git fetch: this repository has no remote")}
	}
	_, err = r.network(ctx, "fetch", false, "fetch", "--no-recurse-submodules")
	return err
}

// Pull moves the current branch up to its upstream when that only moves it forward: git pull
// --ff-only, whatever pull.rebase says. When both sides have new commits it fails with
// ProblemDiverged, and how to combine them, merge or rebase, is left to the user.
func (r *Repo) Pull(ctx context.Context) error {
	_, err := r.network(ctx, "pull", true, "pull", "--ff-only", "--no-recurse-submodules")
	return err
}

// Push sends the current branch to its upstream. With remote set, it publishes the branch on
// that remote under its own name and makes that its upstream. It never forces: a remote with
// commits the branch lacks rejects it (ProblemRejected).
func (r *Repo) Push(ctx context.Context, remote string) error {
	args := []string{"push", "--recurse-submodules=no"}
	if remote != "" {
		if strings.HasPrefix(remote, "-") {
			return errors.New("a remote's name cannot start with -")
		}
		args = append(args, "--set-upstream", remote, "HEAD")
	}
	_, err := r.network(ctx, "push", false, args...)
	return err
}

// network runs a git command that talks to a remote; worktree marks one that also updates the
// working tree (pull), which is refused like any other that converts files (safety).
func (r *Repo) network(ctx context.Context, op string, worktree bool, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, NetworkTimeout)
		defer cancel()
	}
	progs, err := r.projectPrograms(ctx, networkProgramKeys)
	if err != nil {
		return "", classify(ctx, op, r.explain(args, err))
	}
	for _, p := range progs {
		if p.Value != "" { // an empty value clears a list (credential.helper) and runs nothing
			return "", p
		}
	}
	opts, err := r.safety(ctx, worktree)
	if err != nil {
		return "", classify(ctx, op, r.explain(args, err))
	}
	out, err := r.exec(ctx, nil, append(opts, r.batchSSH(ctx)...), args...)
	return out, classify(ctx, op, r.explain(args, err))
}

// batchSSH makes ssh fail at once where it would ask: for a key's passphrase without an agent,
// or to trust a server it has not seen. Its question would go to a console nobody sees. An ssh
// command the user chose (core.sshCommand, GIT_SSH_COMMAND, GIT_SSH) is left as it is.
func (r *Repo) batchSSH(ctx context.Context) []string {
	if os.Getenv("GIT_SSH_COMMAND") != "" || os.Getenv("GIT_SSH") != "" {
		return nil
	}
	if out, err := r.exec(ctx, nil, nil, "config", "--get", "core.sshCommand"); err == nil && strings.TrimSpace(out) != "" {
		return nil
	}
	return []string{"-c", "core.sshCommand=ssh -o BatchMode=yes"}
}

// Problem is the kind of a RemoteError.
type Problem int

const (
	ProblemUnknown      Problem = iota
	ProblemCancelled            // stopped before it finished
	ProblemTimedOut             // took longer than its deadline
	ProblemAuth                 // the remote refused the credentials, or none could be asked for
	ProblemHostKey              // ssh does not trust the server's key (yet)
	ProblemNetwork              // the server could not be reached
	ProblemNotFound             // no repository at the URL, or none this account may see
	ProblemNoRemote             // the repository has no remote
	ProblemNoUpstream           // the branch tracks no remote branch
	ProblemRejected             // the remote has commits the branch does not: pull first
	ProblemDiverged             // both sides have new commits: a pull cannot fast-forward
	ProblemLocalChanges         // the pull would overwrite uncommitted changes
)

// RemoteError is a fetch, pull or push that failed. Error is git's own message.
type RemoteError struct {
	Op      string // "fetch", "pull" or "push"
	Problem Problem
	err     error
}

func (e *RemoteError) Error() string { return e.err.Error() }
func (e *RemoteError) Unwrap() error { return e.err }

// classify wraps a network command's failure in a RemoteError. A refusal of the repository's
// own programs (ProjectProgramError) is no remote problem and stays as it is.
func classify(ctx context.Context, op string, err error) error {
	if err == nil {
		return nil
	}
	if pe := (*ProjectProgramError)(nil); errors.As(err, &pe) {
		return err
	}
	p := problemOf(err.Error())
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		p = ProblemCancelled
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		p = ProblemTimedOut
	}
	return &RemoteError{Op: op, Problem: p, err: err}
}

// problemOf reads the kind of failure from what git printed (LC_ALL=C: in English).
func problemOf(msg string) Problem {
	m := strings.ToLower(msg)
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(m, p) {
				return true
			}
		}
		return false
	}
	switch {
	case has("host key verification failed", "remote host identification has changed", "no matching host key"):
		return ProblemHostKey
	case has("authentication failed", "permission denied", "could not read username", "could not read password",
		"terminal prompts disabled", "invalid username or password", "returned error: 401", "returned error: 403", "access denied"):
		return ProblemAuth
	case has("repository not found", "does not appear to be a git repository", "returned error: 404") ||
		has("fatal: repository '") && has("' not found"):
		return ProblemNotFound
	case has("could not resolve host", "could not resolve hostname", "failed to connect", "connection refused",
		"connection timed out", "connection reset", "network is unreachable", "no route to host",
		"could not connect to server", "operation timed out", "ssl certificate problem", "unable to access"):
		return ProblemNetwork
	case has("no configured push destination", "no remote repository specified", "no such remote"):
		return ProblemNoRemote
	case has("has no upstream branch", "no tracking information"):
		return ProblemNoUpstream
	case has("[rejected]", "non-fast-forward", "updates were rejected", "fetch first"):
		return ProblemRejected
	case has("not possible to fast-forward", "diverging branches"):
		return ProblemDiverged
	case has("would be overwritten by merge", "commit your changes or stash them", "untracked working tree files would be overwritten"):
		return ProblemLocalChanges
	}
	return ProblemUnknown
}
