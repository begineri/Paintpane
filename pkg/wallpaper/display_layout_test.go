package wallpaper

import (
	"runtime"
	"strings"
	"testing"
)

func TestMonitorWatchChecksOnlyWhenTheLayoutChanges(t *testing.T) {
	var w monitorWatch
	if !w.due("A=0,0,1920,1080") {
		t.Fatal("the first tick must check fully")
	}
	for i := 1; i < monitorFullCheckEvery; i++ {
		if w.due("A=0,0,1920,1080") {
			t.Fatalf("tick %d: an unchanged layout should not be checked fully", i)
		}
	}
	if !w.due("A=0,0,1920,1080") {
		t.Fatalf("after %d unchanged ticks the watcher should check fully anyway", monitorFullCheckEvery)
	}
	if w.due("A=0,0,1920,1080") {
		t.Fatal("the backstop check should restart the count")
	}
	if !w.due("A=0,0,2560,1440") {
		t.Fatal("a changed rectangle must be checked fully")
	}
	if !w.due("A=0,0,2560,1440;B=2560,0,4480,1080") {
		t.Fatal("an added monitor must be checked fully")
	}
}

func TestMonitorWatchChecksWhenTheLayoutIsUnknown(t *testing.T) {
	var w monitorWatch
	for i := 0; i < 3; i++ {
		if !w.due("") {
			t.Fatalf("tick %d: an unreadable layout must always be checked fully", i)
		}
	}
}

func TestDisplayLayoutKey(t *testing.T) {
	key := displayLayoutKey()
	if runtime.GOOS != "windows" {
		if key != "" {
			t.Fatalf("got %q, want \"\" outside Windows", key)
		}
		return
	}
	if key == "" {
		t.Skip("no display attached to this session")
	}
	if !strings.Contains(key, "=") {
		t.Fatalf("unexpected layout key %q", key)
	}
	// Many calls must reuse the one callback (the runtime allows about 2000).
	for i := 0; i < 2500; i++ {
		if again := displayLayoutKey(); again != key {
			t.Fatalf("call %d: layout key changed from %q to %q", i, key, again)
		}
	}
}
