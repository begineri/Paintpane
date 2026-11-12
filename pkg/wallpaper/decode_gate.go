package wallpaper

import (
	"context"
	"runtime/debug"
	"sync"
)

// Decoding and fitting a large original takes a lot of memory: 3 MB per megapixel for a common
// JPEG, about 11 for a progressive one even with progjpeg (decode_cost.go). Four download workers doing that at once took
// the app to 1.7 GB (A2 §12.9 #8, phase 5 §13.5). The originals decoded at the same time are
// therefore limited by their estimated memory together, decodeBudgetMB: ordinary ones run several
// at a time, a large or progressive one on its own. Nothing is skipped or scaled down; large ones
// only wait their turn.
const (
	// decodeBudgetMB leaves room for the rest of the app (about 500 MB) under 1 GB.
	decodeBudgetMB = 400
	// After an original estimated at bigDecodeMB or more, the memory it used is handed back to
	// the system at once instead of when the runtime gets round to it.
	bigDecodeMB = 120
)

var masterDecodes = newDecodeGate(decodeBudgetMB)

// acquireMasterDecode waits until the master at path may be decoded, weighted by the memory it
// will take, and returns the function that ends the decode. It fails only when ctx ends first.
func acquireMasterDecode(ctx context.Context, path string) (release func(), err error) {
	return acquireMasterDecodePlus(ctx, path, 0)
}

// acquireMasterDecodePlus is acquireMasterDecode for work that needs extraMB on top of decoding
// and fitting the original, such as shrinking it (master_cap.go).
func acquireMasterDecodePlus(ctx context.Context, path string, extraMB int64) (release func(), err error) {
	cost := masterDecodeCostMB(path) + extraMB
	gate := masterDecodes
	n, err := gate.acquire(ctx, cost)
	if err != nil {
		return nil, err
	}
	return func() {
		gate.release(n)
		if cost >= bigDecodeMB {
			debug.FreeOSMemory()
		}
	}, nil
}

// decodeGate is a first-come, first-served weighted semaphore: a large request waiting for room is
// not overtaken by small ones.
type decodeGate struct {
	mu      sync.Mutex
	size    int64
	used    int64
	waiters []*gateWaiter
}

type gateWaiter struct {
	n     int64
	ready chan struct{}
}

func newDecodeGate(size int64) *decodeGate {
	return &decodeGate{size: size}
}

// acquire takes n units (at most the gate's size) and returns how many it took.
func (g *decodeGate) acquire(ctx context.Context, n int64) (int64, error) {
	if n > g.size {
		n = g.size
	}
	if n < 1 {
		n = 1
	}
	g.mu.Lock()
	if len(g.waiters) == 0 && g.used+n <= g.size {
		g.used += n
		g.mu.Unlock()
		return n, nil
	}
	w := &gateWaiter{n: n, ready: make(chan struct{})}
	g.waiters = append(g.waiters, w)
	g.mu.Unlock()

	select {
	case <-w.ready:
		return n, nil
	case <-ctx.Done():
		g.mu.Lock()
		defer g.mu.Unlock()
		select {
		case <-w.ready:
			// Granted while giving up: hand it back.
			g.used -= n
		default:
			for i, x := range g.waiters {
				if x == w {
					g.waiters = append(g.waiters[:i], g.waiters[i+1:]...)
					break
				}
			}
		}
		g.grant()
		return 0, ctx.Err()
	}
}

// release returns n units taken by acquire.
func (g *decodeGate) release(n int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.used -= n
	g.grant()
}

// grant lets waiting requests in, in order, while they fit. Called with mu held.
func (g *decodeGate) grant() {
	for len(g.waiters) > 0 {
		w := g.waiters[0]
		if g.used+w.n > g.size {
			return
		}
		g.used += w.n
		close(w.ready)
		g.waiters = g.waiters[1:]
	}
}
