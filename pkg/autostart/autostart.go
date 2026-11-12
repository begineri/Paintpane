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

// Refresh keeps an existing registration pointing at the running program: after an update moved
// the exe, or when an older installer registered it without the flag. It never creates a
// registration and never turns a switched-off entry back on. It returns whether it rewrote the
// command.
func Refresh(s Store, exe string) (bool, error) {
	if s == nil {
		return false, nil
	}
	current, registered, err := s.Command()
	if err != nil || !registered {
		return false, err
	}
	want := Command(exe)
	if sameCommand(current, want) {
		return false, nil
	}
	if err := s.SetCommand(want); err != nil {
		return false, err
	}
	return true, nil
}

func sameCommand(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
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
