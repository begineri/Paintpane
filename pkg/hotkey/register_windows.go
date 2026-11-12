//go:build windows

package hotkey

import "golang.design/x/hotkey"

// loop is the single message loop that serves every global hotkey.
var loop msgLoop

type winRegistration struct{ id uintptr }

func (r winRegistration) Unregister() error { return loop.unregister(r.id) }

// registerHotkey registers mods+key system-wide; action runs on its own goroutine for each press.
func registerHotkey(mods []hotkey.Modifier, key hotkey.Key, name string, action func()) (registration, error) {
	var m uint32
	for _, mod := range mods {
		m |= uint32(mod)
	}
	id, err := loop.register(m, uint32(key), name, action)
	if err != nil {
		return nil, err
	}
	return winRegistration{id: id}, nil
}
