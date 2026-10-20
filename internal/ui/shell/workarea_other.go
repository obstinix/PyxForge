//go:build !windows

package shell

// workArea is not known outside Windows; the window keeps the size asked for.
func workArea() (w, h int, scale float32, ok bool) { return 0, 0, 0, false }
