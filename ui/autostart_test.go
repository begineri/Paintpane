package ui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

const fakeExe = `C:\Apps\Paintpane\paintpane.exe`

// useFakeAutostart replaces the Run key with an in-memory one for the test. A test must never touch
// the real list of what starts at logon.
func useFakeAutostart(t *testing.T) *autostart.MemoryStore {
	t.Helper()
	store := &autostart.MemoryStore{}
	prevStore, prevExe := autostartStore, autostartExecutable
	autostartStore = func() autostart.Store { return store }
	autostartExecutable = func() (string, error) { return fakeExe, nil }
	t.Cleanup(func() { autostartStore, autostartExecutable = prevStore, prevExe })
	return store
}

// General › Running › Start with Windows is the program's own switch (S3-6): it shows whether the
// program has registered itself, and turning it on or off registers or removes the entry.
func TestScenarioStartWithWindowsSwitch(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ general, label, oldButton string }{
			"en": {"General", "Start with Windows", "Manage in Windows Settings"},
			"zh": {"常规", "随 Windows 启动", "在 Windows 设置中管理"},
		}[lang]
		store := useFakeAutostart(t)
		s := newScenarioWith(t, wallpaper.GetInstance())
		s.tapText(want.general)

		assert.True(t, s.shows(want.label), "the row is there")
		assert.False(t, s.shows(want.oldButton), "it is a switch now, not a button to Windows' settings")
		require.NotNil(t, s.sa.prefsSM.allWidgets["startWithWindows"])
		assert.False(t, s.switchOn("startWithWindows"), "nothing registered: the switch is off")

		// On: the Run value is written with the exe and the autostart flag.
		s.toggle("startWithWindows")
		s.sa.prefsSM.FlushPending()
		cmd, registered, _ := store.Command()
		assert.True(t, registered)
		assert.Equal(t, `"`+fakeExe+`" --autostart`, cmd)

		// Off: the value is deleted.
		s.toggle("startWithWindows")
		s.sa.prefsSM.FlushPending()
		_, registered, _ = store.Command()
		assert.False(t, registered, "turning it off removes the entry")
	})
}

// The switch shows the registration as it is when the window opens.
func TestScenarioStartWithWindowsShowsTheRegistration(t *testing.T) {
	store := useFakeAutostart(t)
	require.NoError(t, store.SetCommand(autostart.Command(fakeExe)))
	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")
	assert.True(t, s.switchOn("startWithWindows"), "registered: on")

	// Switched off in Windows' Startup apps: the switch reads off.
	store.SetDisabledByUser(true)
	s2 := newScenarioWith(t, wallpaper.GetInstance())
	s2.tapText("General")
	assert.False(t, s2.switchOn("startWithWindows"), "switched off in Windows: off")

	// Turning it on switches the entry back on.
	s2.toggle("startWithWindows")
	s2.sa.prefsSM.FlushPending()
	on, err := autostart.Enabled(store)
	require.NoError(t, err)
	assert.True(t, on)
}

// When the registry cannot be written the person sees what went wrong and the switch shows what is
// really registered.
func TestScenarioStartWithWindowsFailureIsShown(t *testing.T) {
	store := useFakeAutostart(t)
	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")

	store.FailWith(errors.New("access denied"))
	s.toggle("startWithWindows")
	s.sa.prefsSM.FlushPending()
	flushUI() // the error dialog and the rebuild run after the apply, on the UI thread

	assert.True(t, overlayShows(s, "Something went wrong"), "an error dialog opens")
	assert.True(t, overlayShows(s, "access denied"), "with the original text")

	store.FailWith(nil)
	_, registered, _ := store.Command()
	assert.False(t, registered, "nothing was registered")
	assert.False(t, s.switchOn("startWithWindows"), "so the switch is off")
}

// Without a store (anything but Windows, or the Microsoft Store build) the row is not there.
func TestScenarioStartWithWindowsHiddenWithoutAStore(t *testing.T) {
	useFakeAutostart(t)
	autostartStore = func() autostart.Store { return nil }
	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")
	assert.False(t, s.shows("Start with Windows"))
}

func TestRefreshAutostartKeepsTheRegistrationCurrent(t *testing.T) {
	store := useFakeAutostart(t)
	refreshAutostart()
	_, registered, _ := store.Command()
	assert.False(t, registered, "no registration: refreshing creates none")

	require.NoError(t, store.SetCommand(`"D:\Old\paintpane.exe"`))
	refreshAutostart()
	cmd, _, _ := store.Command()
	assert.Equal(t, autostart.Command(fakeExe), cmd, "it points at the running exe, with the flag")
}
