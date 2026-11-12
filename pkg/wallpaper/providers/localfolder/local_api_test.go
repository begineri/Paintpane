package localfolder

import (
	"context"
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

// The local API refuses browsers (docs/review-v1.0.md, S3); the Local Folder source, which calls
// it with Go's HTTP client, still lists the folder and can fetch its pictures.
func TestFetchImagesThroughTheRealLocalAPI(t *testing.T) {
	folder := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(folder, "holiday.jpg"), []byte("jpeg bytes"), 0o644))

	s := api.NewServer()
	s.SetDynamicResolver(func(namespace, collectionID string) (string, bool) { // as cmd/spice/main.go does
		if namespace == wallpaper.LocalFolderNamespace && collectionID == wallpaper.HashFolderPath(folder) {
			return folder, true
		}
		return "", false
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	p := NewProvider(&wallpaper.Config{})
	p.SetTestConfig(strings.TrimPrefix(srv.URL, "http://"))

	images, err := p.FetchImages(context.Background(), folder, 1)
	require.NoError(t, err)
	require.Len(t, images, 1)

	resp, err := http.Get(images[0].Path)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
