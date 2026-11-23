package autostart

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandExe(t *testing.T) {
	assert.Equal(t, `C:\Users\me\Downloads\paintpane (1).exe`, CommandExe(`"C:\Users\me\Downloads\paintpane (1).exe" --autostart`))
	assert.Equal(t, `C:\Apps\paintpane.exe`, CommandExe(`C:\Apps\paintpane.exe --autostart`))
	assert.Equal(t, `C:\Apps\paintpane.exe`, CommandExe(`  "C:\Apps\paintpane.exe"`))
	assert.Equal(t, `C:\Apps\paintpane.exe`, CommandExe(`"C:\Apps\paintpane.exe`))
	assert.Equal(t, "", CommandExe(""))
}

func TestOtherCopyAtLogon(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "paintpane (1).exe")
	require.NoError(t, os.WriteFile(web, []byte("exe"), 0o600))
	self := filepath.Join(dir, "WindowsApps", "Paintpane.exe")

	s := &MemoryStore{}
	_, ok := OtherCopyAtLogon(s, self)
	assert.False(t, ok, "nothing registered")

	require.NoError(t, s.SetCommand(Command(web)))
	exe, ok := OtherCopyAtLogon(s, self)
	assert.True(t, ok)
	assert.Equal(t, web, exe)

	s.SetDisabledByUser(true)
	_, ok = OtherCopyAtLogon(s, self)
	assert.False(t, ok, "switched off in Startup apps: it will not start")
	s.SetDisabledByUser(false)

	_, ok = OtherCopyAtLogon(s, web)
	assert.False(t, ok, "the entry starts this very program")

	require.NoError(t, s.SetCommand(Command(filepath.Join(dir, "deleted.exe"))))
	_, ok = OtherCopyAtLogon(s, self)
	assert.False(t, ok, "the exe it names is gone")

	_, ok = OtherCopyAtLogon(nil, self)
	assert.False(t, ok, "no store")
}

func TestWaitFor(t *testing.T) {
	var slept time.Duration
	sleep := func(d time.Duration) { slept += d }

	calls := 0
	assert.True(t, WaitFor(func() bool { calls++; return calls == 3 }, 10*time.Second, 2*time.Second, sleep))
	assert.Equal(t, 4*time.Second, slept, "found on the third look")

	slept = 0
	assert.False(t, WaitFor(func() bool { return false }, 10*time.Second, 2*time.Second, sleep))
	assert.Equal(t, 10*time.Second, slept, "gives up after the timeout")
}
