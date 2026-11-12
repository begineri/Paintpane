package wallpaper

import (
	"os"
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/zalando/go-keyring"
)

// TestMain pins the language because many tests assert English UI strings,
// and i18n otherwise follows the OS locale of whoever runs them.
//
// It also replaces the OS credential store with an in-memory one: Config reads the wallhaven
// and Pexels API keys from it, and tests must never see the keys of whoever runs them.
func TestMain(m *testing.M) {
	i18n.SetLanguage("en")
	keyring.MockInit()
	// A test run right after logon is not an autostart: no test should wait 60-120 seconds.
	detectAutostart = func() bool { return false }
	os.Exit(m.Run())
}
