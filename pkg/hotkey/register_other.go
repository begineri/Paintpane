//go:build !linux && !windows

package hotkey

import (
	"github.com/dixieflatline76/Spice/v2/util/log"
	"golang.design/x/hotkey"
)

// registerHotkey registers mods+key with golang.design/x/hotkey and runs action for each press.
func registerHotkey(mods []hotkey.Modifier, key hotkey.Key, name string, action func()) (registration, error) {
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		return nil, err
	}
	keydown := hk.Keydown()
	go func() {
		for range keydown {
			runAction(name, action)
		}
		log.Debugf("[Hotkey] Listener loop exited for %s", name)
	}()
	return hk, nil
}

func runAction(name string, action func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Hotkey] PANIC in action for %s: %v", name, r)
		}
	}()
	action()
}
