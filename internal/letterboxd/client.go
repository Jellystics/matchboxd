package letterboxd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/url"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	BaseURL = "https://letterboxd.com"

	// defaultBrowserProfile is the Chrome build whose TLS and HTTP/2
	// fingerprints we reproduce. The User-Agent and the client hints are
	// derived from it, so the whole request stays consistent.
	defaultBrowserProfile = "chrome_152"

	// maxConcurrent bounds the number of simultaneous requests to Letterboxd
	// across the whole process, to stay polite and avoid rate limiting.
	maxConcurrent = 3
	// Requests are spaced by a random delay in [minDelay, maxDelay], like a
	// person clicking through pages instead of a burst of requests.
	minDelay    = 120 * time.Millisecond
	maxDelay    = 320 * time.Millisecond
	maxBodySize = 8 << 20

	acceptHTML     = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
	acceptLanguage = "en-US,en;q=0.9"

	// sessionSaveInterval throttles writes of the cookie jar to disk.
	sessionSaveInterval = 10 * time.Second
)

// fetcherConfig configures the browser-like HTTP session used to read
// Letterboxd.
type fetcherConfig struct {
	baseURL     string
	profile     string // key of profiles.MappedTLSClients, e.g. "chrome_152"
	sessionFile string // where cookies are persisted; empty disables it
	insecureTLS bool   // only for httptest TLS servers
}

// httpDoer is the subset of the TLS client we use (fhttp.Client also
// implements it, which lets us degrade gracefully).
type httpDoer interface {
	Do(*fhttp.Request) (*fhttp.Response, error)
	CloseIdleConnections()
}

// fetcher downloads raw HTML pages from Letterboxd through a session that
// looks like a Chrome tab: same TLS/HTTP2 fingerprint, same headers in the
// same order, a warm-up navigation to the homepage and persistent cookies.
type fetcher struct {
	cfg    fetcherConfig
	client httpDoer
	jar    tls_client.CookieJar
	sess   *sessionStore

	userAgent string
	brands    string
	platform  string

	sem  chan struct{}
	pace pacer

	warmMu sync.Mutex
	warmed bool
}

func newFetcher(cfg fetcherConfig) *fetcher {
	cfg.baseURL = strings.TrimRight(cfg.baseURL, "/")
	profile, major, name := resolveProfile(cfg.profile)
	cfg.profile = name

	jar := tls_client.NewCookieJar()
	opts := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profile),
		tls_client.WithCookieJar(jar),
		tls_client.WithTimeoutSeconds(25),
	}
	if cfg.insecureTLS {
		opts = append(opts, tls_client.WithInsecureSkipVerify())
	}

	var client httpDoer
	if c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...); err == nil {
		client = c
	} else {
		// Should not happen with a known profile; keep the app usable rather
		// than failing at startup.
		log.Printf("letterboxd: TLS fingerprinting unavailable (%v); using a plain HTTP client", err)
		client = &fhttp.Client{Timeout: 25 * time.Second, Jar: jar}
	}

	f := &fetcher{
		cfg:       cfg,
		client:    client,
		jar:       jar,
		sem:       make(chan struct{}, maxConcurrent),
		userAgent: fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", major),
		brands:    fmt.Sprintf(`"Google Chrome";v="%s", "Chromium";v="%s", "Not:A-Brand";v="24"`, major, major),
		platform:  `"macOS"`,
	}
	if cfg.sessionFile != "" {
		f.sess = &sessionStore{path: cfg.sessionFile, profile: cfg.profile}
		f.loadSession()
	}
	return f
}

// resolveProfile returns the TLS profile to impersonate, the matching Chrome
// major version and the profile key. Only Chrome is supported because the
// request headers (User-Agent, client hints) are Chrome's.
func resolveProfile(name string) (profiles.ClientProfile, string, string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = defaultBrowserProfile
	}
	version := strings.TrimPrefix(name, "chrome_")
	if name != defaultBrowserProfile && (version == name || strings.Contains(version, "_")) {
		log.Printf("letterboxd: unsupported browser profile %q, using %s", name, defaultBrowserProfile)
		name, version = defaultBrowserProfile, strings.TrimPrefix(defaultBrowserProfile, "chrome_")
	}
	p, ok := profiles.MappedTLSClients[name]
	if !ok {
		log.Printf("letterboxd: unknown browser profile %q, using %s", name, defaultBrowserProfile)
		name, version = defaultBrowserProfile, strings.TrimPrefix(defaultBrowserProfile, "chrome_")
		p = profiles.MappedTLSClients[name]
	}
	return p, version, name
}

