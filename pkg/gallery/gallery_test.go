package gallery

import (
	"strings"
	"testing"
)

func TestGenerateHTMLWithTranslations(t *testing.T) {
	items := []GalleryItem{
		{ID: "1", URL: "http://example.com/1.jpg"},
	}

	titleTranslations := map[string]string{
		"de": "Das Beste aus Cleveland",
		"fr": "Le Meilleur de Cleveland",
	}

	html, err := GenerateHTML("Best of Cleveland", titleTranslations, items, 1.5, true)
	if err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}

	if !strings.Contains(html, `const translations = {`) {
		t.Error("Expected generated HTML to contain Javascript translations dictionary")
	}

	if !strings.Contains(html, `"Das Beste aus Cleveland"`) {
		t.Error("Expected generated HTML to contain German translated title in dictionary")
	}

	if !strings.Contains(html, `data-i18n="Best of Cleveland"`) {
		t.Error("Expected title tag to have data-i18n attribute")
	}

	if !strings.Contains(html, `data-i18n="Back to Index"`) {
		t.Error("Expected Back to Index link to have data-i18n attribute")
	}
}

// A work's title in other languages is keyed by the work, so the page's script swaps the label.
func TestGenerateHTMLArtworkTitleTranslations(t *testing.T) {
	items := []GalleryItem{
		{ID: "a", URL: "http://example.com/a.jpg", Title: "Portrait of a Woman",
			TitleTranslations: map[string]string{"zh": "女子肖像", "fr": "Portrait de femme"}},
		{ID: "b", URL: "http://example.com/b.jpg", Title: "Portrait of a Woman",
			TitleTranslations: map[string]string{"fr": "Portrait d'une dame"}},
		{ID: "c", URL: "http://example.com/c.jpg", Title: "Landscape"},
	}
	html, err := GenerateHTML("Portraits", nil, items, 1.5, false)
	if err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}
	for _, want := range []string{
		`<div class="title" data-i18n="artwork:a">Portrait of a Woman</div>`,
		`<div class="title" data-i18n="artwork:b">Portrait of a Woman</div>`,
		`<div class="title">Landscape</div>`,
		`"artwork:a":"女子肖像"`,
		`"artwork:a":"Portrait de femme"`,
		`"artwork:b":"Portrait d'une dame"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("generated HTML lacks %s", want)
		}
	}
}
