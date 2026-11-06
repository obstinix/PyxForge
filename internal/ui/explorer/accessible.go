package explorer

import (
	"path"

	"fyne.io/fyne/v2"
)

var _ fyne.Accessible = (*row)(nil)

// AccessibilityLabel names a tree row as it reads: the file or folder name, and for a folder,
// that it is one (#49).
func (r *row) AccessibilityLabel() string {
	name := path.Base(r.id)
	if r.dir {
		return name + " (folder)"
	}
	return name
}

// AccessibilityRole is a button's: pressing a row opens the file or the folder.
func (r *row) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
