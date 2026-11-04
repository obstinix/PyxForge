//go:build !windows

package toolchain

// storeStub is Windows-only: no other system has App Execution Aliases.
func storeStub(string) bool { return false }
