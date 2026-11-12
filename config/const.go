package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AppVersion is the version of the service.
var AppVersion string // Or get it from version.txt during build

// AppName is the name of the service.
const AppName = "Paintpane"

// LogWinSubDir is the sub directory for the log files on windows.
var LogWinSubDir = AppName

// LogSubDir is the sub directory for the log files.
var LogSubDir = "." + strings.ToLower(AppName)

// LogExt is the extension for the log files.
var LogExt = ".log"

// GetWorkingDir returns the working directory for transient/cache data.
// Uses os.UserCacheDir() which is sandbox-safe on macOS and idiomatic on all platforms:
//   - macOS: ~/Library/Caches (sandbox auto-maps to container)
//   - Windows: %LocalAppData%
//   - Linux: ~/.cache (XDG_CACHE_HOME)
func GetWorkingDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir() // Fallback
	}
	return filepath.Join(cacheDir, strings.ToLower(AppName))
}

// GetCacheDir returns the directory for cached gallery and curation files. Use it instead of
// joining "cache" to GetWorkingDir, so a cache moved elsewhere by a link still works.
func GetCacheDir() string {
	return ResolveLinkedDir(filepath.Join(GetWorkingDir(), "cache"))
}

// ResolveLinkedDir returns the directory dir links to, or dir if it is not a link. A cache can be
// moved to another drive with a junction, and on some Windows machines new directories cannot be
// created through one, so callers should work on the target directly.
func ResolveLinkedDir(dir string) string {
	target, err := os.Readlink(dir)
	if err != nil {
		return dir
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(dir), target)
	}
	return target
}

var appDir string

func init() {
	switch runtime.GOOS {
	case "windows":
		userCacheDir, _ := os.UserCacheDir()
		appDir = filepath.Join(userCacheDir, AppName)
	case "darwin":
		// macOS: Use standard Application Support for sandbox compliance.
		// os.UserConfigDir() correctly points to the sandbox container when enabled.
		configDir, err := os.UserConfigDir()
		if err == nil {
			appDir = filepath.Join(configDir, AppName)
		} else {
			// Fallback to home dir if config dir is unavailable
			userHomeDir, _ := os.UserHomeDir()
			appDir = filepath.Join(userHomeDir, "."+strings.ToLower(AppName))
		}
	default:
		userHomeDir, _ := os.UserHomeDir()
		appDir = filepath.Join(userHomeDir, "."+strings.ToLower(AppName))
	}

	// We don't panic here because it crashes the app before logging starts.
	// We'll attempt to create it, but failures will be handled/logged later
	// when the app tries to actually write to it.
	_ = os.MkdirAll(appDir, 0755)
}

// GetAppDir returns the persistent application directory for config and logs.
func GetAppDir() string {
	return appDir
}
