package smk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(hedgeAfter, headerTimeout, stall time.Duration, tries int) *http.Client {
	return &http.Client{Transport: &hedgeTransport{
		next:       &http.Transport{ResponseHeaderTimeout: headerTimeout},
		hedgeAfter: hedgeAfter,
		tries:      tries,
		stall:      stall,
	}}
}

// stuckFirst answers every request except the first n, which never get headers (the stalls seen on
// iip.smk.dk). It counts the requests it saw and how many are still open.
type stuckFirst struct {
	n       int32
	seen    atomic.Int32
	open    atomic.Int32
	release chan struct{}
}

func (s *stuckFirst) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.open.Add(1)
	defer s.open.Add(-1)
	if s.seen.Add(1) <= s.n {
		select {
		case <-s.release:
		case <-r.Context().Done():
		}
		return
	}
	_, _ = w.Write([]byte("image"))
}

// A request without headers gets a second one next to it, and the second answer is used; the
// stuck request is cancelled.
func TestHedgeTransportSendsAgainWhenStalled(t *testing.T) {
	h := &stuckFirst{n: 1, release: make(chan struct{})}
	srv := httptest.NewServer(h)
	defer srv.Close()
	defer close(h.release)

	start := time.Now()
	resp, err := testClient(50*time.Millisecond, 5*time.Second, time.Second, 3).Get(srv.URL)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "image", string(b))
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Equal(t, int32(2), h.seen.Load())
	assert.Eventually(t, func() bool { return h.open.Load() == 0 }, 2*time.Second, 10*time.Millisecond, "the stuck request is cancelled")
}

// fakeNext stands in for the network under hedgeTransport. It counts the requests and lets the
// test decide when and how each one is answered, so the hedge tests below do not depend on how
// quickly a local server answers or how promptly the timers fire on a busy machine (with a real
// server and 50-300 ms margins they failed now and then under load).
type fakeNext struct {
	seen   atomic.Int32
	answer func(r *http.Request) (*http.Response, error)
}

func (f *fakeNext) RoundTrip(r *http.Request) (*http.Response, error) {
	f.seen.Add(1)
	return f.answer(r)
}

func answerAtOnce(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("image"))}, nil
}

// headerTimeout is the error http.Transport returns when ResponseHeaderTimeout runs out.
type headerTimeout struct{}

func (headerTimeout) Error() string   { return "net/http: timeout awaiting response headers" }
func (headerTimeout) Timeout() bool   { return true }
func (headerTimeout) Temporary() bool { return true }

func hedgeClient(next http.RoundTripper, hedgeAfter time.Duration) *http.Client {
	return &http.Client{Transport: &hedgeTransport{next: next, hedgeAfter: hedgeAfter, tries: 3, stall: time.Second}}
}

// A fast answer sends no extra request, and nothing is sent once a request has been answered.
func TestHedgeTransportFastAnswerIsNotRepeated(t *testing.T) {
	next := &fakeNext{answer: answerAtOnce}
	resp, err := hedgeClient(next, time.Hour).Get("https://iip.smk.dk/x.jpg")
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	assert.Equal(t, int32(1), next.seen.Load(), "answered before hedgeAfter: one request")

	// With a short hedgeAfter a busy machine may legitimately send a second request before the
	// first answer is read; what must not happen is a request after the answer.
	next = &fakeNext{answer: answerAtOnce}
	resp, err = hedgeClient(next, 10*time.Millisecond).Get("https://iip.smk.dk/x.jpg")
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	answered := next.seen.Load()
	time.Sleep(100 * time.Millisecond) // ten hedge intervals
	assert.Equal(t, answered, next.seen.Load(), "no request is sent after the answer")
}

