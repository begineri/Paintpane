//go:build release

package autostart

// Default is the store of a release build: the user's real Run key on Windows, none elsewhere.
func Default() Store { return realStore() }
