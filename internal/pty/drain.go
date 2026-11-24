package pty

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// The end of a program's output: once the program has exited, its last output can still be on
// its way (ConPTY renders the console a frame at a time; a background job can hold a Unix pty
// open). The PTY waits until output has been quiet for quiet, at most drainLimit, then closes
// its side, so a reader sees all of it and then io.EOF.
const (
	quiet      = 60 * time.Millisecond
	drainLimit = 2 * time.Second
	// closeGrace is how long Close leaves a program to end once its terminal is gone before
	// it is killed.
	closeGrace = 2 * time.Second
)

// base is what both platforms share: the exit status and the drain after exit.
type base struct {
	exited   chan struct{}
	status   int
	lastRead atomic.Int64 // UnixNano of the last read that returned output
	once     sync.Once
	closing  atomic.Bool
}

func newBase() base { return base{exited: make(chan struct{})} }

// Done is closed when the program has exited.
func (b *base) Done() <-chan struct{} { return b.exited }

// Wait blocks until the program exits and returns its exit status: on Windows its exit code as
// a signed 32-bit number (0xC000013A is negative), on Unix its exit code or 128 plus the
// signal that ended it.
func (b *base) Wait() int {
	<-b.exited
	return b.status
}

// read wraps a platform read: a read after the PTY closed is io.EOF.
func (b *base) read(r func([]byte) (int, error), p []byte) (int, error) {
	n, err := r(p)
	if n > 0 {
		b.lastRead.Store(time.Now().UnixNano())
	}
	if err != nil {
		return n, io.EOF
	}
	return n, nil
}

// exit records the program's status and, once its output has gone quiet, closes the PTY's
// side (end) so readers see io.EOF.
func (b *base) exit(status int, end func()) {
	b.status = status
	close(b.exited)
	start := time.Now()
	for time.Since(start) < drainLimit {
		last := time.Unix(0, b.lastRead.Load())
		if time.Since(last) >= quiet && time.Since(start) >= quiet {
			break
		}
		time.Sleep(quiet / 4)
	}
	b.once.Do(end)
}
