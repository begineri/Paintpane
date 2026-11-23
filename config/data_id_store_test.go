//go:build msstore

package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Store build never uses the portable exe's folders or settings.
func TestDataID_StoreBuildHasItsOwnData(t *testing.T) {
	assert.Equal(t, "PaintpaneStore", DataID)
	for _, dir := range []string{GetWorkingDir(), GetAppDir(), filepath.Dir(FynePreferencesPath(DataID))} {
		assert.False(t, strings.EqualFold(filepath.Base(dir), AppName), "%s is the portable exe's folder", dir)
		assert.True(t, strings.EqualFold(filepath.Base(dir), DataID), "%s", dir)
	}
}
