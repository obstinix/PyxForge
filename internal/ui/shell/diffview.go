package shell

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/diff"
	"github.com/obstinix/PyxForge/internal/git"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// The Git tab's diff: a changed file beside the text it is compared with, in an editor tab.
// An unstaged change (an untracked file included) is the working tree against the index, so
// it shows what Stage would add; a change that is only staged is the index against HEAD, so it
// shows what Commit would record.

// diffSides are the two texts of a diff and what they are.
type diffSides struct {
	oldName, newName string
	old, new         string
}

// readDiffSides reads both sides of a changed file's diff.
func readDiffSides(ctx context.Context, repo *git.Repo, f git.File) (diffSides, error) {
	if f.Unstaged() {
		old, err := repo.Show(ctx, ":", f.Path)
		if err != nil {
			return diffSides{}, err
		}
		d := diffSides{oldName: "HEAD", newName: "Working Tree", old: old}
		if f.Staged() {
			d.oldName = "Staged"
		}
		if f.Untracked() {
			d.oldName = "Not tracked"
		}
		data, err := os.ReadFile(filepath.Join(repo.Root, filepath.FromSlash(f.Path)))
		switch {
		case os.IsNotExist(err):
			d.newName = "Deleted"
		case err != nil:
			return diffSides{}, err
		default:
			d.new = string(data)
		}
		return d, nil
	}
	old, err := repo.Show(ctx, "HEAD", f.Path)
	if err != nil {
		return diffSides{}, err
	}
	staged, err := repo.Show(ctx, ":", f.Path)
	if err != nil {
		return diffSides{}, err
	}
	return diffSides{oldName: "HEAD", newName: "Staged", old: old, new: staged}, nil
}

// diffView is one diff tab.
type diffView struct {
	s     *Shell
	repo  *git.Repo
	file  git.File
	item  *container.TabItem
	sides diffSides
	a, b  []string
	rows  []diff.Row
	hunks []int
	hunk  int // the change Previous and Next Change last went to, or -1

	list   *widget.List
	title  *widget.Label
	stats  *widget.Label
	prev   *widget.Button
	next   *widget.Button
	body   *fyne.Container
	notice *widget.Label // a binary file, or no change
}

// openDiff shows f's diff in its own editor tab, reusing the tab when it is already open.
func (g *gitPanel) openDiff(f git.File) {
	repo := g.repo
	if repo == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sides, err := readDiffSides(ctx, repo, f)
		g.s.dispatch(func() {
			if err != nil {
				g.s.Notify(notifications.Error, "Cannot show the diff", err.Error())
				return
			}
			g.s.showDiff(repo, f, sides)
		})
	}()
}

// showDiff puts a diff in its tab and selects it.
func (s *Shell) showDiff(repo *git.Repo, f git.File, sides diffSides) {
	key := filepath.Join(repo.Root, filepath.FromSlash(f.Path))
	v := s.diffs[key]
	if v == nil || !s.hasTab(v.item) {
		v = s.newDiffView(repo)
		if s.diffs == nil {
			s.diffs = map[string]*diffView{}
		}
		s.diffs[key] = v
		s.selectQuietly = true
		s.editors.Append(v.item)
		s.selectQuietly = false
		s.syncEditor()
	}
	v.repo, v.file = repo, f
	v.set(sides)
	s.selectTab(v.item, false)
}

func (s *Shell) hasTab(it *container.TabItem) bool {
	for _, t := range s.editors.Items {
		if t == it {
			return true
		}
	}
	return false
}

