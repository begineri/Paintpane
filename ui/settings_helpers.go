package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CreateSectionTitleLabel creates the group label shown above a card of
// settings: 13 px, bold, secondary colour (G2).
func (sm *SettingsManager) CreateSectionTitleLabel(desc string) fyne.CanvasObject {
	return newSectionLabel(desc)
}

// CreateSettingTitleLabel creates the primary label of a setting row.
func (sm *SettingsManager) CreateSettingTitleLabel(desc string) *widget.Label {
	return newRowLabel(desc)
}

// CreateSettingDescriptionLabel creates the secondary help text under a setting's label.
func (sm *SettingsManager) CreateSettingDescriptionLabel(desc string) fyne.CanvasObject {
	return newSecondaryText(desc)
}

// addRow adds one card row to a section. Several parts are stacked into a single
// row so the card draws no divider between, say, a control and its help text.
func addRow(section *fyne.Container, parts ...fyne.CanvasObject) fyne.CanvasObject {
	var kept []fyne.CanvasObject
	for _, p := range parts {
		switch p.(type) {
		case nil:
			continue
		case *widget.RichText, *widget.Label, *widget.Hyperlink, *accentLink:
			// Text starts at the card's text edge, like the setting rows (G3).
			p = noInset(p)
		default:
			p = sizedControl(p)
		}
		kept = append(kept, p)
	}
	var row fyne.CanvasObject
	switch len(kept) {
	case 0:
		return nil
	case 1:
		row = kept[0]
	default:
		row = vstack(helpGap, kept...)
	}
	section.Add(row)
	return row
}

// hasLabel reports whether a label object carries visible text.
func hasLabel(label fyne.CanvasObject) bool {
	if label == nil {
		return false
	}
	if l, ok := label.(*widget.Label); ok {
		return l.Text != ""
	}
	return true
}

// flatRowGap separates the lines of a flat section, such as a museum's
// location and its links.
const flatRowGap float32 = 6

// buildSection stacks a section's group label and description above a card
// holding its rows, 8 px apart (G4). A flat section shows its rows as plain
// text, without a card. The page column gives the section its margins.
func buildSection(header []fyne.CanvasObject, rows []fyne.CanvasObject, flat bool) fyne.CanvasObject {
	parts := []fyne.CanvasObject{}
	for _, h := range header {
		parts = append(parts, noInset(h))
	}
	if len(rows) > 0 {
		if flat {
			parts = append(parts, noInset(vstack(flatRowGap, rows...)))
		} else {
			parts = append(parts, newCard(rows...))
		}
	}
	return vstack(labelToCard, parts...)
}

// pageColumn lays out a page's sections in one column: 24 px from both sides
// (G3) and 24 px apart (G4).
func pageColumn(sections ...fyne.CanvasObject) *fyne.Container {
	return inset(vstack(sectionGap, sections...), 0, contentMarginX, sectionGap, contentMarginX)
}
