package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// tapCatcher is an invisible widget that calls onTapped when it is clicked. Stacked
// over other content, it lets the whole area act as one button.
type tapCatcher struct {
	widget.BaseWidget
	onTapped func()
}

var (
	_ fyne.Tappable      = (*tapCatcher)(nil)
	_ desktop.Cursorable = (*tapCatcher)(nil)
)

func newTapCatcher(onTapped func()) *tapCatcher {
	t := &tapCatcher{onTapped: onTapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapCatcher) Tapped(_ *fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

func (t *tapCatcher) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *tapCatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(&fyne.Container{})
}
