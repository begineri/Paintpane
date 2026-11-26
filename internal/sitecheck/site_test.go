package sitecheck

import (
	"encoding/json"
	"encoding/xml"
	"html"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const siteURL = "https://begineri.github.io/Paintpane/"

var root = filepath.Join("..", "..")

// pages are the intro pages with structured data, by language.
var pages = []struct{ file, lang, url string }{
	{"site/index.html", "en", siteURL},
	{"site/zh/index.html", "zh-CN", siteURL + "zh/"},
}

// allowed lists, per schema.org type, the properties the pages may use. All of them are defined by
// schema.org for that type (directly or through CreativeWork / Thing); a typo or an invented property
// fails here instead of being silently ignored by search engines.
var allowed = map[string][]string{
	"SoftwareApplication": {"name", "alternateName", "description", "url", "inLanguage", "image", "screenshot",
		"applicationCategory", "applicationSubCategory", "operatingSystem", "softwareVersion", "downloadUrl",
		"isAccessibleForFree", "offers", "license", "isBasedOn", "sameAs"},
	"Offer":    {"price", "priceCurrency"},
	"FAQPage":  {"inLanguage", "mainEntity"},
	"Question": {"name", "acceptedAnswer"},
	"Answer":   {"text"},
}

// read returns a file of the repository with LF line endings.
func read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

var (
	ldRe   = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	tagRe  = regexp.MustCompile(`<[^>]+>`)
	leadRe = regexp.MustCompile(`(?s)<p class="lead">(.*?)</p>`)
	faqRe  = regexp.MustCompile(`(?s)<div class="faq">(.*?)\n  </div>\n</div></section>`)
	qaRe   = regexp.MustCompile(`(?s)<h3>(.*?)</h3>\s*<p>(.*?)</p>`)
	metaRe = regexp.MustCompile(`<meta (?:property|name)="([^"]+)" content="([^"]*)">`)
	canRe  = regexp.MustCompile(`<link rel="canonical" href="([^"]+)">`)
	urlRe  = regexp.MustCompile(`https://[^\s)]+`)
)

// visible is the text a reader sees for an HTML fragment.
func visible(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(s, "")))
}

// jsonLD parses every JSON-LD block of a page and returns them by @type.
func jsonLD(t *testing.T, src string) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, m := range ldRe.FindAllStringSubmatch(src, -1) {
		var v map[string]any
		if err := json.Unmarshal([]byte(m[1]), &v); err != nil {
			t.Fatalf("JSON-LD does not parse: %v", err)
		}
		if v["@context"] != "https://schema.org" {
			t.Errorf("@context = %v, want https://schema.org", v["@context"])
		}
		checkProps(t, v)
		typ, _ := v["@type"].(string)
		out[typ] = v
	}
	return out
}

func checkProps(t *testing.T, v any) {
	t.Helper()
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			checkProps(t, e)
		}
	case map[string]any:
		typ, _ := x["@type"].(string)
		props, ok := allowed[typ]
		if !ok {
			t.Errorf("unexpected @type %q", typ)
			return
		}
		for k, e := range x {
			if k == "@type" || k == "@context" {
				continue
			}
			if !contains(props, k) {
				t.Errorf("%s has property %q, which is not in the checked list", typ, k)
			}
			checkProps(t, e)
		}
	}
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// squash drops all white space, so line breaks and &nbsp; in the HTML do not matter.
func squash(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, " ", " ")), "")
}

