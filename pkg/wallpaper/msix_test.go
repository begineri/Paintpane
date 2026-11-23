//go:build windows
// +build windows

package wallpaper

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/config"
)

// fakeMSIX pretends the process runs in a package whose Pictures and Documents are in a temp
// folder, and keeps the in-use list there too. It restores everything when the test ends.
func fakeMSIX(t *testing.T) (pictures, documents, stateFile string) {
	t.Helper()
	dir := t.TempDir()
	pictures = filepath.Join(dir, "Pictures", "Paintpane")
	documents = filepath.Join(dir, "Documents", "Paintpane")
	stateFile = filepath.Join(dir, "LocalCache", "msix_staged.json")

	origFamily, origPic, origDoc, origState := msixPackageFamilyName, msixPictureStaging, msixDocumentStaging, msixStateFile
	t.Cleanup(func() {
		msixPackageFamilyName, msixPictureStaging, msixDocumentStaging, msixStateFile = origFamily, origPic, origDoc, origState
		msixInUse = nil
		msixPending = map[string]int{}
	})
	msixPackageFamilyName = "TestPackage_123"
	msixPictureStaging = pictures
	msixDocumentStaging = documents
	msixStateFile = func() string { return stateFile }
	msixInUse = nil
	msixPending = map[string]int{}
	return pictures, documents, stateFile
}

// sourceImage writes a fake processed image and returns its path.
func sourceImage(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(p, []byte("image "+name), 0o600))
	return p
}

func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// setVia calls withMSIXPicture and returns the path Windows was given.
func setVia(t *testing.T, src, role string) string {
	t.Helper()
	var got string
	require.NoError(t, withMSIXPicture(src, role, func(p string) error { got = p; return nil }))
	return got
}

func TestMSIXOutsideAPackageChangesNothing(t *testing.T) {
	origFamily := msixPackageFamilyName
	t.Cleanup(func() { msixPackageFamilyName = origFamily })
	msixPackageFamilyName = ""

	src := sourceImage(t, t.TempDir(), "a.jpg")
	assert.Equal(t, src, setVia(t, src, msixDesktopRole(0)), "the web version sets the original file")
	assert.Equal(t, src, resolveMSIXDocumentPath(src))
	assert.Nil(t, msixInUse, "nothing is recorded outside a package")
}

func TestMSIXStagesIntoPaintpaneFolders(t *testing.T) {
	pictures, documents, _ := fakeMSIX(t)
	assert.NoDirExists(t, pictures, "the picture folder is made on first use")
	assert.NoDirExists(t, documents, "the document folder is made on first use")

	workingDir := config.GetWorkingDir()
	src := sourceImage(t, t.TempDir(), "MetMuseum_1.jpg")
	staged := setVia(t, src, msixDesktopRole(0))
	assert.Equal(t, filepath.Join(pictures, "MetMuseum_1.jpg"), staged)
	assert.FileExists(t, staged)
	assert.NoDirExists(t, documents, "setting a wallpaper does not make the document folder")

	// Documents keep their place under the working folder.
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "cache", "metmuseum"), 0o755))
	srcDoc := filepath.Join(workingDir, "cache", "metmuseum", "american_art.html")
	require.NoError(t, os.WriteFile(srcDoc, []byte("<html>fake gallery</html>"), 0o600))
	t.Cleanup(func() { _ = os.Remove(srcDoc) })
	stagedDoc := resolveMSIXDocumentPath(srcDoc)
	assert.Equal(t, filepath.Join(documents, "cache", "metmuseum", "american_art.html"), stagedDoc)
	assert.FileExists(t, stagedDoc)
}

