// Package shipdocs lists the documents PyxForge ships with the program (decision D22), so Help
// can open them offline. The Windows packages put them in a docs folder beside the program; the
// Linux packages in /usr/share/doc/pyxforge. tools/winpkg, tools/linuxpkg and the shell all read
// this list.
package shipdocs

// Doc is one shipped document.
type Doc struct {
	Title string // as Help lists it
	Name  string // its file name in the packages
	Repo  string // its path in the repository, slash-separated
}

// List is every shipped document, in the order Help shows them.
var List = []Doc{
	{"Keyboard shortcuts", "KEYMAP.md", "docs/architecture/KEYMAP.md"},
	{"Features and their status", "FEATURES.md", "docs/FEATURES.md"},
	{"Installing and updating", "INSTALL.md", "docs/INSTALL.md"},
	{"Accessibility", "ACCESSIBILITY.md", "docs/ACCESSIBILITY.md"},
	{"Changelog", "CHANGELOG.md", "CHANGELOG.md"},
}
