package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/dixieflatline76/Spice/v2/config"
)

// Paintpane is built with plain `go build`, not `fyne package`, so no FyneApp.toml metadata is
// embedded. Without it Fyne assumes the app has not moved its UI calls to fyne.Do and checks the
// calling goroutine on every refresh of every object, which it does by formatting a stack trace
// (runtime.Stack). Switching Appearance or Accent color refreshes every object of every window;
// in the running app those checks were about three quarters of a switch's CPU time (with this
// and the source logos decoded once, a switch went from 239 to 74 ms; POST /debug/prefs/select).
//
// All UI work here already goes through fyne.Do (the log never shows Fyne's "Error in Fyne call
// thread"), and fyne.DoAndWait is only called from the debug server's HTTP goroutines, so the app
// declares the migration the way FyneApp.toml's [Migrations] fyneDo = true would. ID and Name are
// set so that dev builds do not look for a FyneApp.toml, which would replace this. Fyne reads the
// flag once, the first time it checks, so it is set when the package loads.
func init() {
	app.SetMetadata(paintpaneMetadata())
}

func paintpaneMetadata() fyne.AppMetadata {
	return fyne.AppMetadata{
		ID:         config.AppName, // the ID the app is created with (app.NewWithID)
		Name:       config.AppName,
		Migrations: map[string]bool{"fyneDo": true},
	}
}
