package wallpaper

import (
	"sync"
	"time"

	"github.com/dixieflatline76/Spice/v2/util/log"
)

// Some image servers stop answering for long stretches (SMK's iip.smk.dk timed out on every
// request for hours while its API answered, A2 §12.9 #4). Retrying every image then keeps the
// download workers busy for about a minute per image and burns CPU. downloadBackoff counts, per
// source, the images whose download failed because the server could not be reached, and pauses
// that source's downloads after a few in a row. Other sources are not affected.
const (
	// backoffFailures is how many images in a row must fail before a source is paused.
	backoffFailures = 3
	// backoffFirstPause is the first pause; each pause after it is twice as long, up to
	// backoffMaxPause, until an image downloads again.
	backoffFirstPause = 2 * time.Minute
	backoffMaxPause   = 30 * time.Minute
)

type downloadBackoff struct {
	mu     sync.Mutex
	now    func() time.Time
	states map[string]*backoffState
}

type backoffState struct {
	failures    int // images in a row whose server could not be reached
	pauses      int // pauses since the last image that downloaded
	pausedUntil time.Time
}

func newDownloadBackoff() *downloadBackoff {
	return &downloadBackoff{now: time.Now, states: map[string]*backoffState{}}
}

// downloadBackoff returns the plugin's backoff, made on first use.
func (wp *Plugin) downloadBackoff() *downloadBackoff {
	wp.backoffOnce.Do(func() { wp.backoff = newDownloadBackoff() })
	return wp.backoff
}

func (b *downloadBackoff) state(source string) *backoffState {
	s, ok := b.states[source]
	if !ok {
		s = &backoffState{}
		b.states[source] = s
	}
	return s
}

// failed records an image of source whose server could not be reached and returns the pause it
// started, or 0. Failures while the source is paused (downloads that were already running) do not
// count. After a pause, one more failure starts the next, longer pause.
func (b *downloadBackoff) failed(source string) time.Duration {
	if b == nil || source == "" {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.state(source)
	now := b.now()
	if now.Before(s.pausedUntil) {
		return 0
	}
	s.failures++
	if s.failures < backoffFailures {
		return 0
	}
	pause := backoffFirstPause << s.pauses
	if pause > backoffMaxPause || pause <= 0 {
		pause = backoffMaxPause
	} else {
		s.pauses++
	}
	s.pausedUntil = now.Add(pause)
	s.failures = backoffFailures - 1 // the first image after the pause probes the server
	log.Printf("Downloads from %s paused for %v: its image server could not be reached for %d images in a row", source, pause, backoffFailures)
	return pause
}

// pauseForOutage pauses source's downloads for pause after an image failed during an outage the
// source reported (provider.OutagePauseProvider). It does not shorten a longer pause and does not
// count towards the doubling pauses; after it, one more failure starts the usual short pause.
func (b *downloadBackoff) pauseForOutage(source string, pause time.Duration) {
	if b == nil || source == "" || pause <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.state(source)
	now := b.now()
	until := now.Add(pause)
	if !until.After(s.pausedUntil) {
		return
	}
	s.pausedUntil = until
	s.failures = backoffFailures - 1
	log.Printf("Downloads from %s paused for %v: its image server does not answer at this time of day", source, pause.Round(time.Minute))
}

// succeeded records an image of source that downloaded, which ends any backoff.
func (b *downloadBackoff) succeeded(source string) {
	if b == nil || source == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.states[source]; ok && (s.failures > 0 || s.pauses > 0) {
		if s.pauses > 0 {
			log.Printf("Downloads from %s work again", source)
		}
		delete(b.states, source)
	}
}

// pausedFor returns how much longer source's downloads are paused, or 0.
func (b *downloadBackoff) pausedFor(source string) time.Duration {
	if b == nil || source == "" {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.states[source]
	if !ok {
		return 0
	}
	if d := s.pausedUntil.Sub(b.now()); d > 0 {
		return d
	}
	return 0
}
