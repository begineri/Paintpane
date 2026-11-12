//go:build windows

package autostart

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/registry"
)

// The registry store works against keys of the test's own under HKCU\Software, never the real Run
// key: a test must not touch what starts at logon on the machine it runs on.
func testStore(t *testing.T) *registryStore {
	t.Helper()
	root := fmt.Sprintf(`Software\PaintpaneAutostartTest\%d`, os.Getpid())
	runKey, approvedKey := root+`\Run`, root+`\Approved`
	require.False(t, strings.HasSuffix(strings.ToLower(runKey), `currentversion\run`), "never the real Run key")
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, runKey)
		_ = registry.DeleteKey(registry.CURRENT_USER, approvedKey)
		_ = registry.DeleteKey(registry.CURRENT_USER, root)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\PaintpaneAutostartTest`)
	})
	return newRegistryStoreAt(runKey, approvedKey, ValueName)
}

func TestRegistryStoreRoundTrip(t *testing.T) {
	s := testStore(t)

	_, registered, err := s.Command()
	require.NoError(t, err)
	assert.False(t, registered, "no key yet: not registered, not an error")
	require.NoError(t, s.DeleteCommand(), "deleting from a key that is not there is fine")
	disabled, err := s.DisabledByUser()
	require.NoError(t, err)
	assert.False(t, disabled)

	on, err := Enabled(s)
	require.NoError(t, err)
	assert.False(t, on)

	require.NoError(t, SetEnabled(s, exe, true))
	cmd, registered, err := s.Command()
	require.NoError(t, err)
	assert.True(t, registered)
	assert.Equal(t, Command(exe), cmd)
	on, _ = Enabled(s)
	assert.True(t, on)

	require.NoError(t, SetEnabled(s, exe, false))
	_, registered, _ = s.Command()
	assert.False(t, registered)
}

func TestRegistryStoreReadsWindowsStartupAppsRecord(t *testing.T) {
	s := testStore(t)
	require.NoError(t, SetEnabled(s, exe, true))

	// Windows writes 12 bytes per entry; the first byte is 2 or 6 for on and 3 for off.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, s.approvedKey, registry.SET_VALUE)
	require.NoError(t, err)
	defer k.Close()
	for first, wantDisabled := range map[byte]bool{0x02: false, 0x06: false, 0x03: true} {
		data := make([]byte, 12)
		data[0] = first
		require.NoError(t, k.SetBinaryValue(ValueName, data))
		disabled, err := s.DisabledByUser()
		require.NoError(t, err)
		assert.Equal(t, wantDisabled, disabled, "first byte %#x", first)
	}

	data := make([]byte, 12)
	data[0] = 0x03
	require.NoError(t, k.SetBinaryValue(ValueName, data))
	on, _ := Enabled(s)
	assert.False(t, on, "switched off in Windows reads as off")

	require.NoError(t, SetEnabled(s, exe, true))
	on, _ = Enabled(s)
	assert.True(t, on, "turning it on clears the record")
}
