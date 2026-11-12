//go:build !linux

package hotkey

import (
	"time"

	"sync"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
	"golang.design/x/hotkey"
)

// Constants shared by Windows and Darwin (and potentially others in !linux)
// modBase and modExtra are defined in platform-specific files.
const (
	keyRight = hotkey.KeyRight
	keyLeft  = hotkey.KeyLeft
	keyUp    = hotkey.KeyUp
	keyDown  = hotkey.KeyDown
	keyP     = hotkey.KeyP
	keyO     = hotkey.KeyO
	keyD     = hotkey.KeyD

	// Anchor shortcuts (WASD + Q/E)
	keyQ = hotkey.KeyQ // Auto
	keyW = hotkey.KeyW // Up
	keyE = hotkey.KeyE // Center
	keyA = hotkey.KeyA // Left
	keyS = hotkey.KeyS // Down
	// keyD already defined above for Sync — reused for Anchor Right

	// Shuffle + Info shortcuts
	keyR = hotkey.KeyR // Shuffle
	keyI = hotkey.KeyI // Info
)

// GetMonitorIDFromKey is a platform-specific helper.
// On Windows it polls number keys. On Darwin/Linux it's a stub or different impl.
// Since hotkey_windows.go has the real impl, we need a stub here for Darwin?
// No, if hotkey_windows.go has it, and this file is !linux, then on Windows we have TWO generic definitions?
// No, hotkey_windows has the function.
// On Darwin, we need this function to exist.
// So we should define a Stub for it in hotkey_darwin.go, NOT here.
// Or define it here as a var that can be overridden? No.

// combo is a hotkey before it is registered: modifiers plus one key.
type combo struct {
	mods []hotkey.Modifier
	key  hotkey.Key
}

func newCombo(mods []hotkey.Modifier, key hotkey.Key) combo { return combo{mods: mods, key: key} }

// registration is a registered hotkey; registerHotkey (per platform) creates it.
type registration interface {
	Unregister() error
}

var (
	registeredHotkeys []registration
	hkMu              sync.Mutex

	// Debouncing and Background Work
	refreshChan = make(chan ui.PluginManager, 1)
	initOnce    sync.Once
)

// initWorker starts a dedicated background goroutine to handle hotkey refreshes.
// This ensures that OS-level hook registration (Carbon/Cocoa/User32) is decoupled
// from the UI thread and debounced to prevent instability.
func initWorker() {
	go func() {
		var debounceTimer *time.Timer
		var latestMgr ui.PluginManager

		for {
			select {
			case mgr := <-refreshChan:
				latestMgr = mgr
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.NewTimer(250 * time.Millisecond)
			case <-func() <-chan time.Time {
				if debounceTimer == nil {
					return nil
				}
				return debounceTimer.C
			}():
				debounceTimer = nil
				if latestMgr != nil {
					doStartListeners(latestMgr)
				}
			}
		}
	}()
}

// StopListeners unregisters all currently active hotkeys.
func StopListeners() {
	hkMu.Lock()
	defer hkMu.Unlock()

	if len(registeredHotkeys) == 0 {
		return
	}

	log.Printf("[Hotkey] Stopping %d listeners and unregistering hooks...", len(registeredHotkeys))
	for _, hk := range registeredHotkeys {
		if err := hk.Unregister(); err != nil {
			log.Debugf("[Hotkey] Failed to unregister during stop: %v", err)
		}
	}
	registeredHotkeys = nil
}

// StartListeners initializes and starts the global hotkey listeners.
func StartListeners(mgr ui.PluginManager) {
	initOnce.Do(initWorker)

	// Push to the background worker to handle debounced registration
	select {
	case refreshChan <- mgr:
	default:
		// Worker is already queued for an update
	}
}

