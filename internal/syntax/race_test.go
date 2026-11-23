//go:build race

package syntax

func init() { raceDetector = true }
