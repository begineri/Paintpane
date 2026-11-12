//go:build !release

package autostart

import "runtime"

// Default is the store of a development build. It never touches the real Run key (a dev build is
// started from a build folder, and registering it would start it at every logon): it remembers the
// registration for as long as the program runs and logs what a release build would write. Only
// Windows has a switch, so elsewhere there is no store.
func Default() Store {
	if runtime.GOOS != "windows" {
		return nil
	}
	return devMemory
}

var devMemory = &MemoryStore{logWrites: true}