func doStartListeners(mgr ui.PluginManager) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Hotkey] CRITICAL: Recovered from panic during listener registration: %v", r)
		}
	}()

	// First, stop any existing listeners to ensure a clean state if re-initialized
	StopListeners()

	wp := wallpaper.GetInstance()
	if wp != nil && wp.GetShortcutsDisabled() {
		log.Printf("[Hotkey] Global shortcuts are disabled in preferences. Skipping registration.")
		return
	}

	hkMu.Lock()
	defer hkMu.Unlock()

	// --- 2. Global Handlers (Base + Extra Modifier + Key) ---
	// These apply to ALL monitors simultaneously.
	hkGlobalNext := newCombo([]hotkey.Modifier{modBase, modExtra}, keyRight)
	hkGlobalPrev := newCombo([]hotkey.Modifier{modBase, modExtra}, keyLeft)
	hkGlobalSync := newCombo([]hotkey.Modifier{modBase, modExtra}, keyD)
	hkOpts := newCombo([]hotkey.Modifier{modBase, modExtra}, keyO)

	// --- 1. Targeted Handlers (Base Modifier + [1-9] + Key) ---
	// These only trigger if a number key 1-9 is held.
	hkTargetedNext := newCombo([]hotkey.Modifier{modBase}, keyRight)
	hkTargetedPrev := newCombo([]hotkey.Modifier{modBase}, keyLeft)
	hkTargetedTrash := newCombo([]hotkey.Modifier{modBase}, keyDown)
	hkTargetedFav := newCombo([]hotkey.Modifier{modBase}, keyUp)
	hkTargetedPause := newCombo([]hotkey.Modifier{modBase}, keyP)
	hkGlobalPause := newCombo([]hotkey.Modifier{modBase, modExtra}, keyP)

	// Register Targeted listeners
	registerAndListenTargeted(hkTargetedNext, "Targeted Next", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.SetNextWallpaper(mid, true)
			}
		})
	})

	registerAndListenTargeted(hkTargetedPrev, "Targeted Previous", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.SetPreviousWallpaper(mid, true)
			}
		})
	})

	registerAndListenTargeted(hkTargetedTrash, "Targeted Trash", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.DeleteCurrentImage(mid)
			}
		})
	})

	registerAndListenTargeted(hkTargetedFav, "Targeted Favorite", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.TriggerFavorite(mid)
			}
		})
	})

	registerAndListenTargeted(hkTargetedPause, "Targeted Pause", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.TogglePauseMonitorAction(mid)
			}
		})
	})

	// Register Global listeners
	registerAndListen(hkGlobalNext, "Global Next", func() {
		if wp != nil {
			wp.SetNextWallpaper(-1, true)
		}
	})

	registerAndListen(hkGlobalPrev, "Global Previous", func() {
		if wp != nil {
			wp.SetPreviousWallpaper(-1, true)
		}
	})

	registerAndListen(hkOpts, "Open Preferences", func() {
		if wp != nil {
			wp.TriggerOpenSettings()
		}
	})

	registerAndListen(hkGlobalSync, "Sync Monitors", func() {
		if wp != nil {
			wp.SyncMonitors(true)
		}
	})

	registerAndListen(hkGlobalPause, "Global Pause", func() {
		if wp != nil {
			wp.TogglePauseMonitorAction(-1)
		}
	})

	// --- 3. Anchor Shortcuts (Targeted: modBase + WASD/QE) ---
	hkAnchorQ := newCombo([]hotkey.Modifier{modBase}, keyQ)
	hkAnchorW := newCombo([]hotkey.Modifier{modBase}, keyW)
	hkAnchorE := newCombo([]hotkey.Modifier{modBase}, keyE)
	hkAnchorA := newCombo([]hotkey.Modifier{modBase}, keyA)
	hkAnchorS := newCombo([]hotkey.Modifier{modBase}, keyS)
	hkAnchorD := newCombo([]hotkey.Modifier{modBase}, keyD)

	registerAndListenTargeted(hkAnchorQ, "Anchor Auto", func() {
		handleAnchor(provider.AnchorAuto)
	})
	registerAndListenTargeted(hkAnchorW, "Anchor Up", func() {
		handleAnchor(provider.AnchorTopCenter)
	})
	registerAndListenTargeted(hkAnchorE, "Anchor Center", func() {
		handleAnchor(provider.AnchorMiddleCenter)
	})
	registerAndListenTargeted(hkAnchorA, "Anchor Left", func() {
		handleAnchor(provider.AnchorMiddleLeft)
	})
	registerAndListenTargeted(hkAnchorS, "Anchor Down", func() {
		handleAnchor(provider.AnchorBottomCenter)
	})
	registerAndListenTargeted(hkAnchorD, "Anchor Right", func() {
		handleAnchor(provider.AnchorMiddleRight)
	})

	// --- 4. Shuffle Shortcuts ---
	hkTargetedShuffle := newCombo([]hotkey.Modifier{modBase}, keyR)
	hkGlobalShuffle := newCombo([]hotkey.Modifier{modBase, modExtra}, keyR)

	registerAndListenTargeted(hkTargetedShuffle, "Targeted Shuffle", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.TriggerShuffle(mid)
			}
		})
	})

	registerAndListen(hkGlobalShuffle, "Global Shuffle", func() {
		if wp != nil {
			wp.TriggerShuffle(-1)
		}
	})

	// --- 5. Info Shortcuts ---
	hkTargetedInfo := newCombo([]hotkey.Modifier{modBase}, keyI)
	hkGlobalInfo := newCombo([]hotkey.Modifier{modBase, modExtra}, keyI)

	registerAndListenTargeted(hkTargetedInfo, "Targeted Info", func() {
		handleTargeted(func(mid int) {
			if wp != nil {
				wp.ViewCurrentImageOnWeb(mid)
			}
		})
	})

	registerAndListen(hkGlobalInfo, "Info (Primary Display)", func() {
		if wp != nil {
			wp.ViewCurrentImageOnWeb(0)
		}
	})
}

