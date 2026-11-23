package letterboxd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// gridHTML builds a minimal grid page with the given slugs and pagination.
func gridHTML(slugs []string, last int) string {
	var b strings.Builder
	b.WriteString(`<html><body><div id="content"><div class="poster-grid"><ul class="grid">`)
	for _, s := range slugs {
		fmt.Fprintf(&b, `<li class="griditem"><div class="react-component" data-item-slug="%s" data-item-name="%s"></div></li>`, s, s)
	}
	b.WriteString(`</ul></div><div class="paginate-pages"><ul>`)
	for i := 2; i <= last; i++ {
		fmt.Fprintf(&b, `<li class="paginate-page"><a href="/x/films/page/%d/">%d</a></li>`, i, i)
	}
	b.WriteString(`</ul></div></div></body></html>`)
	return b.String()
}

func TestFilms_FetchesAllPagesAndCaches(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/alice/films/":
			fmt.Fprint(w, gridHTML([]string{"a", "b"}, 3))
		case "/alice/films/page/2/":
			fmt.Fprint(w, gridHTML([]string{"c", "b"}, 3))
		case "/alice/films/page/3/":
			fmt.Fprint(w, gridHTML([]string{"d"}, 3))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, time.Minute)
	films, err := c.Films(context.Background(), "alice", SourceFilms)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range films {
		got = append(got, f.Slug)
	}
	if strings.Join(got, ",") != "a,b,c,d" {
		t.Errorf("got %v, want a,b,c,d (deduplicated, in page order)", got)
	}
	if hits.Load() != 3 {
		t.Errorf("%d requests, want 3", hits.Load())
	}
	if _, err := c.Films(context.Background(), "alice", SourceFilms); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 3 {
		t.Errorf("second call made new requests (%d total): cache not used", hits.Load())
	}
}

func TestFilms_Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/blocked/films/":
			w.Header().Set("cf-mitigated", "challenge")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "<title>Just a moment...</title>")
		case "/private/watchlist/":
			fmt.Fprint(w, `<html><body><div id="content"><p>Bob’s watchlist is private.</p></div></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newClient(srv.URL, time.Minute)
	ctx := context.Background()

	cases := []struct {
		user, source string
		want         error
	}{
		{"nobody", SourceFilms, ErrNotFound},
		{"blocked", SourceFilms, ErrBlocked},
		{"private", SourceWatchlist, ErrPrivate},
	}
	for _, tc := range cases {
		if _, err := c.Films(ctx, tc.user, tc.source); !errors.Is(err, tc.want) {
			t.Errorf("%s/%s: err = %v, want %v", tc.user, tc.source, err, tc.want)
		}
	}
}

func TestSourcePath(t *testing.T) {
	ok := map[string]string{
		"watchlist":           "/bob/watchlist/",
		"films":               "/bob/films/",
		"likes":               "/bob/likes/films/",
		"list:my-top-10-2026": "/bob/list/my-top-10-2026/",
	}
	for src, want := range ok {
		got, err := SourcePath("Bob", src)
		if err != nil || got != want {
			t.Errorf("SourcePath(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
	for _, src := range []string{"", "diary", "list:", "list:../../x", "list:a/b"} {
		if _, err := SourcePath("bob", src); err == nil {
			t.Errorf("SourcePath(%q) accepted", src)
		}
	}
	if _, err := SourcePath("../etc", "films"); err == nil {
		t.Error("invalid username accepted")
	}
}
