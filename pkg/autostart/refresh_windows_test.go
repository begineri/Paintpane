//go:build windows

package autostart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// shortPath returns the 8.3 short form of an existing path, or "" when the volume has no short
// names.
func shortPath(t *testing.T, long string) string {
	t.Helper()
	p, err := windows.UTF16PtrFromString(long)
	require.NoError(t, err)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	require.NoError(t, err)
	short := windows.UTF16ToString(buf[:n])
	if short == long {
		return ""
	}
	return short
}

// The Run value names this program by its 8.3 short path (as some tools write it).
func TestRefreshKnowsThisProgramByItsShortPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Paintpane portable copy")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	self := filepath.Join(dir, "paintpane-v1.3.0.exe")
	require.NoError(t, os.WriteFile(self, []byte("exe"), 0o600))
	short := shortPath(t, self)
	if short == "" {
		t.Skip("8.3 short names are turned off on this volume")
	}
	t.Logf("short path: %s", short)

	assert.True(t, SamePath(short, self))

	other := filepath.Join(t.TempDir(), "paintpane.exe")
	require.NoError(t, os.WriteFile(other, []byte("exe"), 0o600))
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(`"`+short+`" --autostart`))

	result, err := Refresh(s, self)
	require.NoError(t, err)
	assert.Equal(t, RefreshRewritten, result, "the same program, written differently: tidied, not another copy")

	require.NoError(t, s.SetCommand(`"`+short+`" --autostart`))
	result, err = Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshOtherCopy, result, "seen from another copy, the short path names an existing exe")
	cmd, _, _ := s.Command()
	assert.Equal(t, `"`+short+`" --autostart`, cmd)
}
