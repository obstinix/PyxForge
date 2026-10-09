// Package command is PyxForge's command registry: every action the user can run from the
// command palette, a keybinding or a menu is registered here once, by ID.
package command

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Command is one runnable action.
type Command struct {
	ID       string
	Title    string
	Category string
	Keys     string // keybinding as shown to the user, e.g. "Ctrl+Shift+P"; empty if none
	Run      func()
}

// Label is the text the palette shows and matches against.
func (c Command) Label() string {
	if c.Category == "" {
		return c.Title
	}
	return c.Category + ": " + c.Title
}

// Registry holds commands in registration order.
type Registry struct {
	cmds []Command
	byID map[string]int
}

// Add registers a command. A duplicate or empty ID is a programming error and panics.
func (r *Registry) Add(c Command) {
	if c.ID == "" || c.Run == nil {
		panic(fmt.Sprintf("command: %q needs an ID and a Run func", c.Title))
	}
	if r.byID == nil {
		r.byID = map[string]int{}
	}
	if _, dup := r.byID[c.ID]; dup {
		panic("command: duplicate ID " + c.ID)
	}
	r.byID[c.ID] = len(r.cmds)
	r.cmds = append(r.cmds, c)
}

// Run executes a command by ID and reports whether it exists.
func (r *Registry) Run(id string) bool {
	i, ok := r.byID[id]
	if ok {
		r.cmds[i].Run()
	}
	return ok
}

// Get returns a command by ID.
func (r *Registry) Get(id string) (Command, bool) {
	i, ok := r.byID[id]
	if !ok {
		return Command{}, false
	}
	return r.cmds[i], true
}

// All returns every command in registration order.
func (r *Registry) All() []Command { return append([]Command(nil), r.cmds...) }

// Search returns the commands matching query, best first. An empty query returns all commands
// in registration order.
func (r *Registry) Search(query string) []Command {
	query = strings.TrimSpace(query)
	if query == "" {
		return r.All()
	}
	type hit struct {
		c     Command
		score int
	}
	var hits []hit
	for _, c := range r.cmds {
		if s, ok := Score(query, c.Title, c.Category); ok {
			hits = append(hits, hit{c, s})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Command, len(hits))
	for i, h := range hits {
		out[i] = h.c
	}
	return out
}

// groupPenalty ranks a match that needs an item's group (a command's category, a file's folder)
// below matches in titles alone, so "theme" finds "Change Theme" before the entries of the
// Theme group (Phase 1 review: Enter applied "Theme: System" by surprise).
const groupPenalty = 10

// Score ranks an item for query by its title, or by its group and title together at a
// penalty, whichever is better. ok is false when neither matches.
func Score(query, title, group string) (score int, ok bool) {
	score, ok = Match(query, title)
	if group == "" {
		return score, ok
	}
	if s, ok2 := Match(query, group+" "+title); ok2 && (!ok || s-groupPenalty > score) {
		score, ok = s-groupPenalty, true
	}
	return score, ok
}

// Positions returns the indexes of the runes in text that Match pairs with query, for
// highlighting. It is nil when query does not match.
func Positions(query, text string) []int {
	q := []rune(strings.ToLower(query))
	t := []rune(text)
	var out []int
	qi := 0
	for i := 0; i < len(t) && qi < len(q); i++ {
		if unicode.ToLower(t[i]) == q[qi] {
			out = append(out, i)
			qi++
		}
	}
	if qi < len(q) {
		return nil
	}
	return out
}

// Match scores query against text as a case-insensitive subsequence; ok is false when the
// query's runes do not all occur in order. Matches at the start of the text and of words, and
// runs of consecutive matches, score highest; gaps cost a little; shorter texts win ties.
func Match(query, text string) (score int, ok bool) {
	q := []rune(strings.ToLower(query))
	t := []rune(text)
	if len(q) == 0 {
		return 0, true
	}
	qi, prev := 0, -2
	for i := 0; i < len(t) && qi < len(q); i++ {
		if unicode.ToLower(t[i]) != q[qi] {
			continue
		}
		s := 1
		switch {
		case i == 0:
			s += 8
		case wordStart(t[i-1], t[i]):
			s += 6
		}
		if i == prev+1 {
			s += 5
		} else if prev >= 0 {
			s -= min(i-prev-1, 3)
		}
		score += s
		prev = i
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	return score - len(t)/16, true
}

func wordStart(before, at rune) bool {
	switch before {
	case ' ', '-', '_', '/', '.', ':', '&', '+':
		return true
	}
	return unicode.IsLower(before) && unicode.IsUpper(at)
}
