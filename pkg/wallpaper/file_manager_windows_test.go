//go:build windows

package wallpaper

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileManager_ResolvesJunctionRoot(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "moved_cache")
	require.NoError(t, os.Mkdir(target, 0755))
	link := filepath.Join(tmp, "wallpaper_downloads")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	require.NoError(t, err, string(out))

	fm := NewFileManager(link)

	assert.Equal(t, target, fm.GetDownloadDir())
	require.NoError(t, fm.EnsureDirs())
	assert.DirExists(t, filepath.Join(target, FittedRootDir, FullArtDir, StandardDir))
	assert.DirExists(t, filepath.Join(target, FittedRootDir, FramedArtDir, StandardDir))
}

func TestNewFileManager_KeepsPlainRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "wallpaper_downloads")

	assert.Equal(t, root, NewFileManager(root).GetDownloadDir(), "a root that does not exist yet")

	require.NoError(t, os.Mkdir(root, 0755))
	assert.Equal(t, root, NewFileManager(root).GetDownloadDir(), "a plain directory")
}