// get fetches path (e.g. "/user/films/page/2/") and returns the body of a
// successful answer, or one of the sentinel errors. referer is the page a
// browser would have come from ("" for a typed URL).
func (f *fetcher) get(ctx context.Context, path, referer string) ([]byte, error) {
	select {
	case f.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-f.sem }()

	if err := f.pace.wait(ctx); err != nil {
		return nil, err
	}
	f.ensureSession(ctx)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, backoff(attempt)); err != nil {
				return nil, err
			}
		}
		body, retry, err := f.do(ctx, path, referer)
		if err == nil || !retry {
			return body, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// ensureSession performs the first navigation of a browser session: the
// homepage, which sets Cloudflare's session cookies (__cf_bm, ...). It runs
// once per process and is best effort. When a valid cf_clearance is already
// stored, we skip it: the session is warm.
func (f *fetcher) ensureSession(ctx context.Context) {
	f.warmMu.Lock()
	if f.warmed {
		f.warmMu.Unlock()
		return
	}
	f.warmed = true
	f.warmMu.Unlock()

	// Local test servers have no Cloudflare session to establish.
	if f.cfg.insecureTLS {
		return
	}

	if u, err := url.Parse(f.cfg.baseURL + "/"); err == nil {
		for _, c := range f.jar.Cookies(u) {
			if c.Name == "cf_clearance" {
				return
			}
		}
	}
	if _, _, err := f.do(ctx, "/", ""); err != nil && ctx.Err() == nil {
		log.Printf("letterboxd: session warm-up failed: %v", err)
	}
}

func (f *fetcher) do(ctx context.Context, path, referer string) (body []byte, retry bool, err error) {
	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, f.cfg.baseURL+path, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header = f.headers(referer)

	// Cookies set by Cloudflare must survive even when the page is blocked.
	defer f.persistSession(true)

	resp, err := f.client.Do(req)
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

	challenge := isChallenge(resp.Header, body)
	switch {
	case resp.StatusCode == fhttp.StatusOK:
		if challenge {
			return nil, false, ErrBlocked
		}
		return body, false, nil
	case resp.StatusCode == fhttp.StatusNotFound:
		return nil, false, ErrNotFound
	case resp.StatusCode == fhttp.StatusTooManyRequests:
		return nil, true, ErrBlocked
	case resp.StatusCode == fhttp.StatusForbidden, challenge:
		return nil, false, ErrBlocked
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("%w: HTTP %d", ErrUpstream, resp.StatusCode)
	default:
		return nil, false, fmt.Errorf("%w: HTTP %d", ErrUpstream, resp.StatusCode)
	}
}

// headers builds a navigation request whose header set, values and order
// match Chrome's.
func (f *fetcher) headers(referer string) fhttp.Header {
	site := "none"
	h := fhttp.Header{
		"sec-ch-ua":                 {f.brands},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {f.platform},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {f.userAgent},
		"accept":                    {acceptHTML},
		"sec-fetch-site":            {site},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-user":            {"?1"},
		"sec-fetch-dest":            {"document"},
		"accept-language":           {acceptLanguage},
		"priority":                  {"u=0, i"},
	}
	order := []string{
		"sec-ch-ua",
		"sec-ch-ua-mobile",
		"sec-ch-ua-platform",
		"upgrade-insecure-requests",
		"user-agent",
		"accept",
		"sec-fetch-site",
		"sec-fetch-mode",
		"sec-fetch-user",
		"sec-fetch-dest",
	}
	if referer != "" {
		h["referer"] = []string{referer}
		h["sec-fetch-site"] = []string{"same-origin"}
		order = append(order, "referer")
	}
	order = append(order, "accept-language", "priority")
	h[fhttp.HeaderOrderKey] = order
	return h
}

// referer returns the page a browser would be on before opening path: none
// for the first page of a source (typed URL) and the source page itself for
// the following ones (link click).
func (f *fetcher) referer(base string, page int) string {
	if page <= 1 {
		return ""
	}
	return f.cfg.baseURL + base
}

func (f *fetcher) homepage() string { return f.cfg.baseURL + "/" }

// close flushes the session to disk and closes idle connections.
func (f *fetcher) close() {
	f.persistSession(false)
	f.client.CloseIdleConnections()
}

// pacer spaces out request starts, with some jitter.
type pacer struct {
	mu   sync.Mutex
	next time.Time
}

func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	now := time.Now()
	at := p.next
	if at.Before(now) {
		at = now
	}
	p.next = at.Add(minDelay + rand.N(maxDelay-minDelay))
	p.mu.Unlock()
	return sleep(ctx, time.Until(at))
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 2500 * time.Millisecond
	return d + rand.N(time.Second)
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isChallenge reports whether a response is a Cloudflare bot challenge.
func isChallenge(h fhttp.Header, body []byte) bool {
	if h.Get("cf-mitigated") == "challenge" {
		return true
	}
	return bytes.Contains(body, []byte("<title>Just a moment...</title>")) ||
		bytes.Contains(body, []byte("_cf_chl_opt"))
}

// isLoopback reports whether rawURL points at a local test server.
func isLoopback(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}
