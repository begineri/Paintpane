//go:build pagelive

package wallpaper_test

// Live check of the page-reset fix: three starts in a row against real sources, each in a new
// plugin that reads what the start before saved. Each start should fetch the next page and queue
// images the library does not have. Only listings are requested; no image is downloaded.
//
//	LOCALAPPDATA=<scratch> USERPROFILE=<scratch> go test -tags pagelive -run TestPageResetLive -v ./pkg/wallpaper/
//
// PAGELIVE_OLD=1 starts every query at page 1, as the app did before, for comparison.

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/getty"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/metmuseum"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/smk"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wikimedia"
)

func TestPageResetLive(t *testing.T) {
	require.NotEmpty(t, os.Getenv("LOCALAPPDATA"))
	require.True(t, strings.Contains(strings.ToLower(config.GetWorkingDir()), "scratch") || os.Getenv("PAGELIVE_OK") != "",
		"set LOCALAPPDATA to a scratch folder (path containing \"scratch\")")
	cfg := wallpaper.GetConfig(fynetest.NewTempApp(t).Preferences())
	client := &http.Client{Timeout: 60 * time.Second}
	sources := []provider.ImageProvider{
		getty.NewProvider(cfg, client), smk.NewProvider(cfg, client),
		metmuseum.NewProvider(cfg, client), wikimedia.NewProvider(cfg, client),
	}
	queries := []wallpaper.LiveQuery{
		{ID: "getty", Provider: sources[0].ID(), URL: "getty_masterpieces"},
		{ID: "smk", Provider: sources[1].ID(), URL: "smk_highlights"},
		{ID: "met", Provider: sources[2].ID(), URL: "metmuseum_best_of"},
		{ID: "wikimedia", Provider: sources[3].ID(), URL: "category:Featured_pictures_on_Wikimedia_Commons"},
	}
	for start := 1; start <= 3; start++ {
		// A new provider per start too: nothing carries over in memory, as after a restart.
		if start > 1 {
			sources = []provider.ImageProvider{
				getty.NewProvider(cfg, client), smk.NewProvider(cfg, client),
				metmuseum.NewProvider(cfg, client), wikimedia.NewProvider(cfg, client),
			}
		}
		res := wallpaper.LiveStart(t, sources, queries)
		ids := make([]string, 0, len(res))
		for id := range res {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			r := res[id]
			fmt.Printf("start %d  %-10s page %d  new images %3d\n", start, id, r.Page, r.New)
		}
	}
}
