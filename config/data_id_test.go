//go:build !msstore

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The portable exe keeps the data folders and settings it has always used.
func TestDataID_PortableBuildUsesAppName(t *testing.T) {
	assert.Equal(t, "Paintpane", DataID)
	assert.Equal(t, "paintpane", filepath.Base(GetWorkingDir()))
	assert.Equal(t, "Paintpane", filepath.Base(GetAppDir()))
	assert.Equal(t, filepath.Join("fyne", "Paintpane", "preferences.json"),
		relTail(FynePreferencesPath(DataID), 3))
}

// relTail returns the last n elements of path.
func relTail(path string, n int) string {
	var parts []string
	for i := 0; i < n; i++ {
		parts = append([]string{filepath.Base(path)}, parts...)
		path = filepath.Dir(path)
	}
	return filepath.Join(parts...)
}
