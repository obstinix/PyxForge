package git

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/obstinix/PyxForge/internal/askpass"
)

// Remote operations against real servers (#47, #67): Git and OpenSSH's client talk to them
// as to any host, over loopback. The SSH server is a small one built on golang.org/x/crypto/ssh
// that runs git-upload-pack and git-receive-pack; the HTTP one is git http-backend behind Basic
// authentication. The user's questions go through PyxForge's askpass server, with this test
// binary as Git's and ssh's askpass program (TestMain), as PyxForge's own executable is.

// sshServer serves the bare repositories in root over SSH to one authorized key.
type sshServer struct {
	addr    string
	hostKey ssh.PublicKey
}

func startSSHServer(t *testing.T, root string, authorized ssh.PublicKey) *sshServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorized.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("this key is not authorized")
		},
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() { ln.Close(); wg.Wait() })
	wg.Go(func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() { serveSSH(c, cfg, root) })
		}
	})
	return &sshServer{addr: ln.Addr().String(), hostKey: signer.PublicKey()}
}

func serveSSH(c net.Conn, cfg *ssh.ServerConfig, root string) {
	defer c.Close()
	conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return // a client that gave up, or a key that was refused
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			_ = nc.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		ch, chReqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go serveSession(ch, chReqs, root)
	}
}

// serveSession runs the one Git command a session asks for, git-upload-pack or
// git-receive-pack on a repository in root, and reports its exit status.
func serveSession(ch ssh.Channel, reqs <-chan *ssh.Request, root string) {
	defer ch.Close()
	var env []string
	for req := range reqs {
		switch req.Type {
		case "env": // Git sends GIT_PROTOCOL to choose protocol version 2
			var kv struct{ Name, Value string }
			if ssh.Unmarshal(req.Payload, &kv) == nil && kv.Name == "GIT_PROTOCOL" {
				env = append(env, kv.Name+"="+kv.Value)
			}
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
		case "exec":
			var payload struct{ Command string }
			service, repo, ok := "", "", ssh.Unmarshal(req.Payload, &payload) == nil
			if ok {
				service, repo, ok = gitService(payload.Command)
			}
			_ = req.Reply(ok, nil)
			if !ok {
				return
			}
			cmd := exec.Command("git", service, filepath.Join(root, repo))
			cmd.Env = append(os.Environ(), env...)
			cmd.Stdout, cmd.Stderr = ch, ch.Stderr()
			stdin, err := cmd.StdinPipe()
			status := uint32(1)
			if err == nil && cmd.Start() == nil {
				go func() { _, _ = io.Copy(stdin, ch); _ = stdin.Close() }()
				err = cmd.Wait()
				var exit *exec.ExitError
				switch {
				case err == nil:
					status = 0
				case errors.As(err, &exit):
					status = uint32(exit.ExitCode())
				}
			}
			_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
			return
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

// gitService reads "git-upload-pack '/name.git'": the git subcommand and the repository's
// name, which must be a plain name in the server's root.
func gitService(command string) (service, repo string, ok bool) {
	prog, arg, ok := strings.Cut(command, " ")
	if !ok || (prog != "git-upload-pack" && prog != "git-receive-pack") {
		return "", "", false
	}
	repo = path.Base(strings.Trim(arg, "'\""))
	if repo == "." || repo == "/" || repo == ".." || strings.ContainsAny(repo, `\:`) {
		return "", "", false
	}
	return strings.TrimPrefix(prog, "git-"), repo, true
}

// clientKey writes a new ed25519 key for ssh, protected by passphrase if it isn't empty.
func clientKey(t *testing.T, dir, name, passphrase string) (file string, pub ssh.PublicKey) {
	t.Helper()
	pubKey, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "pyxforge-test")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "pyxforge-test", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(dir, name)
	if err := os.WriteFile(file, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	pub, err = ssh.NewPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}
	return file, pub
}

// sshClients are the OpenSSH clients to test with: on Windows both Git for Windows' own and
// Windows' (which PyxForge's bundled MinGit, having none of its own, uses); elsewhere the one on
// PATH.
func sshClients(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("git", "--exec-path").Output(); err == nil {
			// <Git>\mingw64\libexec\git-core → <Git>\usr\bin\ssh.exe
			p := filepath.Join(filepath.FromSlash(strings.TrimSpace(string(out))), "..", "..", "..", "usr", "bin", "ssh.exe")
			if _, err := os.Stat(p); err == nil {
				found["Git for Windows"] = filepath.Clean(p)
			}
		}
		if p := filepath.Join(os.Getenv("SystemRoot"), "System32", "OpenSSH", "ssh.exe"); fileExists(p) {
			found["Windows"] = p
		}
	} else if p, err := exec.LookPath("ssh"); err == nil {
		found["OpenSSH"] = p
	}
	if len(found) == 0 {
		t.Skip("ssh is not installed")
	}
	return found
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// sshCommand is a GIT_SSH_COMMAND that uses only the test's key and known-hosts file, never the
// user's own configuration, keys or agent. Paths use forward slashes: Git runs the command with
// its POSIX shell.
func sshCommand(client, key, knownHosts string, extra ...string) string {
	q := func(s string) string { return "'" + filepath.ToSlash(s) + "'" }
	args := []string{q(client), "-F", "none",
		"-o", "UserKnownHostsFile=" + q(knownHosts), "-o", "GlobalKnownHostsFile=none",
		"-o", "IdentityFile=" + q(key), "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none",
		"-o", "PreferredAuthentications=publickey", "-o", "StrictHostKeyChecking=ask"}
	return strings.Join(append(args, extra...), " ")
}

// isolatedGit keeps the user's and the system's Git configuration (credential helpers, an ssh
// command, Git Credential Manager's window) out of a test.
func isolatedGit(t *testing.T) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

// asker answers the user's questions as a person at PyxForge's window would, and remembers
// them.
type asker struct {
	mu        sync.Mutex
	questions []askpass.Question
	answer    func(askpass.Question) (string, bool)
}

func (a *asker) ask(q askpass.Question) (string, bool) {
	a.mu.Lock()
	a.questions = append(a.questions, q)
	a.mu.Unlock()
	return a.answer(q)
}

func (a *asker) asked() []askpass.Question {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]askpass.Question(nil), a.questions...)
}

