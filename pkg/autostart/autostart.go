// Package autostart registers the program to start with Windows (docs/ui_refresh.md §12, S3-6).
//
// The "Start with Windows" switch shows whether the program has registered itself in the current
// user's Run key, under the value name the installer uses: no administrator rights, nothing outside
// the user's own profile. Turning it on writes `"<this exe>" --autostart`, turning it off deletes the
// value. If the person switched the entry off in Windows' own Startup apps page (Task Manager,
// Settings), the switch reads as off, and turning it on switches the entry back on.
//
// The switch is on by default: the first time someone accepts the licence, the program registers
// itself once (RegisterOnFirstRun), unless an earlier version was accepted (an upgrade) or the
// installer already asked. After that only the switch decides.
//
// Everything goes through a Store, so tests never touch the real Run key, and development builds
// use a store that only remembers (see default_dev.go).
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValueName is the name of the Run value. assets-src/installer/Paintpane.iss writes the same one.
const ValueName = "Paintpane"

// Flag is what the Run command passes, so the program knows Windows started it at logon
// (pkg/wallpaper waits before fetching and processing then).
const Flag = "--autostart"

// Store is the place the registration lives: the Run key and Windows' record of whether the person
// switched the entry off.
type Store interface {
	// Command returns the command in the Run value, and whether the value exists.
	Command() (command string, registered bool, err error)
	// SetCommand writes the Run value.
	SetCommand(command string) error
	// DeleteCommand removes the Run value. Removing a value that is not there is not an error.
	DeleteCommand() error
	// DisabledByUser reports whether Windows' Startup apps page has the entry switched off.
	DisabledByUser() (bool, error)
	// ClearDisabledByUser removes that record, so a registered entry runs again.
	ClearDisabledByUser() error
}

// Command is the Run command for an executable: the quoted path and the autostart flag.
func Command(exe string) string {
	return `"` + exe + `" ` + Flag
}

// Enabled reports whether the program will start with Windows: it is registered and Windows has
// not been told to skip it.
func Enabled(s Store) (bool, error) {
	if s == nil {
		return false, nil
	}
	_, registered, err := s.Command()
	if err != nil || !registered {
		return false, err
	}
	disabled, err := s.DisabledByUser()
	if err != nil {
		return false, err
	}
	return !disabled, nil
}

// SetEnabled registers exe to start with Windows, or removes the registration.
func SetEnabled(s Store, exe string, on bool) error {
	if s == nil {
		return fmt.Errorf("starting with Windows is not available here")
	}
	if !on {
		return s.DeleteCommand()
	}
	if err := s.SetCommand(Command(exe)); err != nil {
		return err
	}
	return s.ClearDisabledByUser()
}

// Refreshed is what Refresh found and did.
type Refreshed int

const (
	RefreshNotRegistered Refreshed = iota // nothing registered: nothing created
	RefreshCurrent                        // the entry already starts this program as it should
	RefreshOtherCopy                      // the entry starts another copy that still exists: left alone
	RefreshRewritten                      // rewritten to start this program (see Refresh)
)

func (r Refreshed) String() string {
	switch r {
	case RefreshNotRegistered:
		return "not registered"
	case RefreshCurrent:
		return "already current"
	case RefreshOtherCopy:
		return "another copy that still exists is registered; left as it is"
	case RefreshRewritten:
		return "rewritten"
	}
	return "unknown"
}

