package autostart

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exe = `C:\Program Files\Paintpane\paintpane.exe`

func TestCommandQuotesThePathAndPassesTheFlag(t *testing.T) {
	assert.Equal(t, `"C:\Program Files\Paintpane\paintpane.exe" --autostart`, Command(exe))
	assert.Equal(t, "--autostart", Flag, "pkg/wallpaper and assets-src/installer/Paintpane.iss use this exact flag")
	assert.Equal(t, "Paintpane", ValueName, "the installer writes the same value name")
}

func TestEnabledFollowsTheRegistration(t *testing.T) {
	s := &MemoryStore{}
	on, err := Enabled(s)
	require.NoError(t, err)
	assert.False(t, on, "nothing registered: off")

	require.NoError(t, SetEnabled(s, exe, true))
	on, err = Enabled(s)
	require.NoError(t, err)
	assert.True(t, on)
	cmd, registered, _ := s.Command()
	assert.True(t, registered)
	assert.Equal(t, Command(exe), cmd, `turning it on writes "<exe>" --autostart`)

	require.NoError(t, SetEnabled(s, exe, false))
	on, _ = Enabled(s)
	assert.False(t, on)
	_, registered, _ = s.Command()
	assert.False(t, registered, "turning it off deletes the value")

	require.NoError(t, SetEnabled(s, exe, false), "deleting what is not there is fine")
}

func TestSwitchedOffInWindowsReadsAsOffAndTurningOnUndoesIt(t *testing.T) {
	s := &MemoryStore{}
	require.NoError(t, s.SetCommand(Command(exe)))
	s.disabled = true // the person switched it off in Windows' Startup apps

	on, err := Enabled(s)
	require.NoError(t, err)
	assert.False(t, on, "registered but switched off in Windows: the switch reads off")

	require.NoError(t, SetEnabled(s, exe, true))
	on, _ = Enabled(s)
	assert.True(t, on, "turning it on switches the entry back on")
	assert.False(t, s.disabled)
}

func TestNoStoreMeansOffAndAnError(t *testing.T) {
	on, err := Enabled(nil)
	assert.NoError(t, err)
	assert.False(t, on)
	assert.Error(t, SetEnabled(nil, exe, true))
	changed, err := Refresh(nil, exe)
	assert.NoError(t, err)
	assert.False(t, changed)
}

func TestStoreErrorsAreReturned(t *testing.T) {
	boom := errors.New("access denied")
	s := &MemoryStore{err: boom}
	_, err := Enabled(s)
	assert.ErrorIs(t, err, boom)
	assert.ErrorIs(t, SetEnabled(s, exe, true), boom)
	assert.ErrorIs(t, SetEnabled(s, exe, false), boom)
}

func TestRefreshKeepsARegistrationCurrentButNeverCreatesOne(t *testing.T) {
	s := &MemoryStore{}
	changed, err := Refresh(s, exe)
	require.NoError(t, err)
	assert.False(t, changed, "no registration: nothing is created")
	_, registered, _ := s.Command()
	assert.False(t, registered)

	// An older installer registered it without the flag, from the old folder.
	require.NoError(t, s.SetCommand(`"D:\Old\paintpane.exe"`))
	changed, err = Refresh(s, exe)
	require.NoError(t, err)
	assert.True(t, changed)
	cmd, _, _ := s.Command()
	assert.Equal(t, Command(exe), cmd)

	changed, err = Refresh(s, exe)
	require.NoError(t, err)
	assert.False(t, changed, "already current: no write")

	// A switched-off entry stays switched off.
	s.disabled = true
	require.NoError(t, s.SetCommand(`"D:\Old\paintpane.exe"`))
	_, err = Refresh(s, exe)
	require.NoError(t, err)
	assert.True(t, s.disabled, "Refresh does not turn it back on")
}