func TestMSIXKeepsOnlyThePicturesInUse(t *testing.T) {
	pictures, _, stateFile := fakeMSIX(t)
	srcDir := t.TempDir()

	setVia(t, sourceImage(t, srcDir, "a.jpg"), msixDesktopRole(0))
	setVia(t, sourceImage(t, srcDir, "b.jpg"), msixDesktopRole(1))
	setVia(t, sourceImage(t, srcDir, "c.jpg"), msixLockScreenRole)
	assert.Equal(t, []string{"a.jpg", "b.jpg", "c.jpg"}, filesIn(t, pictures))

	// The user's own files and folders stay.
	require.NoError(t, os.WriteFile(filepath.Join(pictures, "notes.txt"), []byte("mine"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(pictures, "album"), 0o755))
	// A copy left half-done by a crash goes.
	require.NoError(t, os.WriteFile(filepath.Join(pictures, "x.jpg.tmp"), []byte("half"), 0o600))

	setVia(t, sourceImage(t, srcDir, "d.jpg"), msixDesktopRole(0))
	setVia(t, sourceImage(t, srcDir, "e.png"), msixDesktopRole(0))
	assert.Equal(t, []string{"album", "b.jpg", "c.jpg", "e.png", "notes.txt"}, filesIn(t, pictures))

	// The same picture on two monitors is one copy, kept while either shows it.
	setVia(t, sourceImage(t, srcDir, "e.png"), msixDesktopRole(1))
	assert.Equal(t, []string{"album", "c.jpg", "e.png", "notes.txt"}, filesIn(t, pictures))
	setVia(t, sourceImage(t, srcDir, "f.jpg"), msixDesktopRole(0))
	assert.Equal(t, []string{"album", "c.jpg", "e.png", "f.jpg", "notes.txt"}, filesIn(t, pictures))

	// After a restart the list is read back, so the lock screen's copy is not taken for junk.
	assert.FileExists(t, stateFile)
	msixInUse = nil
	setVia(t, sourceImage(t, srcDir, "g.jpg"), msixDesktopRole(0))
	assert.Equal(t, []string{"album", "c.jpg", "e.png", "g.jpg", "notes.txt"}, filesIn(t, pictures))
}

func TestMSIXFailedSetKeepsThePictureInUse(t *testing.T) {
	pictures, _, _ := fakeMSIX(t)
	srcDir := t.TempDir()

	setVia(t, sourceImage(t, srcDir, "a.jpg"), msixDesktopRole(0))
	err := withMSIXPicture(sourceImage(t, srcDir, "b.jpg"), msixDesktopRole(0), func(string) error {
		return errors.New("COM said no")
	})
	require.Error(t, err)
	assert.Equal(t, "a.jpg", msixInUse[msixDesktopRole(0)], "the monitor still shows a.jpg")

	// The failed copy is cleaned up by the next change.
	setVia(t, sourceImage(t, srcDir, "c.jpg"), msixLockScreenRole)
	assert.Equal(t, []string{"a.jpg", "c.jpg"}, filesIn(t, pictures))
}

func TestMSIXCleanupSparesACopyAboutToBeSet(t *testing.T) {
	pictures, _, _ := fakeMSIX(t)
	srcDir := t.TempDir()

	// Monitor 1 has copied b.jpg but Windows has not taken it yet when monitor 0 changes.
	err := withMSIXPicture(sourceImage(t, srcDir, "b.jpg"), msixDesktopRole(1), func(p string) error {
		setVia(t, sourceImage(t, srcDir, "a.jpg"), msixDesktopRole(0))
		assert.FileExists(t, p, "monitor 0's cleanup must not delete monitor 1's new copy")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.jpg", "b.jpg"}, filesIn(t, pictures))
}

func TestMSIXRestagesAChangedPicture(t *testing.T) {
	_, _, _ = fakeMSIX(t)
	srcDir := t.TempDir()
	src := sourceImage(t, srcDir, "a.jpg")
	staged := setVia(t, src, msixDesktopRole(0))

	// Tune Image rewrites the same file: same size, newer time.
	require.NoError(t, os.WriteFile(src, []byte("IMAGE a.jpg"), 0o600))
	later := mustStat(t, staged).ModTime().Add(2e9)
	require.NoError(t, os.Chtimes(src, later, later))
	setVia(t, src, msixDesktopRole(0))
	data, err := os.ReadFile(staged)
	require.NoError(t, err)
	assert.Equal(t, "IMAGE a.jpg", string(data))
}

func mustStat(t *testing.T, p string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(p)
	require.NoError(t, err)
	return fi
}
