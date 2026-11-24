package autostart

import (
	"strings"
	"time"
)

// The portable exe and the Microsoft Store edition cannot run at the same time (they share the
// single-instance lock), and the Store edition cannot change the portable exe's Run entry (its
// registry writes are virtualized). When both start at logon, whichever starts first would run,
// and which one that is changes from day to day. So the Store edition gives way: started at logon
// while the portable exe is set to start too, it waits a little for the portable exe and quits if
// it comes, writing why to its log. Nothing is shown to the person (owner, 2026-11-21).

// OtherCopyAtLogon reports whether Windows will start another copy of Paintpane at logon: the Run
// entry is there, not switched off in Startup apps, and names an exe that exists and is not self.
// It returns that exe.
func OtherCopyAtLogon(s Store, self string) (string, bool) {
	on, err := Enabled(s)
	if err != nil || !on {
		return "", false
	}
	cmd, _, err := s.Command()
	if err != nil {
		return "", false
	}
	exe := CommandExe(cmd)
	if exe == "" || SamePath(exe, self) || !FileExists(exe) {
		return "", false
	}
	return exe, true
}

// CommandExe returns the program a Run command starts: the quoted path, or everything up to the
// first space when it is not quoted.
func CommandExe(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.HasPrefix(cmd, `"`) {
		if end := strings.Index(cmd[1:], `"`); end >= 0 {
			return cmd[1 : end+1]
		}
		return strings.Trim(cmd, `"`)
	}
	if i := strings.IndexByte(cmd, ' '); i >= 0 {
		return cmd[:i]
	}
	return cmd
}

// WaitFor calls running every step until it reports true or timeout has passed, and reports
// whether it did. Tests pass their own sleep.
func WaitFor(running func() bool, timeout, step time.Duration, sleep func(time.Duration)) bool {
	for waited := time.Duration(0); ; waited += step {
		if running() {
			return true
		}
		if waited >= timeout {
			return false
		}
		sleep(step)
	}
}
