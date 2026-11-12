//go:build !windows

package wallpaper

import "time"

// startedSoonAfterLogon is only known on Windows; elsewhere only AutostartFlag marks an autostart.
func startedSoonAfterLogon(time.Duration) bool { return false }
