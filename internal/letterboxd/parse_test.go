package letterboxd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The files in testdata/ are real Letterboxd pages saved in November 2026.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseFilmGrid_WatchedFilms(t *testing.T) {
	p, err := parseFilmGrid(readFixture(t, "films_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Films) != 72 {
		t.Errorf("got %d films, want 72", len(p.Films))
	}
	if p.LastPage != 49 {
		t.Errorf("LastPage = %d, want 49", p.LastPage)
	}
	if p.Private {
		t.Error("page flagged as private")
	}
	first := p.Films[0]
	if first.Slug != "the-further-mis-adventures-of-cliff-booth" {
		t.Errorf("first slug = %q", first.Slug)
	}
	if first.Name != "The Further Mis-Adventures of Cliff Booth (2026)" {
		t.Errorf("first name = %q", first.Name)
	}
}

func TestParseFilmGrid_Watchlist(t *testing.T) {
	p, err := parseFilmGrid(readFixture(t, "watchlist_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Films) != 28 {
		t.Errorf("got %d films, want 28", len(p.Films))
	}
	if p.LastPage != 43 {
		t.Errorf("LastPage = %d, want 43", p.LastPage)
	}
	if p.Total != 1196 {
		t.Errorf("Total = %d, want 1196", p.Total)
	}
	for _, f := range p.Films {
		if f.Slug == "" || f.Name == "" {
			t.Errorf("incomplete film %+v", f)
		}
	}
}

func TestParseFilmGrid_EmptyWatchlist(t *testing.T) {
	p, err := parseFilmGrid(readFixture(t, "watchlist_empty.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Films) != 0 || p.Private || p.LastPage != 1 {
		t.Errorf("got %+v, want an empty public page", p)
	}
}

func TestParseFilmGrid_CustomList(t *testing.T) {
	p, err := parseFilmGrid(readFixture(t, "list_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Films) != 13 {
		t.Fatalf("got %d films, want 13", len(p.Films))
	}
	if p.LastPage != 1 {
		t.Errorf("LastPage = %d, want 1", p.LastPage)
	}
	slugs := make([]string, len(p.Films))
	for i, f := range p.Films {
		slugs[i] = f.Slug
	}
	for _, want := range []string{"dunkirk-2017", "inception", "the-dark-knight"} {
		if !slices.Contains(slugs, want) {
			t.Errorf("missing %q in %v", want, slugs)
		}
	}
}

func TestParseListsPage(t *testing.T) {
	p, err := parseListsPage(readFixture(t, "lists_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lists) == 0 {
		t.Fatal("no lists found")
	}
	if p.LastPage != 6 {
		t.Errorf("LastPage = %d, want 6", p.LastPage)
	}
	first := p.Lists[0]
	want := ListRef{Slug: "christopher-nolan-films-ranked", Name: "CHRISTOPHER NOLAN FILMS: RANKED", Count: 13}
	if first != want {
		t.Errorf("first list = %+v, want %+v", first, want)
	}
	for _, l := range p.Lists {
		if l.Slug == "" || l.Name == "" {
			t.Errorf("incomplete list %+v", l)
		}
	}
}

func TestParseFilm(t *testing.T) {
	f, err := parseFilm("parasite-2019", readFixture(t, "film_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("Title", f.Title, "Parasite")
	check("Year", f.Year, 2019)
	check("Runtime", f.Runtime, 133)
	check("IMDbID", f.IMDbID, "tt6751668")
	check("Rating", f.Rating, 4.51)
	check("URL", f.URL, "https://letterboxd.com/film/parasite-2019/")
	if !slices.Equal(f.Directors, []string{"Bong Joon Ho"}) {
		t.Errorf("Directors = %v", f.Directors)
	}
	if !slices.Equal(f.Genres, []string{"Thriller", "Comedy", "Drama"}) {
		t.Errorf("Genres = %v", f.Genres)
	}
	if f.RatingCount < 1_000_000 {
		t.Errorf("RatingCount = %d", f.RatingCount)
	}
	if f.Synopsis == "" || f.Poster == "" || f.Backdrop == "" {
		t.Errorf("missing synopsis/poster/backdrop: %+v", f)
	}
}

func TestIsChallenge(t *testing.T) {
	if !isChallenge(nil, readFixture(t, "cloudflare_challenge.html")) {
		t.Error("Cloudflare challenge page not detected")
	}
	if isChallenge(nil, readFixture(t, "films_page.html")) {
		t.Error("regular page detected as a challenge")
	}
}
