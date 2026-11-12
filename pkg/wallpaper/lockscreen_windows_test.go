//go:build windows
// +build windows

package wallpaper

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A missing file fails before the lock screen is touched, so this is safe on a real machine.
func TestSetLockScreen_ReportsFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.jpg")

	err := (&windowsOS{}).SetLockScreen(missing)

	assert.Error(t, err)
}
