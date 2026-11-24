package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests use a MemoryStore as the Run key and real files in a temporary folder as the copies
// of the program. They never touch the registry.

// copies makes two copies of the program in a temporary folder: the one the person chose to start
// at logon, and another one started from elsewhere (the 2026-11-21 measurement copy).
func copies(t *testing.T) (chosen, other string) {
	t.Helper()
	dir := t.TempDir()
	chosen = filepath.Join(dir, "Downloads", "paintpane-v1.3.0.exe")
	other = filepath.Join(dir, "scratch", "paintpane.exe")
	for _, p := range []string{chosen, other} {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("exe"), 0o600))
	}
	return chosen, other
}

func registeredExe(t *testing.T, s *MemoryStore) string {
	t.Helper()
	cmd, registered, err := s.Command()
	require.NoError(t, err)
	require.True(t, registered, "still registered")
	return cmd
}

// The 2026-11-21 incident: another copy started once must not take the logon start over while the
// registered exe is still there.
func TestRefreshLeavesAnExistingOtherCopyAlone(t *testing.T) {
	chosen, other := copies(t)
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(Command(chosen)))

	result, err := Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshOtherCopy, result)
	assert.Equal(t, Command(chosen), registeredExe(t, s), "not rewritten")

	// Also when the other copy's entry was written by hand, without the flag or quotes.
	require.NoError(t, s.SetCommand(chosen))
	result, err = Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshOtherCopy, result)
	assert.Equal(t, chosen, registeredExe(t, s))
}

// Moved, renamed or deleted: the registered exe is gone, so the running copy takes over.
func TestRefreshRepointsWhenTheRegisteredExeIsGone(t *testing.T) {
	chosen, other := copies(t)
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(Command(chosen)))
	require.NoError(t, os.Remove(chosen))

	result, err := Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshRewritten, result)
	assert.Equal(t, Command(other), registeredExe(t, s))

	result, err = Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshCurrent, result, "already current: no write")
}

// A folder of the registered name is not the program.
func TestRefreshRepointsWhenTheRegisteredPathIsAFolder(t *testing.T) {
	chosen, other := copies(t)
	require.NoError(t, os.Remove(chosen))
	require.NoError(t, os.Mkdir(chosen, 0o755))
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(Command(chosen)))

	result, err := Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshRewritten, result)
}

// Turning the switch on registers the running copy, even over another copy that still exists.
func TestTurningTheSwitchOnRegistersThisCopy(t *testing.T) {
	chosen, other := copies(t)
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(Command(chosen)))
	s.SetDisabledByUser(true)

	require.NoError(t, SetEnabled(s, other, true))
	assert.Equal(t, Command(other), registeredExe(t, s))
	on, err := Enabled(s)
	require.NoError(t, err)
	assert.True(t, on)
}

// Quotes and arguments in the Run value: only the program is compared.
func TestRefreshReadsQuotesAndArguments(t *testing.T) {
	chosen, other := copies(t)
	s := &MemoryStore{}
	for _, cmd := range []string{
		`"` + chosen + `" --autostart`,
		`  "` + chosen + `"   --autostart --other  `,
		`"` + chosen + `"`,
	} {
		require.NoError(t, s.SetCommand(cmd))
		result, err := Refresh(s, other)
		require.NoError(t, err)
		assert.Equal(t, RefreshOtherCopy, result, cmd)
		assert.Equal(t, cmd, registeredExe(t, s), "left exactly as it was")
	}

	// This very program written without the flag (an older installer): the flag is added.
	require.NoError(t, s.SetCommand(`"`+other+`"`))
	result, err := Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshRewritten, result)
	assert.Equal(t, Command(other), registeredExe(t, s))

	// Unquoted, with arguments, and gone.
	gone := filepath.Join(filepath.Dir(chosen), "old.exe")
	require.NoError(t, s.SetCommand(gone+" --autostart"))
	result, err = Refresh(s, other)
	require.NoError(t, err)
	assert.Equal(t, RefreshRewritten, result)
	assert.Equal(t, Command(other), registeredExe(t, s))
}

// The Run value names this very program in another form: different case, forward slashes, dot
// segments, a relative path. It is this program, so it is not "another copy" and not "gone".
func TestRefreshKnowsThisProgramInAnotherForm(t *testing.T) {
	_, self := copies(t)
	dir := filepath.Dir(self)
	t.Chdir(dir)
	forms := []string{
		strings.ToUpper(self),
		filepath.ToSlash(self),
		filepath.Join(dir, ".", "sub", "..", "paintpane.exe"),
		"paintpane.exe",
		`.\PaintPane.EXE`,
	}
	for _, form := range forms {
		assert.True(t, SamePath(form, self), form)
		assert.True(t, FileExists(form), form)

		s := &MemoryStore{}
		require.NoError(t, s.SetCommand(`"`+form+`" --autostart`))
		result, err := Refresh(s, self)
		require.NoError(t, err)
		assert.NotEqual(t, RefreshOtherCopy, result, "%s is this program", form)
		cmd := registeredExe(t, s)
		assert.True(t, SamePath(CommandExe(cmd), self), "still starts this program: %s", cmd)
		assert.True(t, strings.HasSuffix(cmd, " "+Flag))
	}
}

func TestSamePath(t *testing.T) {
	chosen, other := copies(t)
	assert.True(t, SamePath(chosen, strings.ToLower(chosen)))
	assert.True(t, SamePath(` "`+chosen+`" `, chosen), "quotes and spaces around it")
	assert.False(t, SamePath(chosen, other))
	assert.False(t, SamePath("", other))
	assert.False(t, SamePath(chosen, ""))
	assert.True(t, SamePath(`D:\Gone\paintpane.exe`, `d:\gone\PAINTPANE.exe`), "missing files compare by name")
	assert.False(t, FileExists(""))
	assert.False(t, FileExists(filepath.Dir(chosen)), "a folder")
}
