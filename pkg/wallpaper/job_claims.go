package wallpaper

import (
	"context"
	"errors"
	"sync"
)

// ErrImageInProgress is returned by ProcessImageJob for a download job whose image another job
// is already downloading or processing. The image appears in two queries, or was fetched again
// before its first job finished; the first job adds it to the store, so the second one is dropped
// without cleaning up anything (A2 phase 5 #4).
var ErrImageInProgress = errors.New("image is already being processed")

// jobClaims records which image IDs a job is working on, so one image is downloaded and
// processed by one job at a time. Before, a second job saw the master the first one was still
// writing, failed to decode it ("short Huffman data") and deleted the first job's files.
type jobClaims struct {
	mu   sync.Mutex
	busy map[string]chan struct{}
}

// tryClaim claims id for the caller and returns the function that gives it back, or false when
// another job has it.
func (c *jobClaims) tryClaim(id string) (release func(), ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, taken := c.busy[id]; taken {
		return nil, false
	}
	return c.claimLocked(id), true
}

// claim waits until id is free, claims it and returns the function that gives it back. It fails
// only when ctx ends first.
func (c *jobClaims) claim(ctx context.Context, id string) (release func(), err error) {
	for {
		c.mu.Lock()
		done, taken := c.busy[id]
		if !taken {
			release := c.claimLocked(id)
			c.mu.Unlock()
			return release, nil
		}
		c.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *jobClaims) claimLocked(id string) func() {
	if c.busy == nil {
		c.busy = make(map[string]chan struct{})
	}
	done := make(chan struct{})
	c.busy[id] = done
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			delete(c.busy, id)
			c.mu.Unlock()
			close(done)
		})
	}
}
