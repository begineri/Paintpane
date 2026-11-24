package wallpaper

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/stretchr/testify/assert"
)

// syncGate holds the first Sync until it is released. It stands in for a display mode switch
// that has read its settings and lost the CPU just before changing the store.
type syncGate struct {
	StoreInterface
	entered, release chan struct{}
	second           chan struct{} // closed when a second Sync reaches the store

	mu    sync.Mutex
	calls int
	full  []bool // flagDisplayFull of each Sync, in the order they reached the store
}

func (g *syncGate) Sync(limit int, flags, active, keep map[string]bool) {
	g.mu.Lock()
	g.calls++
	first := g.calls == 1
	if g.calls == 2 {
		close(g.second)
	}
	g.mu.Unlock()
	if first {
		close(g.entered)
		<-g.release
	}
	g.mu.Lock()
	g.full = append(g.full, flags[flagDisplayFull])
	g.mu.Unlock()
	g.StoreInterface.Sync(limit, flags, active, keep)
}

// Switching Museum frame -> Full artwork quickly starts two syncs. The one for the earlier choice
// must not reach the store after the one for the last choice: the store would then keep Museum
// frame, every image rebuilt for Full artwork would be dropped as stale, and the library would stay
// without wallpapers (TestE2E_RapidSwitchingEndsOnLastChoice failed this way in CI for v1.3.0).
// The order is forced with channels and goroutine states, never with sleeps, so the result does not
// depend on the machine's speed: before the fix it failed 200 runs out of 200, after it passes 200.
func TestSyncStoreWithConfig_LastChoiceReachesStoreLast(t *testing.T) {
	env := newDisplayTestEnv(t)
	gate := &syncGate{StoreInterface: env.store, entered: make(chan struct{}), release: make(chan struct{}), second: make(chan struct{})}
	env.wp.store = gate

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)
	first := make(chan struct{})
	go func() { env.wp.syncStoreWithConfig(); close(first) }()
	<-gate.entered // the Museum frame sync is inside the store, holding on

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	second := make(chan struct{})
	go func() { env.wp.syncStoreWithConfig(); close(second) }()
	// Let the second sync get as far as it can: into the store (it overtook the first) or to the
	// lock that makes it wait for the first. Either is a state, so this does not depend on timing.
	deadline := time.Now().Add(time.Minute) // only bounds a hang
	for !closed(gate.second) && !waitingForStoreSyncLock() {
		if time.Now().After(deadline) {
			t.Fatal("the second sync neither reached the store nor waited for the first")
		}
		runtime.Gosched()
	}

	close(gate.release)
	<-first
	<-second

	assert.Equal(t, []bool{false, true}, gate.full, "the sync for the last choice reaches the store last")
	rebuilt := provider.Image{ID: "x", ProcessingFlags: env.wp.targetProcessingFlags()}
	assert.False(t, env.store.isStaleDisplayMode(rebuilt), "images rebuilt for the last choice are kept")
}

func closed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// waitingForStoreSyncLock reports whether a goroutine is blocked on the lock in syncStoreWithConfig.
func waitingForStoreSyncLock() bool {
	buf := make([]byte, 1<<20)
	for _, g := range strings.Split(string(buf[:runtime.Stack(buf, true)]), "\n\n") {
		if strings.Contains(g, "Mutex).Lock") && strings.Contains(g, "(*Plugin).syncStoreWithConfig") &&
			!strings.Contains(g, "(*syncGate).Sync") {
			return true
		}
	}
	return false
}
