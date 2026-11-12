package favorites

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/api"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// The local API refuses browsers (docs/review-v1.0.md, S3). The Favorites source calls it with
// Go's HTTP client, on 127.0.0.1, and must keep working: it lists the favourites through the
// real server and the downloader can fetch each picture from the URL it is given.
func TestFetchImagesThroughTheRealLocalAPI(t *testing.T) {
	appDir := t.TempDir()
	favDir := filepath.Join(appDir, wallpaper.FavoritesCollection)
	require.NoError(t, os.MkdirAll(favDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(favDir, "Met_12345.jpg"), []byte("jpeg bytes"), 0o644))

	s := api.NewServer()
	s.RegisterNamespace(wallpaper.FavoritesNamespace, appDir) // as cmd/spice/main.go does
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	p := NewProvider(&wallpaper.Config{})
	p.SetTestConfig(strings.TrimPrefix(srv.URL, "http://"), favDir)
	defer p.Close()

	images, err := p.FetchImages(context.Background(), wallpaper.FavoritesQueryID, 1)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "Met_12345", strings.TrimSuffix(filepath.Base(images[0].Path), ".jpg"))

	resp, err := http.Get(images[0].Path) // what the downloader does with it
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "jpeg bytes", string(body))
}
