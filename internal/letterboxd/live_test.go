package letterboxd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Live tests hit the real letterboxd.com. They are skipped unless
// MATCHBOXD_LIVE=1 is set:
//
//	MATCHBOXD_LIVE=1 go test ./internal/letterboxd -run Live -v
//
// They check that the parsers still match the current Letterboxd markup.
func liveClient(t *testing.T) (*Client, context.Context) {
	t.Helper()
	if os.Getenv("MATCHBOXD_LIVE") != "1" {
		t.Skip("set MATCHBOXD_LIVE=1 to run tests against letterboxd.com")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return New(time.Minute), ctx
}

func TestLive_Sources(t *testing.T) {
	c, ctx := liveClient(t)
	sources, complete, err := c.Sources(ctx, "davidehrlich")
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Skip("Letterboxd blocked some /lists/ pages from this network; got the first page only")
	}
	var lists int
	for _, s := range sources {
		if s.Kind == SourceList {
			lists++
		}
	}
	t.Logf("%d sources, %d custom lists", len(sources), lists)
	if lists < 50 {
		t.Errorf("only %d custom lists found; pagination of /lists/ may be broken", lists)
	}
}

func TestLive_WatchlistPagination(t *testing.T) {
	c, ctx := liveClient(t)
	first, err := c.gridPage(ctx, "/karsten/watchlist/", 1)
	if err != nil {
		t.Fatal(err)
	}
	films, err := c.Films(ctx, "karsten", SourceWatchlist)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("watchlist: %d films over %d pages (announced: %d)", len(films), first.LastPage, first.Total)
	if first.LastPage < 2 {
		t.Fatalf("expected a paginated watchlist, got %d page(s)", first.LastPage)
	}
	// Allow a small drift if the watchlist changes during the test.
	if diff := first.Total - len(films); diff < -5 || diff > 5 {
		t.Errorf("fetched %d films, page announces %d", len(films), first.Total)
	}
}

func TestLive_WatchedFilmsAndList(t *testing.T) {
	c, ctx := liveClient(t)
	list, err := c.Films(ctx, "davidehrlich", "list:christopher-nolan-films-ranked")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 10 {
		t.Errorf("list has %d films, want >= 10", len(list))
	}
	first, err := c.gridPage(ctx, "/davidehrlich/films/", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Films) != 72 || first.LastPage < 10 {
		t.Errorf("films page 1: %d films, %d pages", len(first.Films), first.LastPage)
	}
}

func TestLive_FilmDetails(t *testing.T) {
	c, ctx := liveClient(t)
	f, err := c.Film(ctx, "parasite-2019")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)
	if f.Title != "Parasite" || f.Year != 2019 || f.Runtime == 0 || f.IMDbID != "tt6751668" ||
		f.Rating == 0 || len(f.Directors) == 0 || len(f.Genres) == 0 || f.Poster == "" {
		t.Errorf("incomplete film details: %+v", f)
	}
}

func TestLive_UnknownUser(t *testing.T) {
	c, ctx := liveClient(t)
	if _, _, err := c.Sources(ctx, "thisuserdoesnotexist12345xyz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLive_Likes(t *testing.T) {
	c, ctx := liveClient(t)
	films, err := c.Films(ctx, "davidehrlich", SourceLikes)
	switch {
	case errors.Is(err, ErrBlocked):
		t.Skip("Letterboxd serves a Cloudflare challenge for /likes/ from this network")
	case err != nil:
		t.Fatal(err)
	}
	t.Logf("%d liked films", len(films))
	if len(films) == 0 {
		t.Error("no liked films parsed")
	}
}
