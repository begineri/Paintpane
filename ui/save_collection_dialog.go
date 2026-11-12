package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// Save collection dialog, shown after photos were picked through an OAuthPickerItem (only Google
// Photos has one, and that source is hidden: ui_refresh.md §12, P10). It uses the Add collection
// dialog's style (AQ1–AQ6) in place of Fyne's own form dialog: a Name field with the suggested
// name, an Active switch, and Cancel / Save. Save stays grey while the name is empty. The internal
// googlephotos:// address the old dialog showed is not shown.

// showSaveCollectionDialog asks for the new collection's name. save stores it and may fail, in
// which case the error shows in red and the dialog stays open; cancel runs when the person backs
// out.
func (sm *SettingsManager) showSaveCollectionDialog(defaultName string, save func(name string, active bool) error, cancel func()) {
	parent := sm.GetSettingsWindow()
	if parent == nil {
		if cancel != nil {
			cancel()
		}
		return
	}

	nameEntry := widget.NewEntry()
	problem := widget.NewLabel("")
	problem.Importance = widget.DangerImportance
	problem.Wrapping = fyne.TextWrapWord
	active := newToggleSwitch(true, nil)

	cancelBtn := newDialogButton(i18n.T("Cancel"), buttonNeutral, nil)
	saveBtn := newDialogButton(i18n.T("Save"), buttonPrimary, nil)

	nameEntry.OnChanged = func(s string) {
		if len(s) > maxQueryNameLen {
			nameEntry.SetText(s[:maxQueryNameLen])
			return
		}
		problem.SetText("")
		if strings.TrimSpace(s) == "" {
			saveBtn.Disable()
		} else {
			saveBtn.Enable()
		}
	}
	nameEntry.SetText(defaultName)

	popup := newDialogPopup(parent, addQueryDialogWidth, vstack(8,
		newDialogTitle(i18n.T("Save collection")),
		vstack(4, newStyledText(i18n.T("Name"), 12, false, theme.ColorNamePlaceHolder), nameEntry),
		noInset(problem),
		container.NewBorder(nil, nil, widget.NewLabel(i18n.T("Active")), hug(active)),
		inset(container.New(&rightRowLayout{gap: 8}, cancelBtn, saveBtn), 8, 0, 0, 0),
	))

	cancelBtn.OnTapped = func() {
		popup.Hide()
		if cancel != nil {
			cancel()
		}
	}
	saveBtn.OnTapped = func() {
		if err := save(strings.TrimSpace(nameEntry.Text), active.Checked); err != nil {
			problem.SetText(err.Error())
			return
		}
		popup.Hide()
	}
}
