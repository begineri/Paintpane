//go:build windows

package autostart

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const (
	// runKeyPath is the current user's list of programs that start at logon.
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	// approvedKeyPath is where Windows' Startup apps page records which entries the person switched
	// off: a 12-byte value per entry whose first byte is odd (3) when it is off.
	approvedKeyPath = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

// registryStore keeps the registration in the current user's registry.
type registryStore struct {
	runKey, approvedKey, name string
}

// newRegistryStore returns the store for the real keys. Tests use newRegistryStoreAt with keys of
// their own.
func newRegistryStore() *registryStore {
	return newRegistryStoreAt(runKeyPath, approvedKeyPath, ValueName)
}

func newRegistryStoreAt(runKey, approvedKey, name string) *registryStore {
	return &registryStore{runKey: runKey, approvedKey: approvedKey, name: name}
}

func (r *registryStore) Command() (string, bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, r.runKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer k.Close()
	v, _, err := k.GetStringValue(r.name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (r *registryStore) SetCommand(command string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, r.runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(r.name, command)
}

func (r *registryStore) DeleteCommand() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, r.runKey, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(r.name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func (r *registryStore) DisabledByUser() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, r.approvedKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer k.Close()
	data, _, err := k.GetBinaryValue(r.name)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(data) > 0 && data[0]&1 == 1, nil // 2 and 6 are on, 3 is off
}

func (r *registryStore) ClearDisabledByUser() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, r.approvedKey, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(r.name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil // without a record Windows runs the entry
}

// realStore is the store of the running program's user.
func realStore() Store { return newRegistryStore() }
