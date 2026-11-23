//go:build !darwin

package wallpaper

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	pigo "github.com/esimov/pigo/core"
)

// TestEvalFaceDownscale compares face detection on the full image with detection on reduced
// copies, for every image in testdata/golden, testdata/tuning_images and testdata/face.png:
// whether a face is found, how far its centre moves, how the 1920x1080 crop changes, and the time
// and peak Go heap each run takes. It only reports; run it with
//
//	EVAL_FACE=1 [EVAL_FACE_EDGES=960,1280,1600] [EVAL_FACE_CAP=1] go test -run TestEvalFaceDownscale -v ./pkg/wallpaper/
//
// EVAL_FACE_CAP=1 first shrinks each image to the 4K cap originals are stored at (what the app
// sees); EVAL_FACE_DUMP=<folder> saves both crops and both faces of every image that changes;
// EVAL_FACE_ONLY=a,b limits the run to file names containing a or b. Results of 2026-11-19 are in
// docs/roadmap.md (L3).
func TestEvalFaceDownscale(t *testing.T) {
	if os.Getenv("EVAL_FACE") != "1" {
		t.Skip("set EVAL_FACE=1 to compare face detection on full and reduced images")
	}
	edges := []int{960, 1280, 1600}
	if s := os.Getenv("EVAL_FACE_EDGES"); s != "" {
		edges = nil
		for _, f := range strings.Split(s, ",") {
			n, err := strconv.Atoi(f)
			if err != nil {
				t.Fatal(err)
			}
			edges = append(edges, n)
		}
	}
	capped := os.Getenv("EVAL_FACE_CAP") == "1"
	dump := os.Getenv("EVAL_FACE_DUMP") // folder: crops of disputed images go there
	only := os.Getenv("EVAL_FACE_ONLY") // comma-separated name substrings

	modelData, err := asset.NewManager().GetModel("facefinder")
	if err != nil {
		t.Fatal(err)
	}
	pg, err := pigo.NewPigo().Unpack(modelData)
	if err != nil {
		t.Fatal(err)
	}
	ResetConfig()
	cfg := GetConfig(NewMockPreferences())
	cfg.SetSmartFitMode(SmartFitAggressive)
	cfg.SetFaceCropEnabled(true)
	cfg.VirtualPaperMatting = false
	mockOS := new(MockOS)
	mockOS.On("GetDesktopDimension").Return(1920, 1080, nil)

	var files []string
	for _, dir := range []string{"testdata/golden", "testdata/tuning_images"} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if ext := strings.ToLower(filepath.Ext(e.Name())); ext == ".jpg" || ext == ".png" {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	files = append(files, "testdata/face.png")
	sort.Strings(files)

	type run struct {
		found bool
		rect  image.Rectangle
		dur   time.Duration
		peak  uint64
		out   image.Image
	}
	measure := func(img image.Image, edge int) run {
		p := NewSmartImageProcessor(mockOS, cfg, pg)
		p.faceLongEdge = edge
		r := run{}
		r.dur, r.peak = heapPeak(func() {
			rect, err := p.findBestFace(img)
			r.found, r.rect = err == nil, rect
		})
		out, err := p.FitImage(context.Background(), img, 1920, 1080, provider.TuningOptions{})
		if err != nil {
			t.Fatal(err)
		}
		r.out = imaging.Resize(out, 384, 216, imaging.Box)
		return r
	}

	type agg struct {
		dur        time.Duration
		peak       uint64
		agree, n   int
		maxShift   float64
		maxDiff    float64
		changed    []string
		speedupMin float64
	}
	sums := map[int]*agg{}
	var full agg
	for _, e := range edges {
		sums[e] = &agg{speedupMin: math.Inf(1)}
	}

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if capped {
			if w, h := cappedSize(img.Bounds().Dx(), img.Bounds().Dy()); w != img.Bounds().Dx() {
				img = imaging.Resize(img, w, h, imaging.Lanczos)
			}
		}
		name := filepath.Base(path)
		if only != "" {
			keep := false
			for _, o := range strings.Split(only, ",") {
				keep = keep || strings.Contains(name, o)
			}
			if !keep {
				continue
			}
		}
		ref := measure(img, -1)
		full.dur += ref.dur
		full.peak = max(full.peak, ref.peak)
		minDim := float64(min(img.Bounds().Dx(), img.Bounds().Dy()))
		line := fmt.Sprintf("%-40s %5dx%-5d full: %-5v %6.2fs %4dMB", name, img.Bounds().Dx(), img.Bounds().Dy(),
			ref.found, ref.dur.Seconds(), ref.peak>>20)
		for _, e := range edges {
			r := measure(img, e)
			a := sums[e]
			a.n++
			a.dur += r.dur
			a.peak = max(a.peak, r.peak)
			a.speedupMin = math.Min(a.speedupMin, ref.dur.Seconds()/r.dur.Seconds())
			shift := 0.0
			if r.found == ref.found {
				a.agree++
				if r.found {
					c1 := r.rect.Min.Add(r.rect.Max).Div(2)
					c0 := ref.rect.Min.Add(ref.rect.Max).Div(2)
					shift = math.Hypot(float64(c1.X-c0.X), float64(c1.Y-c0.Y)) / minDim * 100
					a.maxShift = math.Max(a.maxShift, shift)
				}
			}
			diff := meanAbsDiff(ref.out, r.out)
			a.maxDiff = math.Max(a.maxDiff, diff)
			if dump != "" && (r.found != ref.found || shift > 2 || diff > 2) {
				save := func(tag string, im image.Image) {
					_ = imaging.Save(im, filepath.Join(dump, fmt.Sprintf("%s_%s.jpg", strings.TrimSuffix(name, filepath.Ext(name)), tag)))
				}
				save("fit_full", ref.out)
				save(fmt.Sprintf("fit_%d", e), r.out)
				for tag, rc := range map[string]image.Rectangle{"face_full": ref.rect, fmt.Sprintf("face_%d", e): r.rect} {
					if rc.Empty() {
						continue
					}
					pad := rc.Dx()
					save(tag, imaging.Fit(imaging.Crop(img, image.Rect(rc.Min.X-pad, rc.Min.Y-pad, rc.Max.X+pad, rc.Max.Y+pad)), 300, 300, imaging.Linear))
				}
				save("whole", imaging.Fit(img, 400, 400, imaging.Linear))
			}
			if r.found != ref.found || shift > 2 || diff > 2 {
				a.changed = append(a.changed, fmt.Sprintf("%s (found %v→%v, %v→%v, centre moved %.1f%%, crop diff %.1f)",
					name, ref.found, r.found, ref.rect, r.rect, shift, diff))
			}
			line += fmt.Sprintf(" | %d: %-5v %5.2fs %3dMB Δc%.1f%% Δimg%.1f", e, r.found, r.dur.Seconds(), r.peak>>20, shift, diff)
		}
		t.Log(line)
	}
	t.Logf("full: total %.1fs, peak %d MB", full.dur.Seconds(), full.peak>>20)
	for _, e := range edges {
		a := sums[e]
		t.Logf("%d: total %.1fs (%.0f%% of full, worst image %.1fx faster), peak %d MB; found agrees %d/%d; max centre shift %.1f%% of short edge; max crop diff %.1f/255",
			e, a.dur.Seconds(), 100*a.dur.Seconds()/full.dur.Seconds(), a.speedupMin, a.peak>>20, a.agree, a.n, a.maxShift, a.maxDiff)
		for _, c := range a.changed {
			t.Logf("   %d changed: %s", e, c)
		}
	}
}

// heapPeak runs f and returns how long it took and the most the Go heap grew while it ran,
// sampled every millisecond.
func heapPeak(f func()) (time.Duration, uint64) {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	var peak atomic.Uint64
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		var m runtime.MemStats
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > base && m.HeapAlloc-base > peak.Load() {
				peak.Store(m.HeapAlloc - base)
			}
			select {
			case <-done:
				return
			case <-tick.C:
			}
		}
	}()
	start := time.Now()
	f()
	d := time.Since(start)
	close(done)
	<-stopped
	return d, peak.Load()
}

// meanAbsDiff is the mean absolute difference of two same-size images' RGB values, 0-255.
func meanAbsDiff(a, b image.Image) float64 {
	na, nb := imaging.Clone(a), imaging.Clone(b)
	var sum float64
	for i := range na.Pix {
		if i%4 == 3 {
			continue
		}
		sum += math.Abs(float64(na.Pix[i]) - float64(nb.Pix[i]))
	}
	return sum / float64(len(na.Pix)/4*3)
}