func (s *Shell) newDiffView(repo *git.Repo) *diffView {
	v := &diffView{s: s, repo: repo, hunk: -1}
	v.title = widget.NewLabel("")
	v.title.Truncation = fyne.TextTruncateEllipsis
	v.stats = widget.NewLabel("")
	v.stats.Importance = widget.LowImportance
	v.prev = widget.NewButton("Previous Change", func() { v.goTo(-1) })
	v.next = widget.NewButton("Next Change", func() { v.goTo(1) })
	refresh := widget.NewButton("Refresh", v.reload)
	open := widget.NewButton("Open File", v.openFile)
	for _, b := range []*widget.Button{v.prev, v.next, refresh, open} {
		b.Importance = widget.LowImportance
	}
	bar := container.NewBorder(nil, nil, nil, container.NewHBox(v.stats, v.prev, v.next, refresh, open), v.title)
	v.list = widget.NewList(func() int { return len(v.rows) },
		func() fyne.CanvasObject { return newDiffLine() },
		func(i widget.ListItemID, o fyne.CanvasObject) { v.fill(o.(*diffLine), i) })
	v.list.HideSeparators = true
	v.notice = widget.NewLabel("")
	v.notice.Alignment = fyne.TextAlignCenter
	v.body = container.NewStack(v.list, container.NewCenter(v.notice))
	content := container.NewBorder(container.NewVBox(bar, kit.NewRule(false)), nil, nil, nil, v.body)
	v.item = container.NewTabItem("", content)
	return v
}

// set shows new sides: the tab title, the counts and the rows.
func (v *diffView) set(sides diffSides) {
	v.sides = sides
	name := filepath.Base(filepath.FromSlash(v.file.Path))
	v.item.Text = name + " (diff)"
	v.title.SetText(fmt.Sprintf("%s · %s ↔ %s", v.file.Path, sides.oldName, sides.newName))
	v.hunk = -1
	v.a, v.b, v.rows, v.hunks = nil, nil, nil, nil
	switch {
	case strings.ContainsRune(sides.old, 0) || strings.ContainsRune(sides.new, 0):
		v.notice.SetText("A binary file: there is no line-by-line diff. Open File shows it.")
		v.stats.SetText("")
	default:
		v.a, v.b = diff.Lines(sides.old), diff.Lines(sides.new)
		edits := diff.Compute(v.a, v.b)
		v.rows = diff.SideBySide(edits)
		v.hunks = diff.Hunks(v.rows)
		del, ins := diff.Stats(edits)
		v.stats.SetText(fmt.Sprintf("−%d +%d", del, ins))
		v.notice.SetText("")
		if len(v.hunks) == 0 {
			v.notice.SetText("No line changed (only line endings or the final newline may differ).")
		}
	}
	v.notice.Hidden = v.notice.Text == ""
	v.list.Hidden = !v.notice.Hidden
	v.prev.Disable()
	v.next.Disable()
	if len(v.hunks) > 0 {
		v.prev.Enable()
		v.next.Enable()
	}
	v.list.Refresh()
	v.list.ScrollToTop()
	v.s.editors.Refresh()
	if len(v.hunks) > 0 {
		v.goTo(1) // start at the first change
	}
}

// goTo scrolls to the next (dir 1) or previous (dir -1) change, wrapping around.
func (v *diffView) goTo(dir int) {
	n := len(v.hunks)
	if n == 0 {
		return
	}
	switch {
	case v.hunk < 0 && dir > 0:
		v.hunk = 0
	case v.hunk < 0:
		v.hunk = n - 1
	default:
		v.hunk = (v.hunk + dir + n) % n
	}
	v.list.ScrollTo(max(v.hunks[v.hunk]-3, 0)) // a few lines of context above it
	v.stats.SetText(fmt.Sprintf("%s · change %d of %d", strings.SplitN(v.stats.Text, " · ", 2)[0], v.hunk+1, n))
}

// reload reads both sides again (after an edit, a stage or a commit).
func (v *diffView) reload() {
	repo, f := v.repo, v.file
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sides, err := readDiffSides(ctx, repo, f)
		v.s.dispatch(func() {
			if err != nil {
				v.s.Notify(notifications.Error, "Cannot show the diff", err.Error())
				return
			}
			v.set(sides)
		})
	}()
}

