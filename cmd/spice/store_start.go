package main

import (
	"os"
	"time"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	"github.com/dixieflatline76/Spice/v2/pkg/storeimport"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// What the Microsoft Store edition does at start before anything else (docs/store-feasibility.md
// §12). The portable exe skips all of it.

const (
	// giveWayWait is how long the Store edition, started at logon, waits for the portable exe that
	// is set to start with Windows too.
	giveWayWait = 60 * time.Second
	// afterImportWait is how long the start after an import waits for the previous process to let
	// go of the single-instance lock.
	afterImportWait = 20 * time.Second
)

// storeGivesWay reports whether the Store edition should quit because the portable exe is taking
// this logon (autostart.OtherCopyAtLogon). The reason goes to the log only; nothing is shown.
func storeGivesWay() bool {
	if !config.IsStoreDistribution() || !wallpaper.StartedByWindowsAtLogon() {
		return false
	}
	self, _ := os.Executable()
	exe, ok := autostart.OtherCopyAtLogon(autostart.Default(), self)
	if !ok {
		return false
	}
	log.Printf("[Store] Started at logon while the portable exe (%s) is set to start with Windows too; waiting up to %v for it", exe, giveWayWait)
	if autostart.WaitFor(instanceRunning, giveWayWait, 2*time.Second, time.Sleep) {
		log.Printf("[Store] The portable exe is running; the Store edition quits and leaves this logon to it. Turn off Start with Windows in one of them to choose")
		return true
	}
	log.Printf("[Store] The portable exe did not start within %v; the Store edition carries on", giveWayWait)
	return false
}

// startedAfterImport reports whether this start is the restart after an import.
func startedAfterImport() bool {
	for _, a := range os.Args[1:] {
		if a == storeimport.RestartFlag {
			return true
		}
	}
	return false
}

// acquireLockAfterImport takes the single-instance lock, waiting for the process that imported to
// quit.
func acquireLockAfterImport() (bool, error) {
	var err error
	ok := autostart.WaitFor(func() bool {
		var got bool
		got, err = acquireLock()
		return got || err != nil
	}, afterImportWait, 250*time.Millisecond, time.Sleep)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// applyStoreImport puts the settings an import left (storeimport.ApplyPending) in place before Fyne
// reads them.
func applyStoreImport() {
	if !config.IsStoreDistribution() {
		return
	}
	applied, err := storeimport.ApplyPending(storeimport.PendingPath(), config.FynePreferencesPath(config.DataID))
	if err != nil {
		log.Printf("[Store] Could not apply the imported settings: %v", err)
	} else if applied {
		log.Printf("[Store] Applied the settings imported from the portable exe")
	}
}
