//go:build !windows

package splash

// SystemReducesMotion reports whether the system asks for less motion. Only Windows' setting is
// read; elsewhere the app's own setting decides.
func SystemReducesMotion() bool { return false }
