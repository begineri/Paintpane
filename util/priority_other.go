//go:build !windows

package util

// LowerProcessPriority is only done on Windows, where the background work shares the processor
// with everything that starts at logon.
func LowerProcessPriority() error { return nil }
