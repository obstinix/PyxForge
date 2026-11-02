// Package askpass lets programs PyxForge starts ask the user a question through PyxForge's
// window (#67). Git and ssh ask for a password, a key's passphrase or whether to trust a new
// server through an "askpass" program named in GIT_ASKPASS and SSH_ASKPASS, which they run with
// the question as its argument and whose output is the answer. PyxForge's own executable is that
// program: started with PYXFORGE_ASKPASS set, it passes the question to the running PyxForge
// over a local connection that only the holder of a one-time token may use, and prints the
// answer. Answers are never logged or kept.
package askpass

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// EnvVar holds the address and token an askpass helper uses to reach PyxForge.
const EnvVar = "PYXFORGE_ASKPASS"

// Question is what a program asks.
type Question struct {
	Text   string // the prompt as the program wrote it
	Secret bool   // a password or passphrase: shown as dots, never echoed
	YesNo  bool   // a yes/no decision, such as trusting a server's host key
}

// Classify reads a prompt: ssh and git ask for passwords and passphrases, and ask whether to
// continue connecting to a host they have not seen.
func Classify(prompt string) Question {
	p := strings.ToLower(prompt)
	return Question{
		Text:   prompt,
		Secret: strings.Contains(p, "password") || strings.Contains(p, "passphrase") || strings.Contains(p, "pin for"),
		YesNo:  strings.Contains(p, "(yes/no"),
	}
}

type request struct {
	Token  string `json:"token"`
	Prompt string `json:"prompt"`
}

type response struct {
	OK     bool   `json:"ok"`
	Answer string `json:"answer"`
}

// Server answers askpass helpers for as long as it is open.
type Server struct {
	ln    net.Listener
	token string
	ask   func(Question) (answer string, ok bool)
}

// Start listens on the loopback interface; ask is called, from another goroutine, for each
// question, and returns the answer or false when the user cancelled.
func Start(ask func(Question) (string, bool)) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{ln: ln, token: hex.EncodeToString(b), ask: ask}
	go s.serve()
	return s, nil
}

func (s *Server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	var req request
	if err := json.NewDecoder(io.LimitReader(c, 64*1024)).Decode(&req); err != nil {
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Token), []byte(s.token)) != 1 {
		return // not a helper PyxForge started
	}
	_ = c.SetReadDeadline(time.Time{})
	answer, ok := s.ask(Classify(req.Prompt))
	_ = json.NewEncoder(c).Encode(response{OK: ok, Answer: answer})
}

// Env is what to add to the environment of a program that may ask: exe is the askpass helper
// (PyxForge's executable). SSH_ASKPASS_REQUIRE=force makes ssh use it although no display or
// terminal is there.
func (s *Server) Env(exe string) []string {
	return []string{
		EnvVar + "=" + s.ln.Addr().String() + " " + s.token,
		"GIT_ASKPASS=" + exe,
		"SSH_ASKPASS=" + exe,
		"SSH_ASKPASS_REQUIRE=force",
	}
}

// Close stops answering.
func (s *Server) Close() error { return s.ln.Close() }

// Requested reports whether this process was started as an askpass helper.
func Requested() bool { return os.Getenv(EnvVar) != "" }

// Client is the helper: it asks PyxForge the question in args and writes the answer to out. It
// returns the exit status: 0 with an answer, 1 when cancelled or PyxForge could not be reached.
func Client(args []string, out io.Writer) int {
	addr, token, ok := strings.Cut(os.Getenv(EnvVar), " ")
	if !ok {
		return 1
	}
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyxforge askpass:", err)
		return 1
	}
	defer c.Close()
	if err := json.NewEncoder(c).Encode(request{Token: token, Prompt: strings.Join(args, " ")}); err != nil {
		return 1
	}
	var resp response
	if err := json.NewDecoder(bufio.NewReader(c)).Decode(&resp); err != nil || !resp.OK {
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(os.Stderr, "pyxforge askpass:", err)
		}
		return 1
	}
	fmt.Fprintln(out, resp.Answer)
	return 0
}
