package wallpaper

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// A2 §12.9 #8: masters decoded at the same time are limited by their total
// size, so large originals take turns and ordinary ones still run together.

func acquired(t *testing.T, g *decodeGate, n int64) <-chan int64 {
	t.Helper()
	done := make(chan int64, 1)
	go func() {
		got, err := g.acquire(context.Background(), n)
		if err == nil {
			done <- got
		}
	}()
	return done
}

func waitFor(t *testing.T, ch <-chan int64, what string) int64 {
	t.Helper()
	select {
	case n := <-ch:
		return n
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not get its turn", what)
		return 0
	}
}

func notYet(t *testing.T, ch <-chan int64, what string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("%s went ahead too early", what)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestDecodeGate_LargeOriginalsTakeTurns(t *testing.T) {
	g := newDecodeGate(40)
	first, err := g.acquire(context.Background(), 63)
	require.NoError(t, err)
	assert.Equal(t, int64(40), first, "a request larger than the budget takes all of it")

	second := acquired(t, g, 63)
	notYet(t, second, "a second large original")
	small := acquired(t, g, 5)
	notYet(t, small, "a small one queued behind a large one") // first come, first served

	g.release(first)
	n := waitFor(t, second, "the second large original")
	notYet(t, small, "the small one while the large one runs")
	g.release(n)
	g.release(waitFor(t, small, "the small one"))
	assert.Zero(t, g.used)
}

func TestDecodeGate_OrdinaryImagesRunTogether(t *testing.T) {
	g := newDecodeGate(40)
	for i := 0; i < 3; i++ {
		_, err := g.acquire(context.Background(), 12)
		require.NoError(t, err, "three 12 MP images fit at once")
	}
	fourth := acquired(t, g, 12)
	notYet(t, fourth, "a fourth 12 MP image")
	g.release(12)
	waitFor(t, fourth, "the fourth image")
}

func TestDecodeGate_CancelledWaitGivesWayAndLeaksNothing(t *testing.T) {
	g := newDecodeGate(40)
	_, err := g.acquire(context.Background(), 30)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		_, err := g.acquire(ctx, 40)
		errs <- err
	}()
	time.Sleep(20 * time.Millisecond)
	behind := acquired(t, g, 10)
	notYet(t, behind, "a request behind a waiting large one")

	cancel()
	require.ErrorIs(t, <-errs, context.Canceled)
	waitFor(t, behind, "the request behind the cancelled one")
	g.release(10)
	g.release(30)
	assert.Zero(t, g.used)
	assert.Empty(t, g.waiters)
}

// A2 phase 5 #1: the gate weighs an original by the memory it will take, read from the JPEG frame
// header; a progressive JPEG costs several times a sequential one of the same size.
func TestReadJPEGInfo(t *testing.T) {
	cases := []struct {
		file        string
		progressive bool
		samples     float64
	}{
		{"video-001.q50.420.jpeg", false, 1.5},
		{"video-001.q50.420.progressive.jpeg", true, 1.5},
		{"video-001.q50.444.jpeg", false, 3},
		{"video-001.q50.444.progressive.jpeg", true, 3},
		{"video-005.gray.q50.jpeg", false, 1},
		{"video-005.gray.q50.progressive.jpeg", true, 1},
	}
	for _, c := range cases {
		f, err := os.Open(filepath.Join("testdata", "jpeg", c.file))
		require.NoError(t, err)
		info, err := readJPEGInfo(f)
		f.Close()
		require.NoError(t, err, c.file)
		assert.Equal(t, c.progressive, info.progressive, c.file)
		assert.InDelta(t, c.samples, info.samples, 0.001, c.file)
		assert.Positive(t, info.width, c.file)
		assert.Positive(t, info.height, c.file)
		assert.Equal(t, c.progressive, isProgressiveJPEG(filepath.Join("testdata", "jpeg", c.file)), c.file)
	}

	_, err := readJPEGInfo(bytes.NewReader([]byte("\x89PNG\r\n")))
	assert.ErrorIs(t, err, errNotJPEG)
}

