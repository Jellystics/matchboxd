package letterboxd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL   = "https://letterboxd.com"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	// maxConcurrent bounds the number of simultaneous requests to Letterboxd
	// across the whole process, to stay polite and avoid rate limiting.
	maxConcurrent = 3
	// minInterval spaces out request starts (at most ~6 requests per second).
	minInterval = 160 * time.Millisecond
	maxBodySize = 8 << 20
)

// fetcher downloads raw HTML pages from Letterboxd.
type fetcher struct {
	http    *http.Client
	baseURL string
	sem     chan struct{}
	ticks   <-chan time.Time
}

func newFetcher(baseURL string) *fetcher {
	return &fetcher{
		http:    &http.Client{Timeout: 20 * time.Second},
		baseURL: baseURL,
		sem:     make(chan struct{}, maxConcurrent),
		ticks:   time.Tick(minInterval),
	}
}

// get fetches path (e.g. "/user/films/page/2/") and returns the body of a
// successful answer, or one of the sentinel errors.
func (f *fetcher) get(ctx context.Context, path string) ([]byte, error) {
	select {
	case f.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-f.sem }()
	select {
	case <-f.ticks:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		body, retry, err := f.do(ctx, path)
		if err == nil || !retry {
			return body, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (f *fetcher) do(ctx context.Context, path string) (body []byte, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+path, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, true, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, true, fmt.Errorf("%w: reading body: %v", ErrUpstream, err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		if isChallenge(resp.Header, body) {
			return nil, false, ErrBlocked
		}
		return body, false, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, false, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, true, ErrBlocked
	case resp.StatusCode == http.StatusForbidden:
		return nil, false, ErrBlocked
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("%w: HTTP %d", ErrUpstream, resp.StatusCode)
	default:
		return nil, false, fmt.Errorf("%w: HTTP %d", ErrUpstream, resp.StatusCode)
	}
}

// isChallenge reports whether a response is a Cloudflare bot challenge.
func isChallenge(h http.Header, body []byte) bool {
	if h.Get("cf-mitigated") == "challenge" {
		return true
	}
	return bytes.Contains(body, []byte("<title>Just a moment...</title>"))
}
