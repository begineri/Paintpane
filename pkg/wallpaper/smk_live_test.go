//go:build smklive

package wallpaper_test

// Live SMK download measurement (docs/smk-download.md). It downloads real images from SMK through
// the app's own download, retry and backoff code, the way the dispatcher feeds it: per-source
// pacing if the provider asks for it, jobs held while the source is paused, a fixed number of
// workers. Not part of `go test ./...`; run it with
//
//	SMKLIVE_N=100 SMKLIVE_OUT=run.csv go test -tags smklive -run TestSMKLive -v -timeout 2h ./pkg/wallpaper/
//
// Set LOCALAPPDATA and USERPROFILE to a scratch folder first.

import (
	"context"
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/smk"
)

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func TestSMKLive(t *testing.T) {
	n := envInt("SMKLIVE_N", 100)
	workers := envInt("SMKLIVE_WORKERS", wallpaper.DefaultMaxConcurrentProcessors)
	maxRun := time.Duration(envInt("SMKLIVE_MAX_MINUTES", 30)) * time.Minute
	label := os.Getenv("SMKLIVE_LABEL")

	p := smk.NewProvider(nil, wallpaper.AppHTTPClient())
	ctx, cancel := context.WithTimeout(context.Background(), maxRun)
	defer cancel()

	// Metadata, as FetchImages does it for each collection page.
	metaStart := time.Now()
	seen := map[string]bool{}
	var images []provider.Image
	col := curation.GetManager().GetCollection(p.ID())
	if col == nil {
		t.Fatal("no SMK collection")
	}
	for _, e := range col.Entries {
		for page := 1; ; page++ {
			batch, err := p.FetchImages(ctx, e.Key, page)
			if err != nil {
				t.Fatalf("FetchImages %s: %v", e.Key, err)
			}
			if len(batch) == 0 {
				break
			}
			for _, img := range batch {
				if !seen[img.ID] {
					seen[img.ID] = true
					images = append(images, img)
				}
			}
		}
	}
	t.Logf("metadata for %d images in %v", len(images), time.Since(metaStart).Round(time.Millisecond))
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(images), func(i, j int) { images[i], images[j] = images[j], images[i] })
	for len(images) < n && len(images) > 0 {
		images = append(images, images[:min(len(images), n-len(images))]...)
	}
	images = images[:n]

	var out *csv.Writer
	if path := os.Getenv("SMKLIVE_OUT"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		out = csv.NewWriter(f)
		defer out.Flush()
	}

	d := wallpaper.NewLiveDownloader()
	client := p.GetClient()
	var limiter *rate.Limiter
	if paced, ok := any(p).(provider.PacedProvider); ok && paced.GetProcessPacing() > 0 {
		limiter = rate.NewLimiter(rate.Every(paced.GetProcessPacing()), 1)
	}

	type result struct {
		ok    bool
		dur   time.Duration
		bytes int64
	}
	var mu sync.Mutex
	var results []result
	var held time.Duration
	jobs := make(chan provider.Image)
	dir := t.TempDir()
	start := time.Now()

	go func() { // the dispatcher's pump
		defer close(jobs)
		for _, img := range images {
			for {
				w := d.PausedFor(p.ID())
				if w <= 0 {
					break
				}
				w = min(w, time.Minute)
				held += w
				select {
				case <-time.After(w):
				case <-ctx.Done():
					return
				}
			}
			if limiter != nil && limiter.Wait(ctx) != nil {
				return
			}
			select {
			case jobs <- img:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for img := range jobs {
				path := filepath.Join(dir, fmt.Sprintf("w%d.jpg", w))
				begin := time.Now()
				_, err := d.Download(ctx, client, img.Path, path, p)
				dur := time.Since(begin)
				var size int64
				if st, statErr := os.Stat(path); err == nil && statErr == nil {
					size = st.Size()
				}
				_ = os.Remove(path)
				errText := ""
				if err != nil {
					errText = err.Error()
				}
				t.Logf("%s %-14s ok=%v %6.1fs %5.2f MB %s", time.Now().Format("15:04:05"), img.ID, err == nil, dur.Seconds(), float64(size)/1e6, errText)
				mu.Lock()
				results = append(results, result{err == nil, dur, size})
				if out != nil {
					_ = out.Write([]string{label, begin.Format(time.RFC3339), img.ID, strconv.FormatBool(err == nil),
						fmt.Sprintf("%.2f", dur.Seconds()), strconv.FormatInt(size, 10), errText, img.Path})
					out.Flush()
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	wall := time.Since(start)

	var ok, failed int
	var okTime time.Duration
	var bytes int64
	for _, r := range results {
		if r.ok {
			ok++
			okTime += r.dur
			bytes += r.bytes
		} else {
			failed++
		}
	}
	avg := time.Duration(0)
	if ok > 0 {
		avg = okTime / time.Duration(ok)
	}
	t.Logf("SUMMARY label=%q images=%d ok=%d failed=%d not_started=%d wall=%v avg_ok=%v MB=%.1f held=%v",
		label, n, ok, failed, n-ok-failed, wall.Round(time.Second), avg.Round(100*time.Millisecond), float64(bytes)/1e6, held.Round(time.Second))
}
