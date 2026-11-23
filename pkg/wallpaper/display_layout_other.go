//go:build !windows

package wallpaper

// displayLayoutKey is not implemented outside Windows: "" makes the monitor watcher check fully
// every time, as before.
func displayLayoutKey() string { return "" }
