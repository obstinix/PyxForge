package shell

import (
	"strings"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// outputView is a scrolling list of output lines from a process (a build, QEMU's serial port,
// GDB). Lines may be added from any goroutine; they reach the list in batches on the UI thread
// and the list follows the end. Lines starting with "==> " are headings.
type outputView struct {
	s     *Shell
	max   int
	lines []string
	list  *logList
	frame *kit.FocusFrame

	mu      sync.Mutex
	pending []string
	queued  atomic.Bool
}

func newOutputView(s *Shell, maxLines int) *outputView {
	o := &outputView{s: s, max: maxLines}
	o.list = &logList{onFocus: func(on bool) {
		o.frame.SetFocused(on)
		if on {
			s.activate(regionPanel)
		}
	}}
	o.list.Length = func() int { return len(o.lines) }
	o.list.CreateItem = func() fyne.CanvasObject {
		t := kit.NewText("", kit.Mono, kit.Secondary)
		t.TextSize = theme.TextCaption + 1
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space2), t)
	}
	o.list.UpdateItem = func(i widget.ListItemID, c fyne.CanvasObject) {
		t := c.(*fyne.Container).Objects[0].(*kit.Text)
		line := o.lines[i]
		t.Role = kit.Secondary
		if strings.HasPrefix(line, "==> ") {
			t.Role = kit.Primary
		}
		t.SetText(line)
	}
	o.list.ExtendBaseWidget(o.list)
	o.list.HideSeparators = true // console output reads as one text, not as rows
	o.frame = kit.NewFocusFrame(o.list)
	return o
}

// Widget is the view to place in a panel.
func (o *outputView) Widget() fyne.CanvasObject { return o.frame }

// Add queues a line from any goroutine.
func (o *outputView) Add(line string) {
	o.mu.Lock()
	o.pending = append(o.pending, line)
	o.mu.Unlock()
	if o.queued.Swap(true) {
		return
	}
	o.s.dispatch(o.Flush)
}

// Flush moves queued lines into the list, on the UI thread.
func (o *outputView) Flush() {
	o.queued.Store(false)
	o.mu.Lock()
	more := o.pending
	o.pending = nil
	o.mu.Unlock()
	if len(more) == 0 {
		return
	}
	o.lines = append(o.lines, more...)
	if over := len(o.lines) - o.max; over > 0 {
		o.lines = append(o.lines[:0], o.lines[over:]...)
	}
	o.list.Refresh()
	o.list.ScrollToBottom()
}

// AddNow appends lines on the UI thread, after anything queued.
func (o *outputView) AddNow(lines ...string) {
	o.Flush()
	o.lines = append(o.lines, lines...)
	o.list.Refresh()
	o.list.ScrollToBottom()
}

// Clear empties the view.
func (o *outputView) Clear() {
	o.mu.Lock()
	o.pending = nil
	o.mu.Unlock()
	o.lines = o.lines[:0]
	o.list.Refresh()
}

// Lines returns the lines shown, for tests.
func (o *outputView) Lines() []string { return o.lines }
