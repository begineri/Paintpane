package wallpaper

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// A start does not ask the sources again on a day when a whole fetch round already came back without
// a single new image (A2 §14.9 #3). Nightly maintenance and what the person asks for still fetch.

func withToday(t *testing.T, day string) {
	t.Helper()
	previous := today
	today = func() string { return day }
	t.Cleanup(func() { today = previous })
}

// dryTestPlugin is a plugin with its own preferences and one active query on provider "A".
func dryTestPlugin(t *testing.T) *Plugin {
	t.Helper()
	wp := newTestPlugin(t)
	wp.store.SetAsyncSave(false)
	wp.cfg = &Config{
		Preferences: NewMockPreferences(),
		Queries:     []ImageQuery{{ID: "q1", Provider: "A", Active: true, URL: "u"}},
	}
	wp.cfg.SetCacheSize(Cache100Images)
	mockOS := &MockOS{}
	mockOS.On("GetMonitors").Return([]Monitor{{ID: 0, Rect: image.Rect(0, 0, 100, 100)}}, nil)
	wp.os = mockOS
	wp.jobSubmitter = &recordingSubmitter{}
	wp.ctx = context.Background()
	return wp
}

func TestNoteFetchRound(t *testing.T) {
	wp := dryTestPlugin(t)
	withToday(t, "2026-11-08")

	wp.noteFetchRound(0, 0)
	assert.False(t, wp.fetchedDryToday(), "a round no source answered (offline) says nothing")

	wp.noteFetchRound(3, 0)
	assert.True(t, wp.fetchedDryToday(), "the sources answered with nothing new")

	withToday(t, "2026-11-09")
	assert.False(t, wp.fetchedDryToday(), "the mark is for that day only")

	wp.noteFetchRound(3, 0)
	assert.True(t, wp.fetchedDryToday())
	wp.noteFetchRound(0, 0)
	assert.True(t, wp.fetchedDryToday(), "an offline round does not clear it either")
	wp.noteFetchRound(3, 2)
	assert.False(t, wp.fetchedDryToday(), "a round with new images clears it")
}

func TestStartupFetchSkipsAfterADryRoundToday(t *testing.T) {
	wp := dryTestPlugin(t)
	syncer := &mockSyncerProvider{}
	syncer.On("ID").Return("TestProvider")
	wp.providers["TestProvider"] = syncer
	addReadyImages(t, wp, 50) // half full: not "full"

	withToday(t, "2026-11-08")
	wp.noteFetchRound(1, 0)
	wp.startupFetch()
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, syncer.syncCalled.Load(), "a start later the same day does not ask the sources")

	withToday(t, "2026-11-09")
	wp.startupFetch()
	assert.Eventually(t, func() bool { return syncer.syncCalled.Load() == 1 }, 2*time.Second, 10*time.Millisecond,
		"the next day's first start asks again")
}

// runFetchRound runs one whole round of every active query and waits until it is over.
func runFetchRound(t *testing.T, wp *Plugin, targetProvider ...string) {
	t.Helper()
	wp.FetchNewImages(false, targetProvider...)
	require.Eventually(t, func() bool { return !wp.fetchingInProgress.Value() }, 5*time.Second, 10*time.Millisecond)
}

func providerReturning(images []provider.Image, err error) *MockImageProvider {
	p := &MockImageProvider{}
	p.On("ID").Return("A")
	p.On("Type").Return(provider.TypeCommunity)
	p.On("FetchImages", mock.Anything, mock.Anything, mock.Anything).Return(images, err)
	return p
}

func TestFetchRoundSetsAndClearsTheDryMark(t *testing.T) {
	withToday(t, "2026-11-08")

	t.Run("only images the library already has", func(t *testing.T) {
		wp := dryTestPlugin(t)
		wp.store.Add(provider.Image{ID: "A_1", DerivativePaths: map[string]string{"100x100": "d.jpg"}})
		wp.providers["A"] = providerReturning([]provider.Image{{ID: "1"}}, nil)
		runFetchRound(t, wp)
		assert.True(t, wp.fetchedDryToday())
	})

	t.Run("an empty page", func(t *testing.T) {
		wp := dryTestPlugin(t)
		wp.providers["A"] = providerReturning(nil, nil)
		runFetchRound(t, wp)
		assert.True(t, wp.fetchedDryToday())
	})

	t.Run("a new image", func(t *testing.T) {
		wp := dryTestPlugin(t)
		wp.cfg.SetString(dryFetchDayPrefKey, "2026-11-08")
		wp.providers["A"] = providerReturning([]provider.Image{{ID: "new"}}, nil)
		runFetchRound(t, wp)
		assert.False(t, wp.fetchedDryToday())
	})

	t.Run("the source failed", func(t *testing.T) {
		wp := dryTestPlugin(t)
		wp.providers["A"] = providerReturning(nil, errors.New("offline"))
		runFetchRound(t, wp)
		assert.False(t, wp.fetchedDryToday(), "a failure is not \"nothing new\"")
	})

	t.Run("a fetch for one source only", func(t *testing.T) {
		wp := dryTestPlugin(t)
		wp.providers["A"] = providerReturning(nil, nil)
		runFetchRound(t, wp, "A")
		assert.False(t, wp.fetchedDryToday(), "only a round of every active query counts")
	})
}
