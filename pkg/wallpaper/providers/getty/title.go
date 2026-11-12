package getty

import (
	"regexp"
	"strings"
)

// accessionSuffixRe matches a Getty object number in parentheses at the end of
// a title, as the Getty's labels write it: "A Walk at Dusk (93.PA.14)".
// Two forms are used: a two-digit year, a two-letter department and a number
// ("93.PA.14", "84.XM.1004.1"), or a four-digit year and numbers ("2001.43",
// "2011.17.1"). An optional letter suffix ("85.GA.27a") is allowed. Other text
// in parentheses is kept.
var accessionSuffixRe = regexp.MustCompile(`\s*\(` + accessionNumber + `\)\s*$`)

// nestedAccessionRe matches an object number inside the last parentheses of a
// title, after other text: "Bifolium (Ms. 122 (2021.44))" (A2 phase 5 #3).
var nestedAccessionRe = regexp.MustCompile(`\s*\(` + accessionNumber + `\)\)\s*$`)

const accessionNumber = `(?:\d{2}\.[A-Z]{2}\.\d+(?:\.\d+)*|(?:1[89]|20)\d{2}(?:\.\d+)+)[a-z]?`

// stripAccessionNumber removes a trailing Getty object number from a title
// (A2 §12.9 #7), also one nested in the last parentheses.
func stripAccessionNumber(title string) string {
	title = nestedAccessionRe.ReplaceAllString(title, ")")
	stripped := strings.TrimSpace(accessionSuffixRe.ReplaceAllString(title, ""))
	if stripped == "" {
		return strings.TrimSpace(title)
	}
	return stripped
}

// objectNames reads the title and the first artist from a framed Linked Art
// object. A name the record does not give is left empty, so the tray shows
// its usual text for a missing title or artist instead of a placeholder.
func objectNames(obj map[string]interface{}) (title, artist string) {
	if label, ok := obj["_label"].(string); ok {
		title = stripAccessionNumber(label)
	}
	if prodBy, ok := obj["produced_by"].(map[string]interface{}); ok {
		if carried, ok := prodBy["carried_out_by"].([]interface{}); ok && len(carried) > 0 {
			if a, ok := carried[0].(map[string]interface{}); ok {
				if name, ok := a["_label"].(string); ok {
					artist = strings.TrimSpace(name)
				}
			}
		}
	}
	return title, artist
}

// attribution joins artist and title as "Artist - Title", leaving out a part
// that is empty.
func attribution(artist, title string) string {
	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case artist != "":
		return artist
	default:
		return title
	}
}