func (v *diffView) openFile() {
	p := filepath.Join(v.repo.Root, filepath.FromSlash(v.file.Path))
	if _, err := os.Stat(p); err != nil {
		v.s.Notify(notifications.Info, "Deleted file", v.file.Path+" is no longer on disk; Stage records the deletion.")
		return
	}
	v.s.OpenFile(p)
}

// fill shows row i in a line of the list.
func (v *diffView) fill(l *diffLine, i int) {
	if i >= len(v.rows) {
		return
	}
	r := v.rows[i]
	l.op = r.Op
	l.oldNum, l.oldText, l.newNum, l.newText = "", "", "", ""
	if r.Old >= 0 {
		l.oldNum, l.oldText = strconv.Itoa(r.Old+1), v.a[r.Old]
	}
	if r.New >= 0 {
		l.newNum, l.newText = strconv.Itoa(r.New+1), v.b[r.New]
	}
	l.hasOld, l.hasNew = r.Old >= 0, r.New >= 0
	l.Refresh()
}

// diffLine is one row of the diff: the old line on the left, the new one on the right, each
// with its line number. Text that does not fit its half is cut with an ellipsis; Open File
// shows the whole line.
type diffLine struct {
	widget.BaseWidget
	op                               diff.Op
	hasOld, hasNew                   bool
	oldNum, oldText, newNum, newText string
}

func newDiffLine() *diffLine {
	l := &diffLine{}
	l.ExtendBaseWidget(l)
	return l
}

func (l *diffLine) CreateRenderer() fyne.WidgetRenderer {
	r := &diffLineRenderer{l: l}
	for i := range 2 {
		r.bg[i] = canvas.NewRectangle(color.Transparent)
		r.num[i] = canvas.NewText("", color.Black)
		r.num[i].TextStyle.Monospace = true
		r.num[i].Alignment = fyne.TextAlignTrailing
		r.text[i] = canvas.NewText("", color.Black)
		r.text[i].TextStyle.Monospace = true
	}
	r.divider = canvas.NewRectangle(color.Transparent)
	return r
}

type diffLineRenderer struct {
	l       *diffLine
	bg      [2]*canvas.Rectangle
	num     [2]*canvas.Text
	text    [2]*canvas.Text
	divider *canvas.Rectangle
}

// diffNumChars is the width of the line-number column, in characters.
const diffNumChars = 5

func (r *diffLineRenderer) cell() fyne.Size {
	size := fyne.CurrentApp().Settings().Theme().Size(fynetheme.SizeNameText)
	return fyne.MeasureText("M", size, fyne.TextStyle{Monospace: true})
}

func (r *diffLineRenderer) Layout(s fyne.Size) {
	c := r.cell()
	pad := float32(theme.Space2)
	half := (s.Width - 1) / 2
	numW := c.Width * diffNumChars
	for i := range 2 {
		x := float32(i) * (half + 1)
		r.bg[i].Move(fyne.NewPos(x, 0))
		r.bg[i].Resize(fyne.NewSize(half, s.Height))
		r.num[i].Move(fyne.NewPos(x, (s.Height-c.Height)/2))
		r.num[i].Resize(fyne.NewSize(numW, c.Height))
		r.text[i].Move(fyne.NewPos(x+numW+pad, (s.Height-c.Height)/2))
		r.text[i].Resize(fyne.NewSize(max(half-numW-2*pad, 0), c.Height))
	}
	r.divider.Move(fyne.NewPos(half, 0))
	r.divider.Resize(fyne.NewSize(1, s.Height))
	r.fit(half - numW - 2*pad)
}

// fit cuts each side's text to the characters that fit in width, tabs shown as four spaces.
func (r *diffLineRenderer) fit(width float32) {
	c := r.cell()
	limit := int(width / c.Width)
	for i, s := range []string{r.l.oldText, r.l.newText} {
		s = strings.ReplaceAll(s, "\t", "    ")
		if rs := []rune(s); limit > 0 && len(rs) > limit {
			s = string(rs[:limit-1]) + "…"
		} else if limit <= 0 {
			s = ""
		}
		r.text[i].Text = s
	}
}

