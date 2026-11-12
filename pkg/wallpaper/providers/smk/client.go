package smk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dixieflatline76/Spice/v2/util/log"
)

// SMK's image server (iip.smk.dk, and iip-thumb.smk.dk behind the same front end) behaves unlike
// the other museums' (measurements in docs/smk-download.md):
//
//   - Every connection gets about 0.85 MB/s, however many run at once, and its IIIF JPEGs are
//     quality 100, so a 3840 px image (6–12 MB) takes 8–15 s when nothing is wrong.
//   - At random, single requests get no first byte for 15–100 s, and some are never answered,
//     while a request sent at the same moment is answered in under a second. Small requests
//     (info.json) stall the same way, so it is not the image size.
//
// The app's shared client gives up on a request after 15 s without headers and 60 s in total, and
// tries again after 1.5 s, four times. SMK images use their own client instead: when a request has
// no headers after imageHedgeAfter, a second identical request is sent while the first keeps
// waiting, and whichever answers first is used. Only the body's progress is timed, not the whole
// download, so a large image on a slow connection is not cut off while data still arrives.
const (
	// imageHedgeAfter is how long a request may go without headers before another is sent next to
	// it. Healthy requests have their headers in 0.5–6 s.
	imageHedgeAfter = 12 * time.Second
	// imageTries is how many requests one download may have running at once (the first plus the
	// hedged ones).
	imageTries = 3
	// imageHeaderTimeout gives up on one request that still has no headers. The last of
	// imageTries is sent after (imageTries-1)×imageHedgeAfter, so a download whose requests are all
	// stuck fails after about 70 s, a little longer than the shared client's four tries.
	imageHeaderTimeout = 45 * time.Second
	// imageStallTimeout ends a download whose body has not moved for this long.
	imageStallTimeout = 30 * time.Second
	// imageMaxDuration bounds one download in total.
	imageMaxDuration = 5 * time.Minute
	// imageDownloadAttempts is how often the downloader tries each SMK image. The client already
	// sends up to imageTries requests per attempt.
	imageDownloadAttempts = 2
)

// newImageClient returns the client SMK image downloads use.
func newImageClient() *http.Client {
	return &http.Client{
		Timeout: imageMaxDuration,
		Transport: &hedgeTransport{
			next: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: imageHeaderTimeout,
				IdleConnTimeout:       90 * time.Second,
				MaxIdleConnsPerHost:   4,
			},
			hedgeAfter: imageHedgeAfter,
			tries:      imageTries,
			stall:      imageStallTimeout,
		},
	}
}

// errStalled is returned when a response body stops arriving for longer than the stall timeout.
// It is a timeout, so the downloader retries it like any other.
var errStalled error = &stallError{}

type stallError struct{}

func (*stallError) Error() string {
	return fmt.Sprintf("image download stalled (no data for %v)", imageStallTimeout)
}
func (*stallError) Timeout() bool   { return true }
func (*stallError) Temporary() bool { return true }

// hedgeTransport sends a request again, next to the first, when it has no headers after
// hedgeAfter, up to tries requests, and returns the first response. Requests must have no body
// (image downloads are GETs). The response body is cancelled if it stops moving for stall.
type hedgeTransport struct {
	next       http.RoundTripper
	hedgeAfter time.Duration
	tries      int
	stall      time.Duration
}

type hedgeResult struct {
	i    int // index into cancels
	resp *http.Response
	err  error
}

func (t *hedgeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil && req.Body != http.NoBody {
		return nil, errors.New("smk: hedged requests cannot have a body")
	}
	results := make(chan hedgeResult, t.tries)
	var cancels []context.CancelCauseFunc
	launch := func() {
		ctx, cancel := context.WithCancelCause(req.Context())
		i := len(cancels)
		cancels = append(cancels, cancel)
		r := req.Clone(ctx)
		go func() {
			resp, err := t.next.RoundTrip(r)
			results <- hedgeResult{i, resp, err}
		}()
	}

	start := time.Now()
	launch()
	pending := 1
	timer := time.NewTimer(t.hedgeAfter)
	defer timer.Stop()
	for {
		select {
		case r := <-results:
			pending--
			if r.err == nil {
				if len(cancels) > 1 {
					log.Printf("SMK: request %d/%d answered first, %v after the first was sent (%s)", r.i+1, len(cancels), time.Since(start).Round(100*time.Millisecond), req.URL)
				}
				// Stop the other requests; a response that still arrives is closed.
				for i, c := range cancels {
					if i != r.i {
						c(nil)
					}
				}
				go drain(results, cancels, pending)
				body := &stallBody{body: r.resp.Body, cancel: cancels[r.i], stall: t.stall}
				body.timer = time.AfterFunc(t.stall, body.expire)
				r.resp.Body = body
				return r.resp, nil
			}
			cancels[r.i](nil)
			if pending == 0 {
				// Every request so far failed outright (refused, reset, header timeout): leave the
				// retry to the downloader, which waits between attempts.
				return nil, r.err
			}
		case <-timer.C:
			if len(cancels) < t.tries {
				log.Printf("SMK: no answer after %v for %s; sending request %d/%d next to it", time.Duration(len(cancels))*t.hedgeAfter, req.URL, len(cancels)+1, t.tries)
				launch()
				pending++
				timer.Reset(t.hedgeAfter)
			}
		case <-req.Context().Done():
			for _, c := range cancels {
				c(nil)
			}
			go drain(results, cancels, pending)
			return nil, req.Context().Err()
		}
	}
}

// drain closes the responses of requests that lost or were cancelled.
func drain(results <-chan hedgeResult, cancels []context.CancelCauseFunc, pending int) {
	for ; pending > 0; pending-- {
		r := <-results
		if r.resp != nil {
			_ = r.resp.Body.Close()
		}
		cancels[r.i](nil)
	}
}

type stallBody struct {
	body   io.ReadCloser
	cancel context.CancelCauseFunc
	stall  time.Duration
	timer  *time.Timer

	mu      sync.Mutex
	stalled bool
}

func (b *stallBody) expire() {
	b.mu.Lock()
	b.stalled = true
	b.mu.Unlock()
	b.cancel(errStalled)
}

func (b *stallBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	if n > 0 {
		b.timer.Reset(b.stall)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		b.mu.Lock()
		stalled := b.stalled
		b.mu.Unlock()
		if stalled {
			err = errStalled
		}
	}
	return n, err
}

func (b *stallBody) Close() error {
	b.timer.Stop()
	err := b.body.Close()
	b.cancel(nil)
	return err
}
