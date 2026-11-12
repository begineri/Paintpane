//go:build !windows

package ui

import "fyne.io/fyne/v2"

// styleTitleBar does nothing outside Windows: only Windows draws a title bar the app cannot theme.
func styleTitleBar(fyne.Window) {}

// styleOpenTitleBars does nothing outside Windows.
func styleOpenTitleBars() {}
