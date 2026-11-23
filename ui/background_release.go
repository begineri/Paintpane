//go:build release

package ui

// devBackground is the dev builds' background mode (background_dev_windows.go); release builds
// never have it, so their windows always come to the front.
func devBackground() bool { return false }
