//go:build !windows

package build

// toolText leaves a tool's output as it is: on Linux and macOS, tools print in the locale's
// encoding, which is UTF-8 on any current system.
func toolText(line string) string { return line }