// Refresh repairs an existing registration at start. It rewrites the entry to start exe only when
// the program the entry names is gone (the person moved, renamed or deleted it), or when the entry
// already names exe but differs from Command(exe) (an older installer wrote it without the flag).
// An entry that names another copy which still exists is left alone: any other copy of the
// program, started once from wherever it lies, must not take the logon start over from the copy
// the person chose (2026-11-21, a measurement copy in a scratch folder took it over and was then
// deleted; owner's rule of 2026-11-23). Turning the switch on (SetEnabled) still registers exe.
//
// Refresh never creates a registration and never turns a switched-off entry back on.
func Refresh(s Store, exe string) (Refreshed, error) {
	if s == nil {
		return RefreshNotRegistered, nil
	}
	current, registered, err := s.Command()
	if err != nil || !registered {
		return RefreshNotRegistered, err
	}
	want := Command(exe)
	if sameCommand(current, want) {
		return RefreshCurrent, nil
	}
	if target := CommandExe(current); !SamePath(target, exe) && FileExists(target) {
		return RefreshOtherCopy, nil
	}
	if err := s.SetCommand(want); err != nil {
		return RefreshNotRegistered, err
	}
	return RefreshRewritten, nil
}

func sameCommand(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// normalPath makes a path from a Run command comparable: surrounding spaces and quotes removed,
// made absolute (a relative path is taken from the current folder) and cleaned. Case and 8.3 short
// names are left to SamePath.
func normalPath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), `"`)
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

// SamePath reports whether a and b name the same file: equal ignoring case once made absolute and
// cleaned, or, when both exist, the same file on disk (an 8.3 short name, a different case, a link).
func SamePath(a, b string) bool {
	na, nb := normalPath(a), normalPath(b)
	if na == "" || nb == "" {
		return false
	}
	if strings.EqualFold(na, nb) {
		return true
	}
	ia, errA := os.Stat(na)
	ib, errB := os.Stat(nb)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// FileExists reports whether path (as SamePath reads it) is an existing file, not a folder.
func FileExists(path string) bool {
	p := normalPath(path)
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// SetupMarker is the file assets-src/installer/Paintpane.iss writes next to the exe it installs. Its presence means the
// installer has already asked whether to start with Windows (its "Start with Windows" task), so the
// first run leaves that choice alone, ticked or not. The portable exe has no such file.
const SetupMarker = "setup.ini"

// InstalledBySetup reports whether exe was installed by the installer (SetupMarker next to it).
func InstalledBySetup(exe string) bool {
	if exe == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(exe), SetupMarker))
	return err == nil && !info.IsDir()
}

// FirstRun is what RegisterOnFirstRun decided.
type FirstRun int

const (
	FirstRunRegistered        FirstRun = iota // registered the program
	FirstRunAcceptedBefore                    // not a first run: an earlier version was accepted
	FirstRunInstallerAsked                    // the installer asked; its answer stands
	FirstRunAlreadyRegistered                 // something registered it already; left as it is
)

func (f FirstRun) String() string {
	switch f {
	case FirstRunRegistered:
		return "registered"
	case FirstRunAcceptedBefore:
		return "an earlier version was accepted"
	case FirstRunInstallerAsked:
		return "the installer asked"
	case FirstRunAlreadyRegistered:
		return "already registered"
	}
	return "unknown"
}

// RegisterOnFirstRun registers exe to start with Windows on a first run, which is how the switch
// is on by default for the portable exe. It registers only when acceptedBefore is false (no licence
// acceptance of any version was recorded: not an upgrade), the installer did not ask (no
// SetupMarker), and nothing is registered yet (a registration Windows' Startup apps page switched
// off stays off). The caller makes sure it runs once.
func RegisterOnFirstRun(s Store, exe string, acceptedBefore bool) (FirstRun, error) {
	if acceptedBefore {
		return FirstRunAcceptedBefore, nil
	}
	if InstalledBySetup(exe) {
		return FirstRunInstallerAsked, nil
	}
	if s == nil {
		return 0, fmt.Errorf("starting with Windows is not available here")
	}
	_, registered, err := s.Command()
	if err != nil {
		return 0, err
	}
	if registered {
		return FirstRunAlreadyRegistered, nil
	}
	if err := s.SetCommand(Command(exe)); err != nil {
		return 0, err
	}
	return FirstRunRegistered, nil
}

// Executable returns the path of the running program.
func Executable() (string, error) {
	return os.Executable()
}
