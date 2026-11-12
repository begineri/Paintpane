package wallpaper

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// lockScreenOS is a MockOS that also supports the lock screen.
type lockScreenOS struct {
	MockOS
	paths chan string
	err   error
}

func (o *lockScreenOS) SetLockScreen(path string) error {
	o.paths <- path
	return o.err
}

func newNudeTestConfig(t *testing.T) *Config {
	t.Helper()
	ResetConfig()
	t.Cleanup(ResetConfig)
	return GetConfig(NewMockPreferences())
}

func sfwImage(id string) provider.Image {
	return provider.Image{ID: id, Provider: wallhavenProviderID, Purity: puritySFW, FilePath: id + ".jpg"}
}

func nsfwImage(id string) provider.Image {
	return provider.Image{ID: id, Provider: wallhavenProviderID, Purity: "nsfw", FilePath: id + ".jpg"}
}

// newNudeTestController builds a controller over the images, listed in rotation order.
func newNudeTestController(t *testing.T, monitorID int, cfg *Config, osMock OS, images ...provider.Image) *MonitorController {
	t.Helper()
	store := &MockImageStore{}
	ids := make([]string, 0, len(images))
	for _, img := range images {
		ids = append(ids, img.ID)
		store.On("GetByID", img.ID).Return(img, true).Maybe()
	}
	store.On("GetIDsForResolution", mock.Anything).Return(ids).Maybe()
	store.On("MarkSeen", mock.Anything).Return().Maybe()

	mc := NewMonitorController(monitorID, Monitor{ID: monitorID}, store, nil, osMock, cfg, nil)
	mc.State.ShuffleIDs = ids
	return mc
}

func newRecordingOS() *MockOS {
	osMock := &MockOS{}
	osMock.On("Stat", mock.Anything).Return(nil, nil).Maybe()
	osMock.On("SetWallpaper", mock.Anything, mock.Anything).Return(nil).Maybe()
	return osMock
}

func newLockScreenOS() *lockScreenOS {
	osMock := &lockScreenOS{paths: make(chan string, 10)}
	osMock.On("Stat", mock.Anything).Return(nil, nil).Maybe()
	osMock.On("SetWallpaper", mock.Anything, mock.Anything).Return(nil).Maybe()
	return osMock
}

func nextLockScreenPath(t *testing.T, osMock *lockScreenOS) string {
	t.Helper()
	select {
	case path := <-osMock.paths:
		return path
	case <-time.After(5 * time.Second):
		require.Fail(t, "lock screen was not updated")
		return ""
	}
}

func assertNoLockScreenUpdate(t *testing.T, osMock *lockScreenOS) {
	t.Helper()
	select {
	case path := <-osMock.paths:
		assert.Fail(t, "unexpected lock screen update", path)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestIsImageHidden(t *testing.T) {
	col := curation.GetManager().GetEntry("Getty", "getty_nudes")
	require.NotNil(t, col)
	curatedNude := provider.Image{ID: "Getty_" + col.IDs[0], Provider: "Getty"}

	tests := []struct {
		name string
		img  provider.Image
		want bool
	}{
		{"curated nude", curatedNude, true},
		{"wallhaven sfw", provider.Image{ID: "w1", Provider: wallhavenProviderID, Purity: "sfw"}, false},
		{"wallhaven sketchy", provider.Image{ID: "w2", Provider: wallhavenProviderID, Purity: "sketchy"}, true},
		{"wallhaven nsfw", provider.Image{ID: "w3", Provider: wallhavenProviderID, Purity: "nsfw"}, true},
		{"wallhaven unrated", provider.Image{ID: "w4", Provider: wallhavenProviderID}, true},
		{"other provider", provider.Image{ID: "p1", Provider: "Pexels"}, false},
	}

	cfg := newNudeTestConfig(t)
	require.False(t, cfg.GetShowNudes(), "nude images must be hidden by default")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cfg.IsImageHidden(tt.img))
		})
	}

	cfg.SetShowNudes(true)
	for _, tt := range tests {
		assert.False(t, cfg.IsImageHidden(tt.img), tt.name)
	}
}

func TestNudeMarks(t *testing.T) {
	cfg := newNudeTestConfig(t)
	img := provider.Image{ID: "p1", Provider: "Pexels"}

	cfg.SetNudeMarked(img.ID, true)
	assert.True(t, cfg.IsImageHidden(img))

	cfg.SetShowNudes(true)
	assert.False(t, cfg.IsImageHidden(img))
	cfg.SetShowNudes(false)

	cfg.SetNudeMarked(img.ID, false)
	assert.False(t, cfg.IsImageHidden(img))
}

func TestNudeSettings_SurviveRestart(t *testing.T) {
	ResetConfig()
	t.Cleanup(ResetConfig)
	prefs := NewMockPreferences()
	img := provider.Image{ID: "p1", Provider: "Pexels"}

	cfg := GetConfig(prefs)
	cfg.SetShowNudes(true)
	cfg.SetNudeMarked(img.ID, true)

	ResetConfig()
	cfg = GetConfig(prefs)
	assert.True(t, cfg.GetShowNudes())
	assert.True(t, cfg.IsNudeMarked(img.ID))
}