func TestStructuredDataMatchesPage(t *testing.T) {
	version := strings.TrimPrefix(strings.TrimSpace(read(t, "version.txt")), "v")
	for _, p := range pages {
		t.Run(p.lang, func(t *testing.T) {
			src := read(t, p.file)
			ld := jsonLD(t, src)

			app, ok := ld["SoftwareApplication"]
			if !ok {
				t.Fatal("no SoftwareApplication")
			}
			if app["softwareVersion"] != version {
				t.Errorf("softwareVersion = %v, version.txt says %s", app["softwareVersion"], version)
			}
			if app["url"] != p.url {
				t.Errorf("url = %v, want %s", app["url"], p.url)
			}
			if dl, _ := app["downloadUrl"].(string); !strings.Contains(src, `href="`+dl+`"`) {
				t.Errorf("downloadUrl %q is not a link on the page", dl)
			}
			desc, _ := app["description"].(string)
			if lead := leadRe.FindStringSubmatch(src); lead == nil || squash(visible(lead[1])) != squash(desc) {
				t.Errorf("description %q is not the first-screen description", desc)
			}

			faq, ok := ld["FAQPage"]
			if !ok {
				t.Fatal("no FAQPage")
			}
			if faq["inLanguage"] != p.lang {
				t.Errorf("FAQPage inLanguage = %v, want %s", faq["inLanguage"], p.lang)
			}
			block := faqRe.FindStringSubmatch(src)
			if block == nil {
				t.Fatal("no FAQ block on the page")
			}
			shown := qaRe.FindAllStringSubmatch(block[1], -1)
			entities, _ := faq["mainEntity"].([]any)
			if len(entities) != len(shown) {
				t.Fatalf("FAQPage has %d questions, the page shows %d", len(entities), len(shown))
			}
			for i, e := range entities {
				q, _ := e.(map[string]any)
				a, _ := q["acceptedAnswer"].(map[string]any)
				if q["name"] != visible(shown[i][1]) {
					t.Errorf("question %d: JSON-LD %q, page %q", i+1, q["name"], visible(shown[i][1]))
				}
				if a["text"] != visible(shown[i][2]) {
					t.Errorf("answer %d: JSON-LD %q, page %q", i+1, a["text"], visible(shown[i][2]))
				}
			}
		})
	}
}

func TestShareCards(t *testing.T) {
	for _, p := range pages {
		t.Run(p.lang, func(t *testing.T) {
			src := read(t, p.file)
			meta := map[string]string{}
			for _, m := range metaRe.FindAllStringSubmatch(src, -1) {
				meta[m[1]] = m[2]
			}
			for _, k := range []string{"og:title", "og:description", "og:image", "og:url", "og:locale",
				"twitter:card", "twitter:title", "twitter:description", "twitter:image"} {
				if meta[k] == "" {
					t.Errorf("missing %s", k)
				}
			}
			if can := canRe.FindStringSubmatch(src); can == nil || can[1] != meta["og:url"] {
				t.Errorf("og:url %q is not the canonical address", meta["og:url"])
			}
			if meta["twitter:card"] != "summary_large_image" || meta["twitter:image"] != meta["og:image"] {
				t.Errorf("twitter card %q / image %q", meta["twitter:card"], meta["twitter:image"])
			}
			rel, ok := strings.CutPrefix(meta["og:image"], siteURL)
			if !ok {
				t.Fatalf("og:image %q is not on the site", meta["og:image"])
			}
			f, err := os.Open(filepath.Join(root, "site", filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			cfg, _, err := image.DecodeConfig(f)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width != 1200 || cfg.Height != 630 || meta["og:image:width"] != "1200" || meta["og:image:height"] != "630" {
				t.Errorf("share card is %d × %d (meta %s × %s), want 1200 × 630",
					cfg.Width, cfg.Height, meta["og:image:width"], meta["og:image:height"])
			}
		})
	}
}

// sitePath maps an address on the site to its file under site/ (slash-separated, relative to the
// repository), or "" for other addresses.
func sitePath(u string) string {
	rel, ok := strings.CutPrefix(u, siteURL)
	if !ok {
		return ""
	}
	if rel == "" || strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	return "site/" + rel
}

func TestSitemap(t *testing.T) {
	var sm struct {
		URLs []struct {
			Loc   string `xml:"loc"`
			Links []struct {
				Hreflang string `xml:"hreflang,attr"`
				Href     string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(read(t, "site/sitemap.xml")), &sm); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, u := range sm.URLs {
		listed[u.Loc] = true
		rel := sitePath(u.Loc)
		if rel == "" {
			t.Errorf("sitemap lists %s, which is not on the site", u.Loc)
			continue
		}
		src := read(t, rel)
		// The hreflang links must be the ones the page itself declares.
		for _, l := range u.Links {
			if !strings.Contains(src, `<link rel="alternate" hreflang="`+l.Hreflang+`" href="`+l.Href+`">`) {
				t.Errorf("%s: sitemap has hreflang %s → %s, the page does not", u.Loc, l.Hreflang, l.Href)
			}
		}
	}
	for _, p := range pages {
		if !listed[p.url] {
			t.Errorf("sitemap does not list %s", p.url)
		}
	}
}

func TestLLMsTxt(t *testing.T) {
	src := read(t, "site/llms.txt")
	if !strings.HasPrefix(src, "# Paintpane\n") {
		t.Error("llms.txt must start with the H1 # Paintpane")
	}
	for _, u := range urlRe.FindAllString(src, -1) {
		if rel := sitePath(u); rel != "" {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
				t.Errorf("llms.txt links %s, which is not a page: %v", u, err)
			}
		}
	}
}