// When every request is stuck, no more than tries are sent and the download fails with a timeout
// once the last one gives up, so the downloader can retry or back off.
func TestHedgeTransportGivesUpAfterTries(t *testing.T) {
	giveUp := make(chan struct{}, 3) // each value lets one stuck request fail with a header timeout
	var failed atomic.Int32
	next := &fakeNext{answer: func(r *http.Request) (*http.Response, error) {
		select {
		case <-giveUp:
			failed.Add(1)
			return nil, headerTimeout{}
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}}
	done := make(chan error, 1)
	go func() {
		_, err := hedgeClient(next, time.Millisecond).Get("https://iip.smk.dk/x.jpg")
		done <- err
	}()

	require.Eventually(t, func() bool { return next.seen.Load() == 3 }, 10*time.Second, time.Millisecond, "all tries are sent")

	// Two of the three give up: the download keeps waiting for the last one.
	giveUp <- struct{}{}
	giveUp <- struct{}{}
	require.Eventually(t, func() bool { return failed.Load() == 2 }, 10*time.Second, time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("returned before the last request gave up: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	giveUp <- struct{}{}
	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("no result after the last request gave up")
	}
	require.Error(t, err)
	var ne net.Error
	require.True(t, errors.As(err, &ne), "got %T: %v", err, err)
	assert.True(t, ne.Timeout())
	assert.Equal(t, int32(3), next.seen.Load(), "no more than tries are sent")
}

// An error that comes back at once (nothing listening) is returned, not hedged; the downloader
// waits before it tries again.
func TestHedgeTransportReturnsImmediateErrors(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	url := "http://" + l.Addr().String() + "/x.jpg"
	require.NoError(t, l.Close())

	start := time.Now()
	_, err = testClient(time.Second, time.Second, time.Second, 3).Get(url)
	require.Error(t, err)
	assert.Less(t, time.Since(start), time.Second)
}

// A body that keeps arriving, however slowly, is not cut off; the app's shared client gave up on
// any download after 60 s in total.
//
// The whole body takes longer than the stall timeout, but no gap between reads comes near it. The
// timer counts real time between reads, so a gap that is close to it fails the test whenever the
// machine holds up the server or the reader for the difference: with 60 ms gaps against 200 ms it
// failed once under memory pressure, and a single 250 ms pause of the reader failed it every time.
// Now a pause has to last almost a second.
func TestStallBodyKeepsSlowBodies(t *testing.T) {
	const chunks, gap, stall = 25, 50 * time.Millisecond, time.Second // 1.25 s in all
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < chunks; i++ {
			_, _ = w.Write([]byte("0123456789"))
			w.(http.Flusher).Flush()
			time.Sleep(gap)
		}
	}))
	defer srv.Close()

	resp, err := testClient(time.Second, time.Second, stall, 3).Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Len(t, b, chunks*10)
}

// A body that stops moving ends with a timeout error, which the downloader retries.
func TestStallBodyCutsStalledBodies(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("first bytes"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	resp, err := testClient(time.Second, time.Second, 150*time.Millisecond, 3).Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
	var ne net.Error
	require.True(t, errors.As(err, &ne), "got %T: %v", err, err)
	assert.True(t, ne.Timeout())
}

// Cancelling the download cancels every request it sent.
func TestHedgeTransportCancel(t *testing.T) {
	h := &stuckFirst{n: 100, release: make(chan struct{})}
	srv := httptest.NewServer(h)
	defer srv.Close()
	defer close(h.release)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = testClient(50*time.Millisecond, 5*time.Second, time.Second, 3).Do(req)
	require.Error(t, err)
	assert.Eventually(t, func() bool { return h.open.Load() == 0 }, 2*time.Second, 10*time.Millisecond)
}

func TestProviderUsesOwnImageClient(t *testing.T) {
	shared := &http.Client{}
	p := NewProvider(nil, shared)
	assert.NotSame(t, shared, p.GetClient(), "images use SMK's own client")
	tr, ok := p.GetClient().Transport.(*hedgeTransport)
	require.True(t, ok)
	assert.Equal(t, imageTries, tr.tries)
	assert.Equal(t, imageHeaderTimeout, tr.next.(*http.Transport).ResponseHeaderTimeout)
	assert.Equal(t, imageDownloadAttempts, p.DownloadAttempts())
}
