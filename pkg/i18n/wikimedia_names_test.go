package i18n

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A2 §12.9 #3: Chinese text names Wikimedia Commons 维基共享资源 / 維基共享資源
// (prototype v19); no Chinese value keeps the English word.
func TestChineseWikimediaNames(t *testing.T) {
	want := map[string]map[string]string{
		"zh": {
			"Wikimedia queries":        "维基共享资源查询",
			"Add Wikimedia collection": "添加维基共享资源收藏",
		},
		"zh-Hant": {
			"Wikimedia queries":        "維基共享資源查詢",
			"Add Wikimedia collection": "新增維基共享資源合集",
		},
	}
	for lang, keys := range want {
		data, err := os.ReadFile("translations/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		for key, value := range keys {
			if m[key] != value {
				t.Errorf("%s %q = %q, want %q", lang, key, m[key], value)
			}
		}
		for key, value := range m {
			if strings.Contains(value, "Wikimedia") {
				t.Errorf("%s %q still says Wikimedia: %q", lang, key, value)
			}
			// 维基媒体 is the foundation; the source is 维基共享资源.
			if (strings.Contains(value, "维基媒体") || strings.Contains(value, "維基媒體")) && key != "Donate to Wikimedia" {
				t.Errorf("%s %q names the source 维基媒体: %q", lang, key, value)
			}
		}
	}
}
