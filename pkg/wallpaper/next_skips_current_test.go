package wallpaper

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// Next never shows the picture already on screen while another can be shown. After a start
// restores last time's wallpaper, a new deck may hold it first (A2 §14.9 #2 follow-up).
func TestNextSkipsTheImageOnScreen(t *testing.T) {
	dir := t.TempDir()
	store := NewImageStore()
	store.SetAsyncSave(false)
	add := func(id string) {
		p := filepath.Join(dir, id+".jpg")
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
		require.True(t, store.Add(provider.Image{ID: id, FilePath: p, DerivativePaths: map[string]string{"800x500": p}}))
	}
	add("a")
	desktop := &recordingOS{monitors: []Monitor{{ID: 0, Rect: image.Rect(0, 0, 800, 500)}}, set: map[int][]string{}}
	mc := NewMonitorController(0, desktop.monitors[0], store, nil, desktop, nil, nil)
	mc.State.CurrentID = "a"
	mc.State.CurrentImage, _ = store.GetByID("a")

	// Only the image on screen: it stays, and the monitor does not wait for a fetch.
	mc.next(true)
	assert.Empty(t, desktop.set[0])
	assert.Equal(t, "a", mc.State.CurrentID)
	assert.False(t, mc.State.WaitingForImages)

	// Another image: Next shows it, even when the image on screen is first in the deck.
	add("b")
	mc.State.ShuffleIDs = []string{"a", "b"}
	mc.State.RandomPos = 0
	mc.next(true)
	require.Len(t, desktop.set[0], 1)
	assert.Equal(t, "b", mc.State.CurrentID)
}
