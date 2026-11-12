package autostart

import (
	"sync"

	"github.com/dixieflatline76/Spice/v2/util/log"
)

// MemoryStore keeps the registration in memory. Development builds use it (default_dev.go) and
// tests use it as a fake Run key.
type MemoryStore struct {
	mu         sync.Mutex
	command    string
	registered bool
	disabled   bool
	logWrites  bool
	err        error // returned by every call (tests)
}

// SetDisabledByUser sets whether Windows' Startup apps page has the entry switched off (tests).
func (m *MemoryStore) SetDisabledByUser(disabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disabled = disabled
}

// FailWith makes every following call return err; nil makes them work again (tests).
func (m *MemoryStore) FailWith(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MemoryStore) Command() (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.command, m.registered, m.err
}

func (m *MemoryStore) SetCommand(command string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.command, m.registered = command, true
	if m.logWrites {
		log.Printf("[Autostart] Development build: the real Run key is not touched. A release build would write %s = %s", ValueName, command)
	}
	return nil
}

func (m *MemoryStore) DeleteCommand() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.command, m.registered = "", false
	if m.logWrites {
		log.Printf("[Autostart] Development build: the real Run key is not touched. A release build would delete %s", ValueName)
	}
	return nil
}

func (m *MemoryStore) DisabledByUser() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.disabled, m.err
}

func (m *MemoryStore) ClearDisabledByUser() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.disabled = false
	return nil
}
