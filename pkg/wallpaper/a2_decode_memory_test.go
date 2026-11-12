//go:build !linux

package wallpaper

import (
	"context"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// A2 phase 5: how much memory one large original costs while its derivatives are made, measured
// the way the app runs (default GC settings). Heavy, so it only runs with A2_MEM=1. The originals
// are made-up pictures of the sizes in A2_MEM_SIZES (default 16875x3750, the 63 MP Wikimedia
// panorama), or real downloaded files listed in A2_MEM_FILES (a made-up picture is flat and
// cheaper to process than a photograph).
func TestA2_MemoryOfOneLargeOriginal(t *testing.T) {
	if os.Getenv("A2_MEM") == "" {
		t.Skip("set A2_MEM=1 to measure")
	}
	// A2_MEM_SIZES="16875x3750,7133x5922" runs several originals at the same time, as the
	// download workers do.
	sizes := [][2]int{{16875, 3750}}
	if v := os.Getenv("A2_MEM_SIZES"); v != "" {
		sizes = nil
		for _, one := range strings.Split(v, ",") {
			var w, h int
			parts := strings.Split(one, "x")
			w, _ = strconv.Atoi(parts[0])
			h, _ = strconv.Atoi(parts[1])
			sizes = append(sizes, [2]int{w, h})
		}
	}
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 2560, 1600))

	var ids []string
	totalMP := 0.0
	if v := os.Getenv("A2_MEM_FILES"); v != "" {
		sizes = nil
		for i, src := range strings.Split(v, ",") {
			id := "real" + strconv.Itoa(i)
			ids = append(ids, id)
			cf, err := os.Open(src)
			require.NoError(t, err)
			cfg, _, err := image.DecodeConfig(cf)
			_ = cf.Close()
			require.NoError(t, err)
			sizes = append(sizes, [2]int{cfg.Width, cfg.Height})
			totalMP += float64(cfg.Width) * float64(cfg.Height) / 1e6
			path, err := app.fm.GetMasterPath(id, filepath.Ext(src))
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
			data, err := os.ReadFile(src)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, data, 0644))
		}
	}
	for i, sz := range sizes {
		if len(ids) == len(sizes) {
			break
		}
		id := "big" + strconv.Itoa(i)
		ids = append(ids, id)
		totalMP += float64(sz[0]) * float64(sz[1]) / 1e6
		path, err := app.fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, jpeg.Encode(f, quadrantImage(sz[0], sz[1]), &jpeg.Options{Quality: 85}))
		require.NoError(t, f.Close())
	}

	debug.FreeOSMemory()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	var peakAlloc, peakSys uint64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > peakAlloc {
				peakAlloc = m.HeapAlloc
				// A2_MEM_PROFILE=file keeps the heap profile taken at the highest point seen.
				if name := os.Getenv("A2_MEM_PROFILE"); name != "" && m.HeapAlloc > 300<<20 {
					if pf, err := os.Create(name); err == nil {
						_ = pprof.Lookup("heap").WriteTo(pf, 0)
						_ = pf.Close()
					}
				}
			}
			if m.HeapSys-m.HeapReleased > peakSys {
				peakSys = m.HeapSys - m.HeapReleased
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	start := time.Now()
	var jobs sync.WaitGroup
	errs := make([]error, len(ids))
	for i, id := range ids {
		jobs.Add(1)
		go func(i int, id string) {
			defer jobs.Done()
			_, errs[i] = app.wp.ProcessImageJob(context.Background(), DownloadJob{
				Ctx:   context.Background(),
				Image: provider.Image{ID: id, Path: "http://example.invalid/" + id + ".jpg", SourceQueryID: "q1"},
			})
		}(i, id)
	}
	jobs.Wait()
	took := time.Since(start)
	close(stop)
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	mb := func(b uint64) float64 { return float64(b) / (1 << 20) }
	t.Logf("%d originals together %.0f MP (%v): base heap %.0f MB; peak heap in use %.0f MB (+%.0f); peak heap from the OS %.0f MB (+%.0f); %.1f s",
		len(sizes), totalMP, sizes, mb(base.HeapAlloc), mb(peakAlloc), mb(peakAlloc)-mb(base.HeapAlloc),
		mb(peakSys), mb(peakSys)-mb(base.HeapSys-base.HeapReleased), took.Seconds())
}
