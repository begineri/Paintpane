//go:build !windows

package ui

// letBrowserComeForward does nothing outside Windows, which has no foreground lock.
func letBrowserComeForward() bool { return true }
