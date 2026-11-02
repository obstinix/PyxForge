//go:build !windows

package managed

import (
	"context"
	"errors"
)

// runElevated is only needed for Windows installers.
func runElevated(context.Context, string, []string) error {
	return errors.New("installers run with administrator rights only on Windows")
}
