package pexels

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestMain replaces the OS credential store with an in-memory one: the provider reads the
// Pexels API key from it, and tests must never see the key of whoever runs them.
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}