// askpassRepo is r with PyxForge's askpass server answering through a.
func askpassRepo(t *testing.T, r *Repo, a *asker) *Repo {
	t.Helper()
	srv, err := askpass.Start(a.ask)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return r.WithEnv(srv.Env(exe))
}

func bareRepo(t *testing.T, root, name string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "--quiet", "--bare", filepath.Join(root, name)).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", filepath.Join(root, name), "config", "http.receivepack", "true").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
}

// TestSSHRemote: over SSH, a new server's key fingerprint and a key's passphrase are asked
// through PyxForge (#67), and a push and a fetch then work; refusing the server, a changed host
// key and a key the server doesn't accept are told apart (#47).
func TestSSHRemote(t *testing.T) {
	isolatedGit(t)
	clients := sshClients(t)
	keys := t.TempDir()
	const passphrase = "correct horse"
	key, pub := clientKey(t, keys, "id_ed25519", passphrase)
	strangerKey, _ := clientKey(t, keys, "stranger", "")
	root := t.TempDir()
	srv := startSSHServer(t, root, pub)
	fingerprint := ssh.FingerprintSHA256(srv.hostKey)

	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			repo := strings.ReplaceAll(strings.ToLower(name), " ", "-") + ".git" // one per client
			bareRepo(t, root, repo)
			r := newRepo(t)
			write(t, r, "boot.asm", "org 0x7c00\n")
			commitAll(t, r, "First")
			if err := r.AddRemote(context.Background(), "origin", "ssh://"+srv.addr+"/"+repo); err != nil {
				t.Fatal(err)
			}
			knownHosts := filepath.Join(t.TempDir(), "known_hosts")
			if err := os.WriteFile(knownHosts, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			using := func(key string, extra ...string) {
				t.Setenv("GIT_SSH_COMMAND", sshCommand(client, key, knownHosts, extra...))
			}

			// Refusing a server not seen before: nothing is sent, and nothing is remembered.
			using(key)
			no := &asker{answer: func(q askpass.Question) (string, bool) { return "", false }}
			err := askpassRepo(t, r, no).Push(context.Background(), "origin")
			if problem(err) != ProblemHostKey {
				t.Errorf("refused server: %v (problem %d), want ProblemHostKey", err, problem(err))
			}
			if q := no.asked(); len(q) != 1 || !q[0].YesNo || !strings.Contains(q[0].Text, fingerprint) {
				t.Errorf("refused server: asked %+v, want one yes/no question naming %s", q, fingerprint)
			}

			// Trusting it, then the passphrase: the push and a fetch work.
			yes := &asker{answer: func(q askpass.Question) (string, bool) {
				switch {
				case q.YesNo:
					return "yes", true
				case q.Secret && strings.Contains(strings.ToLower(q.Text), "passphrase"):
					return passphrase, true
				}
				return "", false
			}}
			ar := askpassRepo(t, r, yes)
			if err := ar.Push(context.Background(), "origin"); err != nil {
				t.Fatalf("push over SSH: %v; asked %+v", err, yes.asked())
			}
			if err := ar.Fetch(context.Background()); err != nil {
				t.Fatalf("fetch over SSH: %v", err)
			}
			var trust, secret int
			for _, q := range yes.asked() {
				switch {
				case q.YesNo && strings.Contains(q.Text, fingerprint):
					trust++
				case q.Secret:
					secret++
				}
			}
			if trust != 1 || secret < 2 { // the server once; the passphrase for the push and the fetch
				t.Errorf("asked %+v: want the server's fingerprint once and the passphrase each time", yes.asked())
			}
			if data, _ := os.ReadFile(knownHosts); !bytes.Contains(data, []byte(srv.hostKey.Type())) {
				t.Errorf("the trusted server's key is not in known_hosts: %q", data)
			}

			// A wrong passphrase, and a key the server doesn't know: ProblemAuth.
			wrong := &asker{answer: func(q askpass.Question) (string, bool) { return "wrong", true }}
			using(key, "-o", "NumberOfPasswordPrompts=1")
			if err := askpassRepo(t, r, wrong).Fetch(context.Background()); problem(err) != ProblemAuth {
				t.Errorf("wrong passphrase: %v (problem %d), want ProblemAuth", err, problem(err))
			}
			using(strangerKey, "-o", "BatchMode=yes")
			if err := r.Fetch(context.Background()); problem(err) != ProblemAuth {
				t.Errorf("a key the server refuses: %v (problem %d), want ProblemAuth", err, problem(err))
			}

			// The server's key changed since it was trusted: ProblemHostKey, without asking.
			if err := os.WriteFile(knownHosts, []byte(knownhosts.Line([]string{knownhosts.Normalize(srv.addr)}, newPublicKey(t))+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			using(key, "-o", "BatchMode=yes")
			if err := r.Fetch(context.Background()); problem(err) != ProblemHostKey {
				t.Errorf("changed host key: %v (problem %d), want ProblemHostKey", err, problem(err))
			}

			// A server not seen before, where nobody can be asked: ProblemHostKey.
			if err := os.WriteFile(knownHosts, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := r.Fetch(context.Background()); problem(err) != ProblemHostKey {
				t.Errorf("unknown host in batch mode: %v (problem %d), want ProblemHostKey", err, problem(err))
			}
		})
	}
}

func newPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// TestHTTPRemote: against git http-backend behind Basic authentication, the user name and the
// password are asked through PyxForge and a push works; a wrong password is ProblemAuth (#47).
func TestHTTPRemote(t *testing.T) {
	isolatedGit(t)
	gitExe, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	bareRepo(t, root, "repo.git")
	const user, password = "pyx", "s3cret"
	backend := &cgi.Handler{Path: gitExe, Args: []string{"http-backend"}, InheritEnv: []string{"PATH"},
		Env: []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if u, p, ok := req.BasicAuth(); !ok || u != user || p != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="PyxForge test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, req)
	}))
	t.Cleanup(srv.Close)

	r := newRepo(t)
	write(t, r, "boot.asm", "org 0x7c00\n")
	commitAll(t, r, "First")
	if err := r.AddRemote(context.Background(), "origin", srv.URL+"/repo.git"); err != nil {
		t.Fatal(err)
	}
	answers := func(pw string) *asker {
		return &asker{answer: func(q askpass.Question) (string, bool) {
			if q.Secret {
				return pw, true
			}
			return user, true
		}}
	}
	bad := answers("wrong")
	if err := askpassRepo(t, r, bad).Push(context.Background(), "origin"); problem(err) != ProblemAuth {
		t.Errorf("wrong password: %v (problem %d), want ProblemAuth", err, problem(err))
	}
	good := answers(password)
	if err := askpassRepo(t, r, good).Push(context.Background(), "origin"); err != nil {
		t.Fatalf("push with the right password: %v", err)
	}
	q := good.asked()
	if len(q) != 2 || q[0].Secret || !strings.Contains(q[0].Text, "Username") || !q[1].Secret {
		t.Errorf("asked %+v, want the user name, then the password as a secret", q)
	}
	out, err := exec.Command("git", "-C", filepath.Join(root, "repo.git"), "log", "--oneline", "main").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "First") {
		t.Errorf("the server's repository: %v\n%s", err, out)
	}
}