// The estimates match what real originals measured (docs/test-report-v1.0.md §13.5).
func TestDecodeCostMatchesMeasurements(t *testing.T) {
	getty420 := jpegInfo{width: 9021, height: 7122, samples: 1.5}                   // measured 2.6-2.9 MB/MP
	commons444 := jpegInfo{width: 7616, height: 5504, samples: 3}                   // 5.0-5.7
	panorama := jpegInfo{width: 16875, height: 3750, samples: 3, progressive: true} // 16.0 with image/jpeg, half the coefficients with progjpeg
	assert.InDelta(t, 3.25, getty420.costMBPerMP(), 0.01)
	assert.InDelta(t, 5.5, commons444.costMBPerMP(), 0.01)
	assert.InDelta(t, 1+4.5+3*costProgressivePerSamp, panorama.costMBPerMP(), 0.01)

	// A 43 MP progressive original takes the gate alone; when the gate counted megapixels it
	// let a 13 MP one in beside a 27 MP progressive one (27 + 13 <= 40).
	prog43 := roundUpMB(42.8 * jpegInfo{samples: 3, progressive: true}.costMBPerMP())
	small13 := roundUpMB(12.2 * jpegInfo{samples: 1.5}.costMBPerMP())
	g := newDecodeGate(decodeBudgetMB)
	_, err := g.acquire(context.Background(), prog43)
	require.NoError(t, err)
	notYet(t, acquired(t, g, small13), "a 13 MP original next to a 43 MP progressive one")
}

func TestMasterDecodeCostMB(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, enc func(f *os.File) error) string {
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, enc(f))
		require.NoError(t, f.Close())
		return path
	}
	jpg := write("m.jpg", func(f *os.File) error { return jpeg.Encode(f, quadrantImage(4000, 2500), nil) })
	pngPath := write("m.png", func(f *os.File) error { return png.Encode(f, image.NewGray(image.Rect(0, 0, 2000, 1500))) })

	assert.Equal(t, roundUpMB(10*3.25), masterDecodeCostMB(jpg), "Go writes 4:2:0 sequential JPEGs")
	assert.Equal(t, roundUpMB(3*costOtherMBPerMP), masterDecodeCostMB(pngPath))
	assert.Equal(t, int64(costUnknownMB), masterDecodeCostMB(filepath.Join(dir, "missing.jpg")))
}

// Making derivatives waits for the gate.
func TestProcessImageJob_WaitsForTheDecodeGate(t *testing.T) {
	prev := masterDecodes
	masterDecodes = newDecodeGate(4)
	defer func() { masterDecodes = prev }()

	env := newDisplayTestEnv(t)
	path, err := env.fm.GetMasterPath("waiting", ".jpg")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, quadrantImage(1600, 1000), nil))
	require.NoError(t, f.Close())

	held, err := masterDecodes.acquire(context.Background(), 4) // another large original is being decoded
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := env.wp.ProcessImageJob(context.Background(), DownloadJob{Ctx: context.Background(),
			Image: provider.Image{ID: "waiting", Path: "http://example.invalid/waiting.jpg", SourceQueryID: "q1"}})
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("the job decoded its master while the gate was full")
	case <-time.After(100 * time.Millisecond):
	}
	masterDecodes.release(held)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not go on after the gate opened")
	}
	assert.Zero(t, masterDecodes.used)
}

// A progressive original is decoded by progjpeg, others by imaging; both give image/jpeg's image.
func TestOpenMaster(t *testing.T) {
	for _, name := range []string{"video-001.q50.444.progressive.jpeg", "video-001.q50.420.jpeg"} {
		path := filepath.Join("testdata", "jpeg", name)
		f, err := os.Open(path)
		require.NoError(t, err)
		want, err := jpeg.Decode(f)
		f.Close()
		require.NoError(t, err)

		got, err := openMaster(path)
		require.NoError(t, err, name)
		require.Equal(t, want.Bounds(), got.Bounds(), name)
		for y := 0; y < want.Bounds().Dy(); y += 7 {
			for x := 0; x < want.Bounds().Dx(); x += 7 {
				wr, wg, wb, _ := want.At(x, y).RGBA()
				gr, gg, gb, _ := got.At(x, y).RGBA()
				require.Equal(t, [3]uint32{wr, wg, wb}, [3]uint32{gr, gg, gb}, "%s (%d,%d)", name, x, y)
			}
		}
	}
	_, err := openMaster(filepath.Join(t.TempDir(), "missing.jpg"))
	assert.Error(t, err)
}
