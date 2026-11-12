package ui

import (
	"net/url"

	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// allowForeground is letBrowserComeForward; tests replace it.
var allowForeground = letBrowserComeForward

// OpenURL opens u in the default browser and lets the browser come to the front (see
// letBrowserComeForward). The plugins open links through this (the tray's first row opens the
// artwork's page, the update row the release page), so it shadows fyne.App's OpenURL.
func (sa *SpiceApp) OpenURL(u *url.URL) error {
	if !allowForeground() {
		utilLog.Debugf("OpenURL: could not let the browser come to the front for %s", u)
	}
	err := sa.App.OpenURL(u)
	if err != nil {
		utilLog.Printf("OpenURL: failed to open %s: %v", u, err)
	}
	return err
}