func registerAndListen(hk combo, name string, action func()) {
	reg, err := registerHotkey(hk.mods, hk.key, name, func() {
		if wp := wallpaper.GetInstance(); wp != nil && wp.GetShortcutsDisabled() {
			return
		}
		now := time.Now().Format("15:04:05.000")
		log.Debugf("[%s] Hotkey triggered: %s", now, name)
		action()
	})
	if err != nil {
		log.Printf("Failed to register hotkey %s: %v", name, err)
		return
	}
	log.Printf("[Hotkey] Registered: %s", name)
	registeredHotkeys = append(registeredHotkeys, reg)
}

func registerAndListenTargeted(hk combo, name string, action func()) {
	wp := wallpaper.GetInstance()
	if wp != nil && (wp.GetTargetedShortcutsDisabled() || wp.GetShortcutsDisabled()) {
		log.Printf("[Hotkey] Skipping targeted registration for %s (Disabled in Preferences)", name)
		return
	}

	reg, err := registerHotkey(hk.mods, hk.key, name, func() {
		wp := wallpaper.GetInstance()
		if wp != nil && (wp.GetShortcutsDisabled() || wp.GetTargetedShortcutsDisabled()) {
			return
		}
		now := time.Now().Format("15:04:05.000")
		log.Debugf("[%s] Targeted hotkey triggered: %s", now, name)
		action()
	})
	if err != nil {
		log.Printf("Failed to register hotkey %s: %v", name, err)
		return
	}
	log.Printf("[Hotkey] Registered Targeted: %s", name)
	registeredHotkeys = append(registeredHotkeys, reg)
}

func handleTargeted(action func(mid int)) {
	mid := GetMonitorIDFromKey()
	if mid == -1 {
		log.Debugf("[Hotkey] Suppression: Base modifier combination detected but no number key held. Skipping.")
		return
	}
	action(mid)
}

// handleAnchor sets a crop anchor on the targeted monitor.
func handleAnchor(anchor provider.CropAnchor) {
	handleTargeted(func(mid int) {
		if wp := wallpaper.GetInstance(); wp != nil {
			wp.SetTuningOptions(mid, provider.TuningOptions{Anchor: anchor})
		}
	})
}
