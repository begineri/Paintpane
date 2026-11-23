package ui

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"github.com/go-text/typesetting/font"

	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// Narrow punctuation (docs/ui_refresh.md, G1a).
//
// Noto Sans SC draws a few punctuation marks a full em wide, centred, as Chinese typesetting
// wants: the middle dot ·, the curly quotes ‘ ’ “ ”, the ellipsis …, × and •. In English
// "Delete “Wikimedia”?" then looks spaced out, and "Wikimedia · Ausgewählt" gets a wide gap on
// each side of the dot. The font has narrow forms of the quotes (its 'pwid' feature), but Fyne
// shapes text without a language or features, so they are never used, and it has no narrow
// middle dot at all.
//
// So the theme hands Fyne a copy of the font whose character map leaves these characters out,
// and Fyne draws them in its fallback font, Noto Sans (Latin, Regular or Bold to match), with
// the narrow widths Latin text expects. Which characters depends on the UI language:
//   - the middle dot in every language: it separates parts of a name (" · ") in all of them,
//     and many Chinese UI fonts draw it narrow in names (达·芬奇) too;
//   - the quotes, the ellipsis, × and • only outside Chinese and Japanese, whose typesetting
//     wants them full width (删除“{{.Name}}”？, 正在计算…).

// narrowAlways are drawn narrow in every UI language.
const narrowAlways = "·" // ·

// narrowOutsideCJK are drawn narrow except in Chinese and Japanese.
const narrowOutsideCJK = "‘’“”…×•" // ‘ ’ “ ” … × •

// narrowPunctuation returns the characters drawn in the Latin font for a UI language code.
func narrowPunctuation(lang string) string {
	if lang == "ja" || lang == "zh" || strings.HasPrefix(lang, "zh-") {
		return narrowAlways
	}
	return narrowAlways + narrowOutsideCJK
}

// textFonts are the Regular and Bold fonts for a set of narrow characters (narrowPunctuation).
type textFonts struct{ regular, bold fyne.Resource }

var (
	textFontsMu    sync.Mutex
	textFontsCache = map[string]textFonts{}
)

// textFontsFor returns the theme's fonts with the characters in narrow left out, built once per
// set. Should a font not be rewritable, the original is used: the text is then only wider.
func textFontsFor(narrow string) textFonts {
	textFontsMu.Lock()
	defer textFontsMu.Unlock()
	if f, ok := textFontsCache[narrow]; ok {
		return f
	}
	f := textFonts{
		regular: fontWithout(fontRegular, narrow),
		bold:    fontWithout(fontBold, narrow),
	}
	textFontsCache[narrow] = f
	return f
}

func fontWithout(res fyne.Resource, chars string) fyne.Resource {
	if chars == "" {
		return res
	}
	data, err := dropFromCmap(res.Content(), []rune(chars))
	if err != nil {
		utilLog.Printf("font: keeping %s as it is: %v", res.Name(), err)
		return res
	}
	return fyne.NewStaticResource(strings.TrimSuffix(res.Name(), ".otf")+".narrow.otf", data)
}

// dropFromCmap returns a copy of an OpenType font whose character map leaves out chars. The
// copy gets a new 'cmap' table, appended at the end, with one format 12 subtable (Windows,
// Unicode full repertoire) holding every other mapping of the font's own best subtable. The
// other tables are not touched; the font's whole-file checksum (head.checkSumAdjustment) is
// left stale, which font loaders, go-text among them, do not check.
func dropFromCmap(data []byte, chars []rune) ([]byte, error) {
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	drop := make(map[rune]bool, len(chars))
	for _, r := range chars {
		drop[r] = true
	}
	type mapping struct {
		r rune
		g uint32
	}
	var maps []mapping
	for it := face.Cmap.Iter(); it.Next(); {
		r, g := it.Char()
		if g != 0 && !drop[r] {
			maps = append(maps, mapping{r, uint32(g)})
		}
	}
	if len(maps) == 0 {
		return nil, errors.New("empty character map")
	}
	sort.Slice(maps, func(i, j int) bool { return maps[i].r < maps[j].r })

	// Groups of consecutive characters mapped to consecutive glyphs.
	type group struct{ start, end, glyph uint32 }
	var groups []group
	for _, m := range maps {
		if n := len(groups); n > 0 {
			last := &groups[n-1]
			if uint32(m.r) == last.end+1 && m.g == last.glyph+last.end+1-last.start {
				last.end++
				continue
			}
			if uint32(m.r) == last.end { // the same character twice: keep the first
				continue
			}
		}
		groups = append(groups, group{uint32(m.r), uint32(m.r), m.g})
	}

	be := binary.BigEndian
	subLen := 16 + 12*len(groups)
	cmap := make([]byte, 12+subLen)
	be.PutUint16(cmap[0:], 0)  // version
	be.PutUint16(cmap[2:], 1)  // one subtable
	be.PutUint16(cmap[4:], 3)  // platform: Windows
	be.PutUint16(cmap[6:], 10) // encoding: Unicode full repertoire
	be.PutUint32(cmap[8:], 12) // subtable offset
	sub := cmap[12:]
	be.PutUint16(sub[0:], 12) // format
	be.PutUint32(sub[4:], uint32(subLen))
	be.PutUint32(sub[12:], uint32(len(groups)))
	for i, g := range groups {
		o := 16 + 12*i
		be.PutUint32(sub[o:], g.start)
		be.PutUint32(sub[o+4:], g.end)
		be.PutUint32(sub[o+8:], g.glyph)
	}

	// Find the cmap record in the table directory and point it at the new table.
	if len(data) < 12 {
		return nil, errors.New("font too short")
	}
	numTables := int(be.Uint16(data[4:]))
	if len(data) < 12+16*numTables {
		return nil, errors.New("table directory cut short")
	}
	rec := -1
	for i := 0; i < numTables; i++ {
		if string(data[12+16*i:12+16*i+4]) == "cmap" {
			rec = 12 + 16*i
		}
	}
	if rec < 0 {
		return nil, errors.New("no cmap table")
	}
	offset := (len(data) + 3) &^ 3
	out := make([]byte, offset, offset+len(cmap)+3)
	copy(out, data)
	out = append(out, cmap...)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	be.PutUint32(out[rec+4:], tableChecksum(cmap))
	be.PutUint32(out[rec+8:], uint32(offset))
	be.PutUint32(out[rec+12:], uint32(len(cmap)))
	return out, nil
}

// tableChecksum is the OpenType checksum of a table: the sum of its big-endian 32-bit words,
// the last one padded with zeros.
func tableChecksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}
