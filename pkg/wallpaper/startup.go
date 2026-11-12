package wallpaper

import (
	"math/rand"
	"os"
	"time"

	"github.com/dixieflatline76/Spice/v2/util/log"
)

// Start-up behaviour of a light background app (docs/lightweight-eval.md §4 C1 and D2):
//
//   - A start that Windows made at logon waits 60–120 seconds before it fetches or processes
//     anything, so it does not compete with everything else starting up. A start the person made
//     does not wait.
//   - A start with a full library does not ask the sources for anything, and neither does a start
//     on a day when a fetch round already came back without a single new image. Nightly
//     maintenance, the starvation fetch and whatever the person asks for keep the library topped up.
const (
	// AutostartFlag is what the "start with Windows" entry passes to the program.
	AutostartFlag = "--autostart"

	autostartDelayMin    = 60 * time.Second
	autostartDelaySpread = 60 * time.Second // the delay is 60 s plus up to this much
	logonWindow          = 3 * time.Minute  // a start this soon after logon is an autostart
)

// detectAutostart says whether this run was started by Windows at logon: the command line has
// AutostartFlag, or the person logged on only moments ago. Tests replace it.
var detectAutostart = func() bool {
	return hasAutostartFlag(os.Args[1:]) || startedSoonAfterLogon(logonWindow)
}

func hasAutostartFlag(args []string) bool {
	for _, a := range args {
		if a == AutostartFlag {
			return true
		}
	}
	return false
}

// autostartDelay returns how long background work waits after an autostart: 60–120 s.
func autostartDelay() time.Duration {
	return autostartDelayMin + time.Duration(rand.Int63n(int64(autostartDelaySpread)+1)) //nolint:gosec // jitter, not security
}

// startupDelay returns how long this start's background work waits. A run the person started has
// no wait, and neither has an empty library: its first images are what the person is waiting for.
func (wp *Plugin) startupDelay() time.Duration {
	if wp.store.Count() == 0 || !detectAutostart() {
		return 0
	}
	return autostartDelay()
}

// libraryFullPercent is how full the library must be to count as full at start. A cycle only
// downloads what the cache still has room for and skips images it already has, so after a few
// starts the library settles a few images short of the limit (140–148 of 150, A2 §14.9 #3) and
// would never count as full if the limit itself were the bar.
const libraryFullPercent = 90

// libraryFull reports whether the store holds at least libraryFullPercent of a whole cache.
func (wp *Plugin) libraryFull() bool {
	return wp.store.Count()*100 >= wp.cacheLimit()*libraryFullPercent
}

// dryFetchDayPrefKey keeps the local date (YYYY-MM-DD) of the last fetch round that the sources
// answered without a single new image; empty after a round that found some.
const dryFetchDayPrefKey = pluginPrefix + "dry_fetch_day"

// today returns the local date as dryFetchDayPrefKey stores it. Tests replace it.
var today = func() string { return time.Now().Format(time.DateOnly) }

// noteFetchRound records what a finished round of every active query found: a round with new
// images clears the mark, a round the sources answered with nothing new sets it for today.
// A round that no source answered (offline, every source paused) says nothing about the sources
// and leaves the mark alone.
func (wp *Plugin) noteFetchRound(answered, newImages int) {
	if wp.cfg == nil {
		return
	}
	switch {
	case newImages > 0:
		if wp.cfg.String(dryFetchDayPrefKey) != "" {
			wp.cfg.SetString(dryFetchDayPrefKey, "")
		}
	case answered > 0:
		if day := today(); wp.cfg.String(dryFetchDayPrefKey) != day {
			log.Printf("[Fetch] The sources had no new images. Starts later today will not ask them again.")
			wp.cfg.SetString(dryFetchDayPrefKey, day)
		}
	}
}

// fetchedDryToday reports whether a fetch round today came back without new images.
func (wp *Plugin) fetchedDryToday() bool {
	return wp.cfg != nil && wp.cfg.String(dryFetchDayPrefKey) == today()
}

// afterStartupDelay runs fn now when delay is zero, otherwise after delay (and not at all if the
// plugin is deactivated meanwhile).
func (wp *Plugin) afterStartupDelay(delay time.Duration, fn func()) {
	if delay <= 0 {
		fn()
		return
	}
	ctx := wp.ctx
	go func() {
		select {
		case <-time.After(delay):
			fn()
		case <-ctx.Done():
		}
	}()
}

// startupFetch is what a start does about new images when the person has not asked for a change at
// start: top up from the sources, unless the library is full or the sources had nothing new today.
func (wp *Plugin) startupFetch() {
	if wp.libraryFull() {
		log.Printf("[Startup] The library is full (%d of %d images). Not asking the sources for more.", wp.store.Count(), wp.cacheLimit())
		return
	}
	if wp.fetchedDryToday() {
		log.Printf("[Startup] The sources had no new images earlier today (library: %d of %d). Not asking them again until tomorrow.", wp.store.Count(), wp.cacheLimit())
		return
	}
	wp.SyncProviders()
	wp.FetchNewImages(false)
}
