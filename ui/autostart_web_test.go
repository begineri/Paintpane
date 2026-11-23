//go:build !appstore && !msstore

package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// These tests check Start with Windows in the portable exe. The Store builds have no such switch
// (Windows starts them through the package's StartupTask), so they are left out there.

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
