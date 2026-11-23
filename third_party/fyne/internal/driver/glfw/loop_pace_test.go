package glfw

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

// PAINTPANE: the event ticker runs at 60 Hz only while a window is visible.
func TestEventPaceFollowsVisibleWindows(t *testing.T) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	p := eventPace{tick: tick, fast: true}

	hidden := &window{}
	shown := &window{visible: true}

	p.update([]fyne.Window{hidden})
	if p.fast {
		t.Fatal("with only hidden windows the loop should slow down")
	}
	p.update(nil)
	if p.fast {
		t.Fatal("with no windows the loop should stay slow")
	}
	p.update([]fyne.Window{hidden, shown})
	if !p.fast {
		t.Fatal("a visible window should bring the loop back to 60 Hz")
	}
	shown.visible = false
	p.update([]fyne.Window{hidden, shown})
	if p.fast {
		t.Fatal("once the window is hidden the loop should slow down again")
	}
}

// The slow pace still ticks, so hidden windows are serviced.
func TestEventPaceIdleTickFires(t *testing.T) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	p := eventPace{tick: tick, fast: true}
	p.update(nil)
	select {
	case <-tick.C:
	case <-time.After(5 * idleEventTick):
		t.Fatalf("no tick within %v while idle", 5*idleEventTick)
	}
}