func (r *diffLineRenderer) MinSize() fyne.Size {
	c := r.cell()
	return fyne.NewSize(2*c.Width*(diffNumChars+8), c.Height+2)
}

func (r *diffLineRenderer) Refresh() {
	t := theme.Current()
	size := fyne.CurrentApp().Settings().Theme().Size(fynetheme.SizeNameText)
	removed, added := tint(t.Status.Error, 0.24), tint(t.Status.Success, 0.24)
	blank := tint(t.Border.Hairline, 0.35)
	l := r.l
	r.bg[0].FillColor, r.bg[1].FillColor = color.Transparent, color.Transparent
	if l.op != diff.Equal {
		r.bg[0].FillColor, r.bg[1].FillColor = removed, added
		if !l.hasOld {
			r.bg[0].FillColor = blank
		}
		if !l.hasNew {
			r.bg[1].FillColor = blank
		}
	}
	r.num[0].Text, r.num[1].Text = l.oldNum, l.newNum
	for i := range 2 {
		r.num[i].Color, r.num[i].TextSize = t.Text.Tertiary, size
		r.text[i].Color, r.text[i].TextSize = t.Text.Primary, size
		r.bg[i].Refresh()
	}
	r.divider.FillColor = t.Border.Hairline
	r.divider.Refresh()
	r.Layout(l.Size())
	for i := range 2 {
		r.num[i].Refresh()
		r.text[i].Refresh()
	}
}

// tint is c at the given opacity: a change's background, under text that keeps its contrast.
func tint(c color.NRGBA, alpha float64) color.NRGBA {
	c.A = uint8(float64(c.A) * alpha)
	return c
}

func (r *diffLineRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg[0], r.bg[1], r.divider, r.num[0], r.num[1], r.text[0], r.text[1]}
}

func (r *diffLineRenderer) Destroy() {}

// diffCurrent shows the diff of the file in the active editor tab (Git: Show Changes in
// Current File), read from a fresh status so it never acts on a stale list. It opens the
// workspace's repository itself when the Git tab has not read it yet.
func (g *gitPanel) diffCurrent() {
	repo, root, p := g.repo, g.s.root, g.s.activeFile()
	if p == "" {
		g.s.Notify(notifications.Info, "No file open", "Open a file in the editor first.")
		return
	}
	if repo == nil && root == "" {
		g.s.Notify(notifications.Info, "No folder open", "Show Changes works on a file in the open folder's Git repository.")
		return
	}
	trust := g.hooksTrust(root)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if repo == nil {
			if repo, err = git.Open(ctx, root); err == nil {
				repo.AllowHooksPath = trust
			}
		}
		var file *git.File
		var sides diffSides
		rel := ""
		if err == nil {
			r, rerr := filepath.Rel(repo.Root, p)
			if rerr != nil || strings.HasPrefix(r, "..") {
				err = fmt.Errorf("%s is outside the repository %s", p, repo.Root)
			}
			rel = filepath.ToSlash(r)
		}
		if err == nil {
			var st git.Status
			st, err = repo.Status(ctx)
			for i := range st.Files {
				if st.Files[i].Path == rel {
					file = &st.Files[i]
				}
			}
		}
		if err == nil && file != nil {
			sides, err = readDiffSides(ctx, repo, *file)
		}
		g.s.dispatch(func() {
			switch {
			case err != nil:
				g.s.Notify(notifications.Error, "Cannot show the changes", err.Error())
			case file == nil:
				g.s.Notify(notifications.Info, "No changes", rel+" is the same as its committed text.")
			default:
				g.s.showDiff(repo, *file, sides)
			}
		})
	}()
}
