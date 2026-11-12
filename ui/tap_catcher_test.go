package ui

import (
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
)

func TestTapCatcherCoversTheContentUnderIt(t *testing.T) {
	test.NewTempApp(t)
	closed := 0
	content := container.NewStack(widget.NewLabel("animation"), newTapCatcher(func() { closed++ }))
	w := test.NewWindow(content)
	t.Cleanup(w.Close)
	w.Resize(content.MinSize())

	test.TapCanvas(w.Canvas(), content.Position().AddXY(content.MinSize().Width/2, content.MinSize().Height/2))
	assert.Equal(t, 1, closed, "a click anywhere over the content closes it")
}
