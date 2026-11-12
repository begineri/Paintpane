package ui

import (
	"regexp"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// Add collection dialog (docs/ui_refresh.md §12, AQ1–AQ6), opened by "Add new collection" on
// Pexels, wallhaven and Wikimedia Commons. The dialog style is the confirm dialogs' (A-1), 420
// wide: a field for the address (labelled for the source), a field for the name the collection
// has in the list, a red line only while the address is wrong, an Active switch and the buttons
// Cancel / Add. Add stays grey until the form is valid.

const (
	addQueryDialogWidth float32 = 420
	maxQueryURLLen              = 500
	maxQueryNameLen             = 100
)

// ShowAddQueryDialog opens the modal for adding image queries.
func (sm *SettingsManager) ShowAddQueryDialog(cfg schema.AddQueryConfig, initialURL, initialDesc string, onAdded func()) {
	utilLog.Debugf("ShowAddQueryDialog: Triggered with URL: %s", initialURL)
	if sm.prefsWindow == nil {
		return
	}

	urlRe := compileOrNil(cfg.URLValidator)
	nameRe := compileOrNil(cfg.DescValidator)

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder(cfg.URLPlaceholder)
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder(cfg.DescPlaceholder)

	problem := widget.NewLabel("") // red, and empty while nothing is wrong
	problem.Importance = widget.DangerImportance
	problem.Wrapping = fyne.TextWrapWord
	active := newToggleSwitch(true, nil)

	cancel := newDialogButton(i18n.T("Cancel"), buttonNeutral, nil)
	add := newDialogButton(i18n.T("Add"), buttonPrimary, nil)
	add.Disable()

	// check says whether the form can be added and shows what is wrong with the address, but only
	// once something has been typed there.
	check := func() {
		urlText, nameText := urlEntry.Text, nameEntry.Text
		msg := ""
		valid := urlText != "" && nameText != ""
		if urlText != "" {
			if urlRe != nil && !urlRe.MatchString(urlText) {
				msg, valid = cfg.URLErrorMsg, false
			} else if cfg.ValidateFunc != nil {
				if err := cfg.ValidateFunc(urlText, nameText); err != nil {
					msg, valid = err.Error(), false
				}
			}
		}
		if nameText != "" && nameRe != nil && !nameRe.MatchString(nameText) {
			if msg == "" {
				msg = cfg.DescErrorMsg
			}
			valid = false
		}
		problem.SetText(msg)
		if valid {
			add.Enable()
		} else {
			add.Disable()
		}
	}
	limit := func(entry *widget.Entry, max int) func(string) {
		return func(s string) {
			if len(s) > max {
				entry.SetText(s[:max])
				return
			}
			check()
		}
	}
	urlEntry.OnChanged = limit(urlEntry, maxQueryURLLen)
	nameEntry.OnChanged = limit(nameEntry, maxQueryNameLen)
	urlEntry.SetText(initialURL)
	nameEntry.SetText(initialDesc)

	field := func(label string, entry *widget.Entry) fyne.CanvasObject {
		return vstack(4, newStyledText(label, 12, false, theme.ColorNamePlaceHolder), entry)
	}
	parts := []fyne.CanvasObject{newDialogTitle(cfg.Title)}
	if cfg.Description != "" {
		parts = append(parts, noInset(newDialogText(cfg.Description, addQueryDialogWidth)))
	}
	parts = append(parts,
		field(cfg.URLLabel, urlEntry),
		field(i18n.T("Name"), nameEntry),
		noInset(problem),
		container.NewBorder(nil, nil, widget.NewLabel(i18n.T("Active")), hug(active)),
		inset(container.New(&rightRowLayout{gap: 8}, cancel, add), 8, 0, 0, 0),
	)
	var popup *widget.PopUp
	popup = newDialogPopup(sm.prefsWindow, addQueryDialogWidth, vstack(8, parts...))

	cancel.OnTapped = func() { popup.Hide() }
	add.OnTapped = func() {
		newID, err := cfg.AddHandler(nameEntry.Text, urlEntry.Text, active.Checked)
		if err != nil {
			problem.SetText(err.Error())
			return
		}
		if onAdded != nil {
			onAdded()
		}
		if active.Checked {
			sm.SetRefreshFlag(newID)
			sm.checkAndEnableApply()
		}
		popup.Hide()
	}
	utilLog.Debug("ShowAddQueryDialog: shown")
}

func compileOrNil(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}
