package letterboxd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// maxPages caps how many pages of a single source we are willing to fetch
// (250 pages = 18 000 watched films, or 7 000 watchlist entries).
const maxPages = 250

// Source kinds.
const (
	SourceWatchlist = "watchlist"
	SourceFilms     = "films"
	SourceLikes     = "likes"
	SourceList      = "list"
)

// Source is a selectable set of films for a user.
type Source struct {
	ID    string `json:"id"` // "watchlist", "films", "likes" or "list:<slug>"
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Count int    `json:"count,omitempty"` // known film count, if any
}

var (
	usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,40}$`)
	slugRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,200}$`)
)

// ValidUsername reports whether s looks like a Letterboxd username.
func ValidUsername(s string) bool { return usernameRe.MatchString(s) }

// Client reads public Letterboxd pages, with an in-memory cache.
type Client struct {
	f         *fetcher
	gridPages *Cache[filmPage]
	listPages *Cache[listsPage]
	films     *Cache[Film]
}

// Option configures the Letterboxd client.
type Option func(*clientConfig)

type clientConfig struct {
	sessionFile string
	profile     string
}

// WithSessionFile stores the browser session (cookies) in path so that it
// survives restarts. An empty path disables persistence.
func WithSessionFile(path string) Option {
	return func(c *clientConfig) { c.sessionFile = path }
}

// WithBrowserProfile selects the Chrome build whose TLS and HTTP/2
// fingerprints are imitated, e.g. "chrome_152".
func WithBrowserProfile(name string) Option {
	return func(c *clientConfig) { c.profile = name }
}

// New returns a client that caches pages for ttl.
func New(ttl time.Duration, opts ...Option) *Client { return newClient(BaseURL, ttl, opts...) }

func newClient(baseURL string, ttl time.Duration, opts ...Option) *Client {
	var cfg clientConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Client{
		f: newFetcher(fetcherConfig{
			baseURL:     baseURL,
			profile:     cfg.profile,
			sessionFile: cfg.sessionFile,
			insecureTLS: isLoopback(baseURL),
		}),
		gridPages: NewCache[filmPage](ttl),
		listPages: NewCache[listsPage](ttl),
		// Film details change rarely; keep them longer.
		films: NewCache[Film](6 * ttl),
	}
}

// Close flushes the browser session to disk and releases idle connections.
func (c *Client) Close() { c.f.close() }

// Sources returns all the film sources of a user: the three built-in ones
// and every custom list. It returns ErrNotFound for an unknown user.
// complete is false when some pages of the user's lists could not be read
// (e.g. blocked by Letterboxd): the lists found so far are still returned.
func (c *Client) Sources(ctx context.Context, user string) (sources []Source, complete bool, err error) {
	user = strings.ToLower(user)
	if !ValidUsername(user) {
		return nil, false, ErrNotFound
	}
	lists, complete, err := c.lists(ctx, user)
	if err != nil {
		return nil, false, err
	}
	sources = []Source{
		{ID: SourceWatchlist, Kind: SourceWatchlist, Label: "Watchlist"},
		{ID: SourceFilms, Kind: SourceFilms, Label: "Watched films"},
		{ID: SourceLikes, Kind: SourceLikes, Label: "Liked films"},
	}
	for _, l := range lists {
		sources = append(sources, Source{ID: SourceList + ":" + l.Slug, Kind: SourceList, Label: l.Name, Count: l.Count})
	}
	return sources, complete, nil
}

