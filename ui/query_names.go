package ui

import "github.com/dixieflatline76/Spice/v2/pkg/i18n"

// queryDisplayName returns the label to show for a saved query.
//
// The names of the default queries (asset/text/default_config.json and the Favorites query added by
// the config migrations) are stored in the user's settings in English, so they cannot be translated
// when they are written. Translate the known defaults when they are shown. A name the user typed is
// shown exactly as typed, even if it happens to match a translation key.
func queryDisplayName(desc string) string {
	switch desc {
	case "Random General":
		return i18n.T("Random General")
	case "Toplist last 3 months General":
		return i18n.T("Toplist last 3 months General")
	case "Wikimedia Featured":
		return i18n.T("Wikimedia Featured")
	case "Wikimedia Featured Astronomy":
		return i18n.T("Wikimedia Featured Astronomy")
	case "Favorite Images":
		return i18n.T("Favorite Images")
	}
	return desc
}
