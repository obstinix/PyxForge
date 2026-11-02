// Package trash moves files and folders to the system's trash instead of deleting them, so a
// deletion in PyxForge can be undone from the Recycle Bin (Windows) or the desktop's Trash
// (Linux, the freedesktop.org Trash specification). It never deletes anything for good: where
// the trash cannot take a file, it says so and leaves the file where it is.
package trash

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Move puts path in the trash.
func Move(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(abs); err != nil {
		return err
	}
	return move(abs)
}

// ErrOtherDrive means the file is on another file system than the trash, which can then only
// be emptied of it by copying; PyxForge leaves it alone instead.
var ErrOtherDrive = errors.New("it is on another drive than the trash; delete it from your file manager")

// toTrash moves abs into the freedesktop.org trash folder dir: the file goes to dir/files under
// a name not yet taken there, and dir/info gets a .trashinfo saying where it came from and when,
// which file managers read to restore it.
func toTrash(dir, abs string) error {
	files, info := filepath.Join(dir, "files"), filepath.Join(dir, "info")
	for _, d := range []string{files, info} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	base := filepath.Base(abs)
	name := base
	var record *os.File
	for i := 2; ; i++ {
		// Creating the .trashinfo first, exclusively, claims the name (the specification's way).
		f, err := os.OpenFile(filepath.Join(info, name+".trashinfo"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			record = f
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		name = base + "." + strconv.Itoa(i)
	}
	u := url.URL{Path: filepath.ToSlash(abs)}
	_, werr := fmt.Fprintf(record, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", u.EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
	cerr := record.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(record.Name())
		return err
	}
	if err := os.Rename(abs, filepath.Join(files, name)); err != nil {
		os.Remove(record.Name())
		var le *os.LinkError
		if errors.As(err, &le) && isCrossDevice(le.Err) {
			return ErrOtherDrive
		}
		return err
	}
	return nil
}
