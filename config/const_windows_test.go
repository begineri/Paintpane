//go:build windows

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func makeJunction(t *testing.T, link, target string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink failed: %v: %s", err, out)
	}
}

func TestResolveLinkedDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "moved")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "cache")
	makeJunction(t, link, target)

	if got := ResolveLinkedDir(link); got != target {
		t.Errorf("junction: got %q, want %q", got, target)
	}
	if got := ResolveLinkedDir(target); got != target {
		t.Errorf("plain directory: got %q, want %q", got, target)
	}
	missing := filepath.Join(tmp, "missing")
	if got := ResolveLinkedDir(missing); got != missing {
		t.Errorf("missing directory: got %q, want %q", got, missing)
	}
}

func TestGetCacheDir_FollowsMovedCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	working := GetWorkingDir()
	if err := os.MkdirAll(working, 0755); err != nil {
		t.Fatal(err)
	}

	plain := filepath.Join(working, "cache")
	if got := GetCacheDir(); got != plain {
		t.Errorf("no cache yet: got %q, want %q", got, plain)
	}

	target := filepath.Join(root, "moved_cache")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, plain, target)

	if got := GetCacheDir(); got != target {
		t.Errorf("moved cache: got %q, want %q", got, target)
	}
	if err := os.MkdirAll(filepath.Join(GetCacheDir(), "curation"), 0755); err != nil {
		t.Errorf("creating a directory in the moved cache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "curation")); err != nil {
		t.Errorf("the directory must be on the target: %v", err)
	}
}
