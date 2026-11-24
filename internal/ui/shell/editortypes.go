package shell

// Breakpoint is the breakpoints of one file, set in the editor (PXF-FEAT-004). Its fields are
// kept in the saved workspace state.
type Breakpoint struct {
	File  string // absolute
	Lines []int  // 1-based, in order
}

// Diagnostic is one problem in a file, from a language server or a build: 1-based line and
// column, as the Problems panel shows them.
type Diagnostic struct {
	Line, Col int
	Severity  int // 1 error, 2 warning, 3 info, 4 hint
	Message   string
	Source    string
}

// FindOptions are the find bar's toggles.
type FindOptions struct {
	MatchCase bool
	WholeWord bool
}

// FindResult is where a search ended: the match the cursor is on, counted from 1, of Total
// matches in the document; Current is 0 when nothing matched.
type FindResult struct {
	Found   bool
	Current int
	Total   int
}