func TestImagePurityPersists(t *testing.T) {
	data, err := json.Marshal(provider.Image{ID: "w1", Purity: "sketchy"})
	require.NoError(t, err)

	var loaded provider.Image
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "sketchy", loaded.Purity)

	fresh := provider.Image{ID: "w1"}
	fresh.MergeExistingMetadata(loaded)
	assert.Equal(t, "sketchy", fresh.Purity)
}

func TestFetch_BackfillsPurityOnCachedImage(t *testing.T) {
	t.Setenv("LocalAppData", t.TempDir()) // saveQueryPages writes under the user cache dir

	store := &MockImageStore{}
	store.On("GetByID", "Wallhaven_img").Return(provider.Image{
		ID:              "Wallhaven_img",
		Provider:        wallhavenProviderID,
		DerivativePaths: map[string]string{"1920x1080": "img.jpg"},
	}, true)
	store.On("SetPurity", "Wallhaven_img", "sketchy").Return(true).Once()

	p := &MockImageProvider{}
	p.On("ID").Return(wallhavenProviderID).Maybe()
	p.On("Type").Return(provider.TypeCommunity).Maybe()
	p.On("FetchImages", mock.Anything, "search", 1).Return([]provider.Image{
		{ID: "img", Provider: wallhavenProviderID, Purity: "sketchy"},
	}, nil)

	osMock := &MockOS{}
	osMock.On("GetMonitors").Return([]Monitor{}, nil).Maybe()

	wp := &Plugin{
		cfg:              newNudeTestConfig(t),
		store:            store,
		os:               osMock,
		ctx:              context.Background(),
		queryPages:       make(map[string]*util.SafeCounter),
		queryContexts:    make(map[string]context.Context),
		queryCancelFuncs: make(map[string]context.CancelFunc),
		Monitors:         make(map[int]*MonitorController),
		jobSubmitter:     &MockPipeline{},
	}

	var mu sync.Mutex
	wp.fetchFromProvider(context.Background(), ImageQuery{ID: "q1", Provider: wallhavenProviderID, URL: "search"}, p, false, 100, &mu, map[string]bool{}, util.NewSafeInt())

	store.AssertExpectations(t)
}

func TestNext_SkipsHiddenImages(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, nsfwImage("hidden"), sfwImage("ok"))

	mc.next(true)

	assert.Equal(t, "ok", mc.State.CurrentID)
	assert.Equal(t, []string{"ok"}, mc.State.History)
	osMock.AssertCalled(t, "SetWallpaper", "ok.jpg", 1)
	osMock.AssertNotCalled(t, "SetWallpaper", "hidden.jpg", mock.Anything)
}

func TestNext_ShowsNudesWhenEnabled(t *testing.T) {
	cfg := newNudeTestConfig(t)
	cfg.SetShowNudes(true)
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, cfg, osMock, nsfwImage("nude"), sfwImage("ok"))

	mc.next(true)

	assert.Equal(t, "nude", mc.State.CurrentID)
}

func TestNext_AllHiddenLeavesWallpaperUntouched(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, nsfwImage("a"), nsfwImage("b"))

	mc.next(true)

	assert.True(t, mc.State.WaitingForImages)
	assert.Empty(t, mc.State.CurrentID)
	osMock.AssertNotCalled(t, "SetWallpaper", mock.Anything, mock.Anything)
}

func TestPrev_SkipsHiddenHistory(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, sfwImage("first"), nsfwImage("hidden"), sfwImage("last"))
	mc.State.History = []string{"first", "hidden", "last"}
	mc.State.CurrentID = "last"
	mc.State.RandomPos = 0

	mc.prev()

	assert.Equal(t, "first", mc.State.CurrentID)
	assert.Equal(t, []string{"first"}, mc.State.History)
	assert.Equal(t, 1, mc.State.RandomPos, "should step back once per popped entry and wrap")
	osMock.AssertCalled(t, "SetWallpaper", "first.jpg", 1)
}

func TestPrev_StaysWhenAllHistoryHidden(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, nsfwImage("hidden"), sfwImage("last"))
	mc.State.History = []string{"hidden", "last"}
	mc.State.CurrentID = "last"

	mc.prev()

	assert.Equal(t, "last", mc.State.CurrentID)
	assert.Equal(t, []string{"hidden", "last"}, mc.State.History)
	osMock.AssertNotCalled(t, "SetWallpaper", mock.Anything, mock.Anything)
}

func TestEnforceNudeFilter_ReplacesHiddenImageEvenWhenPaused(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, nsfwImage("nude"), sfwImage("ok"))
	mc.State.CurrentImage = nsfwImage("nude")
	mc.State.Paused = true

	mc.handleCommand(CmdEnforceNudeFilter)

	assert.Equal(t, "ok", mc.State.CurrentImage.ID)
}

func TestEnforceNudeFilter_LeavesVisibleImage(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, sfwImage("a"), sfwImage("b"))
	mc.State.CurrentImage = sfwImage("a")

	mc.handleCommand(CmdEnforceNudeFilter)

	assert.Equal(t, "a", mc.State.CurrentImage.ID)
	osMock.AssertNotCalled(t, "SetWallpaper", mock.Anything, mock.Anything)
}

