//go:build !release

package autostart

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A development build never writes the real Run key: its default store is the in-memory one.
func TestDevBuildsDoNotUseTheRealRunKey(t *testing.T) {
	s := Default()
	if runtime.GOOS != "windows" {
		assert.Nil(t, s)
		return
	}
	require.NotNil(t, s)
	_, isMemory := s.(*MemoryStore)
	assert.True(t, isMemory, "the default store of a development build only remembers")

	// Use it, and leave it as found.
	t.Cleanup(func() { _ = s.DeleteCommand() })
	require.NoError(t, SetEnabled(s, exe, true))
	on, err := Enabled(s)
	require.NoError(t, err)
	assert.True(t, on)
	require.NoError(t, SetEnabled(s, exe, false))
}
