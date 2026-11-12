package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// First-run licence window (docs/ui_refresh.md §12, E1–E7): the agreement in a scrolling card and
// the buttons Decline / Accept at its right. There is no prompt line; the window's title and the
// Accept button already say what it is for.

// eulaTextFor returns the agreement for a language code (i18n.EffectiveLanguage): the Chinese
// translation for Simplified or Traditional Chinese, the English text for everything else. The
// English text prevails, and its hash is what the acceptance record stores.
func eulaTextFor(am *asset.Manager, lang string) (string, error) {
	name := "eula.txt"
	if lang == "zh" || lang == "zh-Hant" || strings.HasPrefix(lang, "zh-") {
		name = "eula.zh.txt"
	}
	text, err := am.GetText(name)
	if err != nil && name != "eula.txt" {
		return am.GetText("eula.txt")
	}
	return text, err
}

// newEULAContent builds the window's content: the agreement and the two buttons.
func newEULAContent(text string, onDecline, onAccept func()) fyne.CanvasObject {
	body := widget.NewRichTextFromMarkdown(text)
	body.Wrapping = fyne.TextWrapWord
	card := container.NewStack(newDialogBackground(), inset(container.NewVScroll(body), 4, 8, 4, 18))

	decline := newDialogButton(i18n.T("Decline"), buttonNeutral, onDecline)
	accept := newDialogButton(i18n.T("Accept"), buttonPrimary, onAccept)
	buttons := container.New(&rightRowLayout{gap: 8}, decline, accept)

	return container.NewBorder(nil, inset(buttons, 14, 24, 18, 24), nil, nil, inset(card, 12, 24, 0, 24))
}
