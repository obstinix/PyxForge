package shell

import (
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
)

// StallLimit is the longest the window may go without answering before the watchdog says so
// (owner review gate 7: no stall over 200 ms).
const StallLimit = 200 * time.Millisecond

// WatchStalls asks the UI thread to answer every 100 ms and logs each time it took longer than
// StallLimit, so a freeze leaves a trace in the Log tab (and report, when set, hears of it). It
// returns the longest wait so far, for measuring.
func (s *Shell) WatchStalls(report func(time.Duration)) func() time.Duration {
	var worst atomic.Int64
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			if s.closed.Load() {
				return
			}
			sent := time.Now()
			done := make(chan struct{})
			fyne.Do(func() { close(done) })
			<-done
			wait := time.Since(sent)
			if int64(wait) > worst.Load() {
				worst.Store(int64(wait))
			}
			if wait > StallLimit {
				s.dispatch(func() { s.logf("The window did not answer for %d ms", wait.Milliseconds()) })
				if report != nil {
					report(wait)
				}
			}
		}
	}()
	return func() time.Duration { return time.Duration(worst.Load()) }
}
