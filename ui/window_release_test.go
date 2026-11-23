package ui

import (
	"image/color"
	"runtime"
	"testing"
	"time"
	"weak"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shortWindowPurge(t *testing.T) {
	t.Helper()
	old := windowPurgeAfter
	windowPurgeAfter = 30 * time.Millisecond
	t.Cleanup(func() { windowPurgeAfter = old })
}

func waitWindowPurge() {
	time.Sleep(3 * windowPurgeAfter)
	flushUI()
	time.Sleep(windowPurgeAfter / 2)
	flushUI()
}

// A window closed through releaseClosingWindow shows nothing any more, and a minute later its
// widgets are gone from Fyne's caches, so they can be collected.
func TestReleaseClosingWindow_ClosedWindowIsCollected(t *testing.T) {
	shortWindowPurge(t)
	test.NewTempApp(t)

	var labels []weak.Pointer[widget.Label]
	for i := 0; i < 3; i++ {
		w := test.NewWindow(nil)
		l := widget.NewLabel("Tune image")
		w.SetContent(container.NewVBox(l, widget.NewButton("Reset", func() {})))
		w.SetOnClosed(func() { releaseClosingWindow(w) })
		flushUI()
		labels = append(labels, weak.Make(l))
		w.Close()
		r, ok := w.Content().(*canvas.Rectangle)
		require.True(t, ok, "the closed window should show an empty rectangle")
		assert.Equal(t, color.Transparent, r.FillColor)
	}
	waitWindowPurge()
	runtime.GC()
	runtime.GC()
	for i, l := range labels {
		assert.Nil(t, l.Value(), "the label of closed window %d is still reachable", i+1)
	}
}

// Windows closed shortly after one another share one purge, after the last of them.
func TestReleaseClosingWindow_OnePurgeForWindowsClosedTogether(t *testing.T) {
	shortWindowPurge(t)
	test.NewTempApp(t)
	before := windowPurges
	for i := 0; i < 3; i++ {
		w := test.NewWindow(widget.NewLabel("menu"))
		releaseClosingWindow(w)
		w.Close()
	}
	waitWindowPurge()
	assert.Equal(t, before+1, windowPurges)
}
