//go:build !release && !windows

package ui

// devBackground tells whether background mode is on; see background_dev_windows.go.
// Elsewhere it is not available.
func devBackground() bool { return false }