func (c *Client) lists(ctx context.Context, user string) (lists []ListRef, complete bool, err error) {
	base := "/" + user + "/lists/"
	first, err := c.listsPage(ctx, base, 1)
	if err != nil {
		return nil, false, err
	}
	pages := make([]listsPage, min(first.LastPage, maxPages))
	pages[0] = first
	// Later pages are best effort: a failure there should not hide the
	// lists we already have.
	err = forEachPage(ctx, len(pages), func(ctx context.Context, n int) error {
		p, err := c.listsPage(ctx, base, n)
		pages[n-1] = p
		return err
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	seen := map[string]bool{}
	for _, p := range pages {
		for _, l := range p.Lists {
			if !seen[l.Slug] {
				seen[l.Slug] = true
				lists = append(lists, l)
			}
		}
	}
	return lists, err == nil, nil
}

func (c *Client) listsPage(ctx context.Context, base string, n int) (listsPage, error) {
	path := pagePath(base, n)
	return c.listPages.GetOrLoad(path, func() (listsPage, error) {
		body, err := c.f.get(ctx, path, c.f.referer(base, n))
		if err != nil {
			return listsPage{}, err
		}
		return parseListsPage(body)
	})
}

// SourcePath returns the Letterboxd path of a source, or an error if the
// source ID is malformed.
func SourcePath(user, sourceID string) (string, error) {
	user = strings.ToLower(user)
	if !ValidUsername(user) {
		return "", ErrNotFound
	}
	switch {
	case sourceID == SourceWatchlist:
		return "/" + user + "/watchlist/", nil
	case sourceID == SourceFilms:
		return "/" + user + "/films/", nil
	case sourceID == SourceLikes:
		return "/" + user + "/likes/films/", nil
	case strings.HasPrefix(sourceID, SourceList+":"):
		slug := strings.TrimPrefix(sourceID, SourceList+":")
		if !slugRe.MatchString(slug) {
			return "", fmt.Errorf("invalid list slug %q", slug)
		}
		return "/" + user + "/list/" + slug + "/", nil
	}
	return "", fmt.Errorf("unknown source %q", sourceID)
}

// Films returns every film of a source, fetching all its pages.
func (c *Client) Films(ctx context.Context, user, sourceID string) ([]FilmRef, error) {
	base, err := SourcePath(user, sourceID)
	if err != nil {
		return nil, err
	}
	first, err := c.gridPage(ctx, base, 1)
	if err != nil {
		return nil, err
	}
	if first.Private {
		return nil, ErrPrivate
	}
	pages := make([]filmPage, min(first.LastPage, maxPages))
	pages[0] = first
	err = forEachPage(ctx, len(pages), func(ctx context.Context, n int) error {
		p, err := c.gridPage(ctx, base, n)
		pages[n-1] = p
		return err
	})
	if err != nil {
		return nil, err
	}
	var out []FilmRef
	seen := map[string]bool{}
	for _, p := range pages {
		for _, f := range p.Films {
			if !seen[f.Slug] {
				seen[f.Slug] = true
				out = append(out, f)
			}
		}
	}
	return out, nil
}

func (c *Client) gridPage(ctx context.Context, base string, n int) (filmPage, error) {
	path := pagePath(base, n)
	return c.gridPages.GetOrLoad(path, func() (filmPage, error) {
		body, err := c.f.get(ctx, path, c.f.referer(base, n))
		if err != nil {
			return filmPage{}, err
		}
		return parseFilmGrid(body)
	})
}

// Film returns the details of a film.
func (c *Client) Film(ctx context.Context, slug string) (Film, error) {
	if !slugRe.MatchString(slug) {
		return Film{}, ErrNotFound
	}
	return c.films.GetOrLoad(slug, func() (Film, error) {
		body, err := c.f.get(ctx, "/film/"+slug+"/", c.f.homepage())
		if err != nil {
			return Film{}, err
		}
		return parseFilm(slug, body)
	})
}

func pagePath(base string, n int) string {
	if n <= 1 {
		return base
	}
	return fmt.Sprintf("%spage/%d/", base, n)
}

// forEachPage calls fn for pages 2..last concurrently. The global request
// semaphore in the fetcher keeps the actual concurrency reasonable. The first
// error cancels the remaining requests.
func forEachPage(ctx context.Context, last int, fn func(context.Context, int) error) error {
	if last < 2 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	for n := 2; n <= last; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if err := fn(ctx, n); err != nil {
				once.Do(func() { firstErr = err; cancel() })
			}
		}(n)
	}
	wg.Wait()
	return firstErr
}
