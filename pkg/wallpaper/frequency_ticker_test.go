package wallpaper

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// Changing the frequency again and again must not race with the loops of the earlier tickers
// (go test -race), and every replaced loop must end.
func TestChangeWallpaperFrequency_TickerLoopsDoNotRace(t *testing.T) {
	ResetConfig()
	mockPM := &MockPluginManager{}
	mockPM.On("RebuildTrayMenu").Return().Maybe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp := &Plugin{cfg: GetConfig(NewMockPreferences()), manager: mockPM, ctx: ctx}

	before := runtime.NumGoroutine()
	freqs := []Frequency{FrequencyHourly, FrequencyDaily}
	for i := 0; i < 50; i++ {
		wp.ChangeWallpaperFrequency(freqs[i%2], true)
	}
	// Only the last loop may still run.
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before+1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before+1 {
		t.Fatalf("%d goroutines left after 50 frequency changes, want at most %d", n, before+1)
	}

	cancel()
	deadline = time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Fatalf("%d goroutines left after the plugin context ended, want %d", n, before)
	}
}
