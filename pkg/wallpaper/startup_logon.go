package wallpaper

// StartedByWindowsAtLogon reports whether Windows started this run at logon: the Run entry's
// --autostart flag, or (the Microsoft Store edition's StartupTask passes no flag) a start in the
// first minutes after logon.
func StartedByWindowsAtLogon() bool { return detectAutostart() }
