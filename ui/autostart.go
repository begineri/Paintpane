package ui

import (
	"fyne.io/fyne/v2"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// "Start with Windows" (S3-6) is the program's own switch. It shows whether the program has
// registered itself in the user's Run key (pkg/autostart). It is on by default: the first licence
// acceptance registers the program once (registerAutostartOnFirstRun). Tests replace the store and
// the exe path below, so they never touch the real Run key; a development build's default store
// only remembers.
var (
	autostartStore      = autostart.Default
	autostartExecutable = autostart.Executable
)

// autostartAvailable reports whether this build has the switch: Windows, outside the Microsoft
// Store (which has its own start-up mechanism).
func autostartAvailable() bool {
	return autostartStore() != nil && !config.IsStoreDistribution()
}

// autostartOn reads the switch's state from the registration.
func autostartOn() bool {
	on, err := autostart.Enabled(autostartStore())
	if err != nil {
		utilLog.Printf("[Autostart] Could not read the registration: %v", err)
		return false
	}
	return on
}

// setAutostart registers or unregisters the program. When that fails the person sees the error and
// the page is rebuilt, so the switch shows what is really registered.
func (sa *SpiceApp) setAutostart(on bool) {
	exe, err := autostartExecutable()
	if err == nil {
		err = autostart.SetEnabled(autostartStore(), exe, on)
	}
	if err == nil {
		utilLog.Printf("[Autostart] Start with Windows is now %v", on)
		return
	}
	utilLog.Printf("[Autostart] Could not change the registration: %v", err)
	onUIThread(func() { // after the current apply, which the rebuild would otherwise re-enter
		if sa.prefsSM != nil {
			sa.prefsSM.ShowError(err)
		}
		sa.RebuildPreferencesContent("")
	})
}

// autostartFirstRunKey is the preference that records that the first-run registration was handled,
// whatever it decided, so it happens at most once: after that, turning the switch off (or switching
// the entry off in Windows' Startup apps page) is never undone by the program.
const autostartFirstRunKey = "autostart_first_run_done"

// registerAutostartOnFirstRun makes "Start with Windows" on by default for the portable exe (S3-6):
// called when the licence is accepted, it registers the program once if this is a first run
// (acceptedBefore false). An upgrade, an installer that already asked, or an existing registration
// leave things as they are (autostart.RegisterOnFirstRun). Nothing happens where the switch is not
// available (outside Windows, the Microsoft Store build).
func registerAutostartOnFirstRun(prefs fyne.Preferences, acceptedBefore bool) {
	if prefs.Bool(autostartFirstRunKey) || !autostartAvailable() {
		return
	}
	prefs.SetBool(autostartFirstRunKey, true) // once, even if registering fails below
	exe, err := autostartExecutable()
	if err != nil {
		utilLog.Printf("[Autostart] First run: could not find the program: %v", err)
		return
	}
	result, err := autostart.RegisterOnFirstRun(autostartStore(), exe, acceptedBefore)
	if err != nil {
		utilLog.Printf("[Autostart] First run: could not register: %v", err)
		return
	}
	utilLog.Printf("[Autostart] First run: %v", result)
}

// refreshAutostart keeps an existing registration pointing at this program, so an update that
// moved the exe, or an installer that registered it without --autostart, does not leave a stale
// entry. It never creates a registration.
func refreshAutostart() {
	if !autostartAvailable() {
		return
	}
	exe, err := autostartExecutable()
	if err != nil {
		return
	}
	changed, err := autostart.Refresh(autostartStore(), exe)
	if err != nil {
		utilLog.Printf("[Autostart] Could not refresh the registration: %v", err)
	} else if changed {
		utilLog.Printf("[Autostart] The registration now points at %s", exe)
	}
}
