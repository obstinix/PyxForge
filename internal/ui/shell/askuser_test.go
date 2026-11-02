package shell

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/askpass"
)

// TestAskUserDialogs: a password is typed into a hidden field and answered; a new server's
// trust is a choice made by hand; Esc and Cancel answer nothing (#67).
func TestAskUserDialogs(t *testing.T) {
	a := test.NewTempApp(t)
	q := make(queue, 64)
	s := NewWithOptions(a, t.TempDir(), Options{Dispatch: q.post})
	type reply struct {
		text string
		ok   bool
	}
	ask := func(prompt string) chan reply {
		ch := make(chan reply, 1)
		s.lastDialog = nil
		go func() { text, ok := s.askUser(askpass.Classify(prompt)); ch <- reply{text, ok} }()
		q.pumpUntil(t, "the question", func() bool { return s.lastDialog != nil })
		return ch
	}
	answered := func(ch chan reply) reply {
		t.Helper()
		var r reply
		q.pumpUntil(t, "the answer", func() bool {
			select {
			case r = <-ch:
				return true
			default:
				return false
			}
		})
		return r
	}

	ch := ask("Enter passphrase for key '/home/u/.ssh/id_ed25519': ")
	e, ok := s.win.Canvas().Focused().(*dialogEntry)
	if !ok || !e.Password {
		t.Fatalf("the passphrase field is not focused and hidden: %T", s.win.Canvas().Focused())
	}
	test.Type(e, "correct horse")
	tapInDialog(t, s, "OK")
	if r := answered(ch); r != (reply{"correct horse", true}) {
		t.Errorf("passphrase answer %+v", r)
	}
	if logged(s, "correct horse") {
		t.Error("the passphrase reached the Log")
	}

	ch = ask("Password for 'https://u@example.invalid': ")
	test.Type(s.win.Canvas().Focused().(*dialogEntry), "typed then cancelled")
	s.modalKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if r := answered(ch); r != (reply{"", false}) {
		t.Errorf("Esc answered %+v", r)
	}

	ch = ask("The authenticity of host 'git.example.invalid' can't be established.\nED25519 key fingerprint is SHA256:abc.\nAre you sure you want to continue connecting (yes/no/[fingerprint])? ")
	if findButton(s.win.Canvas().Overlays().Top(), "Trust and Connect") == nil {
		t.Fatal("no Trust and Connect")
	}
	s.modalKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // Enter does not trust a server
	select {
	case r := <-ch:
		t.Fatalf("Enter answered the trust question: %+v", r)
	default:
	}
	tapInDialog(t, s, "Trust and Connect")
	if r := answered(ch); r != (reply{"yes", true}) {
		t.Errorf("trust answer %+v", r)
	}
}

// TestFetchAsksForCredentials: a fetch from a server that wants a password makes Git ask
// through the helper, which opens the question in this window; cancelling it ends the fetch
// as not signed in (#67).
func TestFetchAsksForCredentials(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	var mu sync.Mutex
	asked := 0
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked++
		mu.Unlock()
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	gitRun(t, root, "remote", "add", "origin", "http://"+ln.Addr().String()+"/repo.git")
	gitRun(t, root, "config", "credential.helper", "")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	a := test.NewTempApp(t)
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post, Askpass: exe})
	g := s.gitp
	g.refresh()
	q.pumpUntil(t, "the repository", func() bool { return g.repo != nil && !g.running })
	s.lastDialog = nil
	g.fetch()
	q.pumpUntil(t, "Git's question in the window", func() bool {
		return s.lastDialog != nil && findButton(s.win.Canvas().Overlays().Top(), "OK") != nil
	})
	if f, ok := s.win.Canvas().Focused().(*dialogEntry); !ok || f.Password {
		t.Errorf("the username question: focused %T", s.win.Canvas().Focused())
	}
	tapInDialog(t, s, "Cancel")
	q.pumpUntil(t, "the fetch ending", func() bool { return g.netBusy == "" && strings.Contains(strings.Join(s.logLines, "\n"), "Git:") })
	mu.Lock()
	defer mu.Unlock()
	if asked == 0 {
		t.Error("the server was never asked")
	}
}
