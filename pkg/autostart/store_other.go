//go:build !windows

package autostart

// realStore is only there on Windows; elsewhere the switch is not shown (macOS keeps its button
// that opens System Settings).
func realStore() Store { return nil }
