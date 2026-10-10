//go:build !windows

package main

// releaseOwnConsole does nothing outside Windows: a terminal-less launch has no console.
func releaseOwnConsole() bool { return false }
