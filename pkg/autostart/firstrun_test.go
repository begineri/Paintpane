package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// portableExe is an exe in a folder of the test's own, with no SetupMarker next to it.
func portableExe(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "paintpane.exe")
}

// installedExe is an exe with the installer's SetupMarker next to it.
func installedExe(t *testing.T) string {
	t.Helper()
	exe := portableExe(t)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(exe), SetupMarker), []byte("[Paintpane]\r\nStartWithWindows=0\r\n"), 0o644))
	return exe
}

func TestInstalledBySetupLooksForTheMarkerNextToTheExe(t *testing.T) {
	assert.False(t, InstalledBySetup(portableExe(t)), "the portable exe has no marker")
	assert.True(t, InstalledBySetup(installedExe(t)))
	assert.False(t, InstalledBySetup(""))

	dirExe := portableExe(t)
	require.NoError(t, os.Mkdir(filepath.Join(filepath.Dir(dirExe), SetupMarker), 0o755))
	assert.False(t, InstalledBySetup(dirExe), "a folder of that name is not the marker")
}

func TestFirstRunRegistersThePortableExe(t *testing.T) {
	s := &MemoryStore{}
	exe := portableExe(t)
	got, err := RegisterOnFirstRun(s, exe, false)
	require.NoError(t, err)
	assert.Equal(t, FirstRunRegistered, got)
	cmd, registered, _ := s.Command()
	assert.True(t, registered)
	assert.Equal(t, Command(exe), cmd, `"<exe>" --autostart, the same as the switch writes`)
	on, _ := Enabled(s)
	assert.True(t, on)
}

func TestFirstRunLeavesAnUpgradeAlone(t *testing.T) {
	s := &MemoryStore{}
	got, err := RegisterOnFirstRun(s, portableExe(t), true)
	require.NoError(t, err)
	assert.Equal(t, FirstRunAcceptedBefore, got)
	_, registered, _ := s.Command()
	assert.False(t, registered, "someone who accepted an earlier version is not registered on upgrade")
}

func TestFirstRunLeavesTheInstallersAnswerAlone(t *testing.T) {
	s := &MemoryStore{}
	got, err := RegisterOnFirstRun(s, installedExe(t), false)
	require.NoError(t, err)
	assert.Equal(t, FirstRunInstallerAsked, got)
	_, registered, _ := s.Command()
	assert.False(t, registered, "the person unticked the installer's task: not registered")
}

func TestFirstRunKeepsAnExistingRegistration(t *testing.T) {
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(`"D:\Elsewhere\paintpane.exe" --autostart`))
	s.SetDisabledByUser(true)
	got, err := RegisterOnFirstRun(s, portableExe(t), false)
	require.NoError(t, err)
	assert.Equal(t, FirstRunAlreadyRegistered, got)
	cmd, _, _ := s.Command()
	assert.Equal(t, `"D:\Elsewhere\paintpane.exe" --autostart`, cmd, "not rewritten (Refresh does that at start)")
	on, _ := Enabled(s)
	assert.False(t, on, "an entry switched off in Windows stays off")
}

func TestFirstRunErrors(t *testing.T) {
	_, err := RegisterOnFirstRun(nil, portableExe(t), false)
	assert.Error(t, err, "no store")

	boom := errors.New("access denied")
	s := &MemoryStore{err: boom}
	_, err = RegisterOnFirstRun(s, portableExe(t), false)
	assert.ErrorIs(t, err, boom)

	got, err := RegisterOnFirstRun(s, portableExe(t), true)
	assert.NoError(t, err, "an upgrade does not even read the store")
	assert.Equal(t, FirstRunAcceptedBefore, got)
}

func TestFirstRunResultsReadInTheLog(t *testing.T) {
	for _, f := range []FirstRun{FirstRunRegistered, FirstRunAcceptedBefore, FirstRunInstallerAsked, FirstRunAlreadyRegistered} {
		assert.NotEqual(t, "unknown", f.String())
	}
}
