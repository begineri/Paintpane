package ui

import (
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Fyne wraps a label only at spaces (unicode.IsSpace, which includes U+00A0). That splits
// "12 MB" across two lines, and a Chinese sentence, which has spaces only around Latin letters
// and digits, breaks at whichever of those comes last ("（12 / MB）", A2 §15.3b #8). Dialog text
// is wrapped here instead:
//
//   - between Chinese or Japanese characters, as Chinese text is set;
//   - at a space, except a space right after a digit, so a number stays with its unit or noun
//     ("12 MB", "1.2 GB", "2 张", "312 downloaded");
//   - never before closing punctuation (）。，、) or after opening punctuation (（“).

// wrappedText is a label whose text is wrapped by wrapText to the width it is given.
type wrappedText struct {
	widget.Label
	source  string
	wrapped float32 // width the text is currently wrapped for
}

// newWrappedText makes a muted label wrapped for width (its text area, without padding) until it
// is laid out at another width.
func newWrappedText(text string, width float32) *wrappedText {
	t := &wrappedText{source: text}
	t.Importance = widget.LowImportance
	t.ExtendBaseWidget(t)
	t.wrapTo(width)
	return t
}

func (t *wrappedText) wrapTo(width float32) {
	if width <= 0 || width == t.wrapped {
		return
	}
	t.wrapped = width
	size := theme.TextSize()
	t.Label.Text = wrapText(t.source, width, func(s string) float32 {
		return fyne.MeasureText(s, size, fyne.TextStyle{}).Width
	})
}

// Resize wraps the text again for the new width.
func (t *wrappedText) Resize(s fyne.Size) {
	before := t.Label.Text
	t.wrapTo(s.Width - 2*theme.InnerPadding())
	t.Label.Resize(s)
	if t.Label.Text != before {
		t.Label.Refresh()
	}
}

// wrapText breaks text into lines no wider than width, as measure measures them, at the places
// described above. Existing line breaks are kept. A single piece wider than width is broken
// between characters.
func wrapText(text string, width float32, measure func(string) float32) string {
	paragraphs := strings.Split(text, "\n")
	for i, p := range paragraphs {
		paragraphs[i] = strings.Join(wrapParagraph([]rune(p), width, measure), "\n")
	}
	return strings.Join(paragraphs, "\n")
}

func wrapParagraph(text []rune, width float32, measure func(string) float32) []string {
	var lines []string
	start := 0
	for start < len(text) {
		lastBreak := -1 // a break is allowed before this index
		end := start
		for end < len(text) {
			if end > start && canBreakBefore(text, end) {
				lastBreak = end
			}
			if measure(strings.TrimRight(string(text[start:end+1]), " ")) > width && end > start {
				break
			}
			end++
		}
		if end == len(text) {
			lines = append(lines, strings.TrimRight(string(text[start:]), " "))
			break
		}
		cut := lastBreak
		if cut <= start {
			cut = end // nothing to break at: break between characters
		}
		lines = append(lines, strings.TrimRight(string(text[start:cut]), " "))
		start = cut
		for start < len(text) && text[start] == ' ' {
			start++
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// canBreakBefore reports whether a line may start at text[i].
func canBreakBefore(text []rune, i int) bool {
	prev, cur := text[i-1], text[i]
	switch {
	case cur == ' ':
		return false // break after the space, not before it
	case prev == ' ':
		return i < 2 || !unicode.IsDigit(text[i-2]) // "12 MB" stays together
	case isNoLineStart(cur) || isNoLineEnd(prev):
		return false
	}
	return isCJK(prev) || isCJK(cur)
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) ||
		(r >= 0x3000 && r <= 0x303F) || // CJK symbols and punctuation
		(r >= 0xFF00 && r <= 0xFFEF) // full-width forms
}

func isNoLineStart(r rune) bool {
	return strings.ContainsRune("）。，、！？：；」』》〉】”’…%)]}.,!?:;", r)
}

func isNoLineEnd(r rune) bool {
	return strings.ContainsRune("（「『《〈【“‘([{", r)
}
