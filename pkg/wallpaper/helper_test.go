package wallpaper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// stateOf copies a monitor's state under mc.mu, the way the UI reads it, so a test can look
// at it while the actor goroutine is running.
func stateOf(mc *MonitorController) MonitorState {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return *mc.State
}

func TestExtractFilenameFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"http://example.com/image.jpg", "image.jpg"},
		{"http://example.com/path/to/image.png", "image.png"},
		{"image.gif", "image.gif"},
		{"http://example.com/", ""},
		{"", ""},
	}

	for _, tt := range tests {
		result := extractFilenameFromURL(tt.url)
		assert.Equal(t, tt.expected, result, "URL: %s", tt.url)
	}
}

func TestGenerateQueryID(t *testing.T) {
	url1 := "https://wallhaven.cc/search?q=anime"
	url2 := "https://wallhaven.cc/search?q=anime"
	url3 := "https://wallhaven.cc/search?q=landscape"

	id1 := GenerateQueryID(url1)
	id2 := GenerateQueryID(url2)
	id3 := GenerateQueryID(url3)

	assert.Equal(t, id1, id2)
	assert.NotEqual(t, id1, id3)
	assert.NotEmpty(t, id1)
}
