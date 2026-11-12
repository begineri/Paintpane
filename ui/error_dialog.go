package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
)

// Error dialog (docs/ui_refresh.md §12, ER1–ER3): the same box as the confirm dialogs, a bold title
// that says what did not work, one plain sentence in the app language and one OK button. The four
// failures people meet (wrong key, no connection, unknown wallhaven user, no collections) have
// their own sentences; anything else says "Something went wrong" and shows the original text in
// one small grey line.

// errorTexts returns the title and sentence for err, and the original text when the error is not
// one of the known ones.
func errorTexts(err error) (title, body, detail string) {
	kind, source := setting.Classify(err)
	switch kind {
	case setting.ErrorInvalidKey:
		if source == "" {
			source = i18n.T("The service")
		}
		return i18n.T("Couldn't verify the key"),
			i18n.Tf("{{.Source}} didn't accept this API key. Check it and try again.", map[string]string{"Source": source}), ""
	case setting.ErrorNetwork:
		return i18n.T("Couldn't connect"), i18n.T("Check your connection and try again."), ""
	case setting.ErrorUnknownUser:
		return i18n.T("Couldn't verify the username"), i18n.T("That user wasn't found, or the account is private."), ""
	case setting.ErrorNoCollection:
		return i18n.T("Can't sync favorites"), i18n.T("This wallhaven account has no collections yet."), ""
	}
	if err != nil {
		detail = err.Error()
	}
	return i18n.T("Something went wrong"), i18n.T("Try again later."), detail
}

// showErrorDialog shows err over parent. It replaces Fyne's dialog.ShowError, which prints the
// raw English error under the title "Error".
func showErrorDialog(parent fyne.Window, err error) {
	if parent == nil {
		return
	}
	title, bodyText, detail := errorTexts(err)

	var popup *widget.PopUp
	ok := newDialogButton(i18n.TMap("OK", map[string]string{"zh": "好", "zh-Hant": "好"}), buttonPrimary, func() { popup.Hide() })
	buttons := container.New(&rightRowLayout{gap: 8}, ok)

	parts := []fyne.CanvasObject{newDialogTitle(title), noInset(newDialogText(bodyText, dialogWidth))}
	if detail != "" {
		parts = append(parts, noInset(newErrorDetail(detail)))
	}
	parts = append(parts, inset(buttons, 12, 0, 0, 0))
	popup = newDialogPopup(parent, dialogWidth, vstack(8, parts...))
}

// newErrorDetail is the original error text: small, grey, monospace, wrapped anywhere.
func newErrorDetail(text string) *widget.RichText {
	rich := widget.NewRichText(&widget.TextSegment{
		Text: text,
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNamePlaceHolder,
			SizeName:  theme.SizeNameCaptionText,
			TextStyle: fyne.TextStyle{Monospace: true},
		},
	})
	rich.Wrapping = fyne.TextWrapBreak
	return rich
}

// ShowError displays a modal error dialog to the user.
func (sm *SettingsManager) ShowError(err error) {
	showErrorDialog(sm.prefsWindow, err)
}