func TestEnforceNudeFilter_KeepsImageWhenNothingCanReplace(t *testing.T) {
	osMock := newRecordingOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, nsfwImage("nude"))
	mc.State.CurrentImage = nsfwImage("nude")

	mc.handleCommand(CmdEnforceNudeFilter)

	assert.Equal(t, "nude", mc.State.CurrentImage.ID)
}

func TestEnforceNudeFilter_NeverRemovesImages(t *testing.T) {
	cfg := newNudeTestConfig(t)
	cfg.SetShowNudes(true)
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, cfg, osMock, nsfwImage("nude"), sfwImage("ok"), sfwImage("other"))
	mc.State.CurrentImage = nsfwImage("nude")
	mc.lockImageID = "nude"

	cfg.SetShowNudes(false)
	mc.handleCommand(CmdEnforceNudeFilter)
	nextLockScreenPath(t, osMock)

	assert.NotEqual(t, "nude", mc.State.CurrentImage.ID)
	mc.Store.(*MockImageStore).AssertNotCalled(t, "Remove", mock.Anything)
}

func TestLockScreen_FailureDoesNotAffectDesktop(t *testing.T) {
	osMock := newLockScreenOS()
	osMock.err = errors.New("lock screen unavailable")
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock, sfwImage("a"), sfwImage("b"))

	mc.applyImage(sfwImage("a"))
	nextLockScreenPath(t, osMock)
	assert.Equal(t, "a", mc.State.CurrentImage.ID)
	osMock.AssertCalled(t, "SetWallpaper", "a.jpg", mock.Anything)

	// A failed attempt must not stop later updates.
	mc.applyImage(sfwImage("b"))
	nextLockScreenPath(t, osMock)
	assert.Equal(t, "b", mc.State.CurrentImage.ID)
	osMock.AssertCalled(t, "SetWallpaper", "b.jpg", mock.Anything)
}

func TestLockScreen_UsesDifferentVisibleImage(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock, nsfwImage("hidden"), sfwImage("desktop"), sfwImage("lock"))

	mc.applyImage(sfwImage("desktop"))

	assert.Equal(t, "lock.jpg", nextLockScreenPath(t, osMock))
	assert.Equal(t, "lock", mc.lockImageID)
}

func TestLockScreen_FallsBackToDesktopImage(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock, nsfwImage("hidden"), sfwImage("desktop"))

	mc.applyImage(sfwImage("desktop"))

	assert.Equal(t, "desktop.jpg", nextLockScreenPath(t, osMock))
}

func TestLockScreen_OnlyPrimaryMonitor(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, sfwImage("a"), sfwImage("b"))

	mc.applyImage(sfwImage("a"))

	assert.Nil(t, mc.lockScreen)
	assertNoLockScreenUpdate(t, osMock)
}

func TestEnforceNudeFilter_ReplacesHiddenLockScreenImage(t *testing.T) {
	cfg := newNudeTestConfig(t)
	cfg.SetShowNudes(true)
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, cfg, osMock, nsfwImage("nude"), sfwImage("desktop"), sfwImage("other"))
	mc.State.CurrentImage = sfwImage("desktop")
	mc.lockImageID = "nude"

	cfg.SetShowNudes(false)
	mc.handleCommand(CmdEnforceNudeFilter)

	assert.Equal(t, "desktop", mc.State.CurrentImage.ID, "a visible desktop image should stay")
	assert.Equal(t, "other.jpg", nextLockScreenPath(t, osMock))
}

func TestEnforceNudeFilter_ReplacesUnknownLockScreenImage(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock, sfwImage("desktop"), sfwImage("other"))
	mc.State.CurrentImage = sfwImage("desktop")

	mc.handleCommand(CmdEnforceNudeFilter)

	assert.Equal(t, "other.jpg", nextLockScreenPath(t, osMock))
}

func TestEnforceNudeFilter_LeavesVisibleLockScreenImage(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock, sfwImage("desktop"), sfwImage("lock"))
	mc.State.CurrentImage = sfwImage("desktop")
	mc.lockImageID = "lock"

	mc.handleCommand(CmdEnforceNudeFilter)

	assertNoLockScreenUpdate(t, osMock)
}

func TestLockScreenSyncer_KeepsOnlyLatestRequest(t *testing.T) {
	release := make(chan struct{})
	setter := &blockingLockScreenSetter{release: release, paths: make(chan string, 10)}
	syncer := &lockScreenSyncer{setter: setter}

	syncer.request("first")
	assert.Equal(t, "first", <-setter.paths)
	syncer.request("second")
	syncer.request("third")
	close(release)

	assert.Equal(t, "third", <-setter.paths)
	select {
	case path := <-setter.paths:
		assert.Fail(t, "older request should have been dropped", path)
	case <-time.After(100 * time.Millisecond):
	}
}

type blockingLockScreenSetter struct {
	release chan struct{}
	paths   chan string
}

func (s *blockingLockScreenSetter) SetLockScreen(path string) error {
	s.paths <- path
	<-s.release
	return nil
}
