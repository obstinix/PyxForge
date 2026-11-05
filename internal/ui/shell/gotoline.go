package shell

import (
	"errors"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// parseLineCol reads "12" or "12:5" (line, column from 1).
func parseLineCol(s string) (line, col int, err error) {
	l, c, hasCol := strings.Cut(strings.TrimSpace(s), ":")
	line, err = strconv.Atoi(strings.TrimSpace(l))
	if err != nil || line < 1 {
		return 0, 0, errors.New("a line number, or line:column, such as 120 or 120:8")
	}
	col = 1
	if hasCol {
		if col, err = strconv.Atoi(strings.TrimSpace(c)); err != nil || col < 1 {
			return 0, 0, errors.New("a column is a number from 1")
		}
	}
	return line, col, nil
}

// goToLine asks for a line (and column) and moves the editor's cursor there.
func (s *Shell) goToLine() {
	if s.activeBuffer() == nil {
		s.Notify(notifications.Info, "No file to go to a line in", "Open a file in the editor first.")
		return
	}
	entry := s.newDialogEntry()
	entry.SetPlaceHolder("line or line:column")
	entry.Validator = func(v string) error { _, _, err := parseLineCol(v); return err }
	d := escForm("Go to Line", "Go", "Cancel", []*widget.FormItem{widget.NewFormItem("Line", entry)}, func(ok bool) {
		if ok {
			s.goToLineIn(entry.Text)
		}
	}, s.win)
	d.Resize(fyne.NewSize(360, d.MinSize().Height))
	s.submitForm(d, entry)
	s.win.Canvas().Focus(entry)
	s.lastDialog = d
}

// goToLineIn moves the cursor to "line" or "line:column", kept within the file, and centres
// the line; the editor takes keyboard focus.
func (s *Shell) goToLineIn(text string) {
	line, col, err := parseLineCol(text)
	if err != nil || s.activeBuffer() == nil {
		return
	}
	sess := s.ed.sess
	s.ed.do(func() {
		_ = sess.ExecLua(`local line, col = ...
line = math.min(line, vim.api.nvim_buf_line_count(0))
local text = vim.api.nvim_buf_get_lines(0, line - 1, line, false)[1] or ""
local byte = vim.fn.byteidx(text, col - 1)
if byte < 0 then byte = #text end
vim.api.nvim_win_set_cursor(0, { line, byte })
vim.cmd("normal! zz")`, nil, line, col)
	})
	s.FocusEditor()
}
