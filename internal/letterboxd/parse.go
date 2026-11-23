package letterboxd

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// FilmRef is a film as it appears in a grid (watchlist, films, list, ...).
type FilmRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// filmPage is the parsed content of one page of a film grid.
type filmPage struct {
	Films    []FilmRef
	LastPage int
	Private  bool
	Total    int // announced film count, when the page shows one (watchlist)
}

// ListRef is a custom list of a user.
type ListRef struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type listsPage struct {
	Lists    []ListRef
	LastPage int
}

// Film holds the details shown on the result card.
type Film struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Year        int      `json:"year,omitempty"`
	Directors   []string `json:"directors"`
	Runtime     int      `json:"runtime,omitempty"` // minutes
	Genres      []string `json:"genres"`
	Synopsis    string   `json:"synopsis,omitempty"`
	Poster      string   `json:"poster,omitempty"`
	Backdrop    string   `json:"backdrop,omitempty"`
	Rating      float64  `json:"rating,omitempty"` // Letterboxd average, out of 5
	RatingCount int      `json:"ratingCount,omitempty"`
	IMDbID      string   `json:"imdbId,omitempty"`
	URL         string   `json:"url"`
}

var (
	pageNumRe  = regexp.MustCompile(`/page/(\d+)/?$`)
	imdbIDRe   = regexp.MustCompile(`tt\d{5,}`)
	durationRe = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?`)
	minsRe     = regexp.MustCompile(`(\d+)\s*(?:&nbsp;|\x{00a0}|\s)*mins?`)
	yearRe     = regexp.MustCompile(`\((\d{4})\)\s*$`)
	digitsRe   = regexp.MustCompile(`[\d,]+`)
)

// parseFilmGrid extracts the films and pagination info from a grid page
// (watchlist, watched films, liked films or a custom list).
func parseFilmGrid(body []byte) (filmPage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return filmPage{}, err
	}
	var p filmPage
	seen := map[string]bool{}
	doc.Find("li.griditem [data-item-slug], li.posteritem [data-item-slug]").Each(func(_ int, s *goquery.Selection) {
		slug := strings.TrimSpace(s.AttrOr("data-item-slug", ""))
		if slug == "" || seen[slug] {
			return
		}
		seen[slug] = true
		name := s.AttrOr("data-item-name", "")
		if name == "" {
			name = s.Find("img").AttrOr("alt", slug)
		}
		p.Films = append(p.Films, FilmRef{Slug: slug, Name: name})
	})
	p.LastPage = lastPage(doc)
	p.Total = parseCount(doc.Find(".js-watchlist-count").First().Text())

	hasGrid := doc.Find(".poster-grid, ul.poster-list, .js-watchlist-count").Length() > 0
	if len(p.Films) == 0 && !hasGrid {
		p.Private = looksPrivate(doc)
	}
	return p, nil
}

// parseListsPage extracts the custom lists from a /user/lists/ page.
func parseListsPage(body []byte) (listsPage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return listsPage{}, err
	}
	var p listsPage
	doc.Find("article.list-summary").Each(func(_ int, s *goquery.Selection) {
		a := s.Find("h2.name a").First()
		href := a.AttrOr("href", "")
		// href looks like /user/list/list-slug/
		parts := strings.Split(strings.Trim(href, "/"), "/")
		if len(parts) < 3 || parts[1] != "list" {
			return
		}
		l := ListRef{Slug: parts[2], Name: strings.TrimSpace(a.Text())}
		if v := s.Find(".content-reactions-strip .value").First().Text(); v != "" {
			l.Count = parseCount(v)
		}
		p.Lists = append(p.Lists, l)
	})
	p.LastPage = lastPage(doc)
	return p, nil
}

// parseFilm extracts the details of a film page.
func parseFilm(slug string, body []byte) (Film, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return Film{}, err
	}
	f := Film{Slug: slug, URL: BaseURL + "/film/" + slug + "/", Directors: []string{}, Genres: []string{}}

	// Most data comes from the schema.org JSON-LD block.
	var ld struct {
		Name        string   `json:"name"`
		Image       string   `json:"image"`
		Description string   `json:"description"`
		Duration    string   `json:"duration"`
		Genre       []string `json:"genre"`
		Director    []struct {
			Name string `json:"name"`
		} `json:"director"`
		AggregateRating *struct {
			RatingValue float64 `json:"ratingValue"`
			RatingCount int     `json:"ratingCount"`
		} `json:"aggregateRating"`
		ReleasedEvent []struct {
			StartDate string `json:"startDate"`
		} `json:"releasedEvent"`
	}
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		raw := s.Text()
		raw = strings.ReplaceAll(raw, "/* <![CDATA[ */", "")
		raw = strings.ReplaceAll(raw, "/* ]]> */", "")
		if json.Unmarshal([]byte(strings.TrimSpace(raw)), &ld) == nil && ld.Name != "" {
			return false
		}
		return true
	})

	f.Title = ld.Name
	if f.Title == "" {
		f.Title = strings.TrimSpace(doc.Find("h1.primaryname .name, h1.headline-1").First().Text())
	}
	f.Poster = ld.Image
	f.Synopsis = strings.TrimSpace(ld.Description)
	if f.Synopsis == "" {
		f.Synopsis = strings.TrimSpace(doc.Find(`meta[name="description"]`).AttrOr("content", ""))
	}
	if ld.Genre != nil {
		f.Genres = ld.Genre
	}
	for _, d := range ld.Director {
		if d.Name != "" {
			f.Directors = append(f.Directors, d.Name)
		}
	}
	if ld.AggregateRating != nil {
		f.Rating = ld.AggregateRating.RatingValue
		f.RatingCount = ld.AggregateRating.RatingCount
	}

	// Year: production info block, then og:title "Title (2019)", then JSON-LD.
	if y, err := strconv.Atoi(strings.TrimSpace(doc.Find(".releasedate a, .releasedate").First().Text())); err == nil {
		f.Year = y
	} else if m := yearRe.FindStringSubmatch(doc.Find(`meta[property="og:title"]`).AttrOr("content", "")); m != nil {
		f.Year, _ = strconv.Atoi(m[1])
	} else if len(ld.ReleasedEvent) > 0 && len(ld.ReleasedEvent[0].StartDate) >= 4 {
		f.Year, _ = strconv.Atoi(ld.ReleasedEvent[0].StartDate[:4])
	}

	// Runtime: ISO 8601 duration, falling back to the "133 mins" footer.
	if m := durationRe.FindStringSubmatch(ld.Duration); m != nil && (m[1] != "" || m[2] != "") {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		f.Runtime = h*60 + mi
	}
	footer := doc.Find("p.text-footer").First()
	if f.Runtime == 0 {
		if m := minsRe.FindStringSubmatch(footer.Text()); m != nil {
			f.Runtime, _ = strconv.Atoi(m[1])
		}
	}

	// IMDb ID from the "More at IMDb" link.
	doc.Find(`a[data-track-action="IMDb"], a[href*="imdb.com/title/"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if id := imdbIDRe.FindString(s.AttrOr("href", "")); id != "" {
			f.IMDbID = id
			return false
		}
		return true
	})

	// Backdrop image, when the film has one.
	if bd := doc.Find("#backdrop, [data-backdrop]").First(); bd.Length() > 0 {
		f.Backdrop = bd.AttrOr("data-backdrop2x", bd.AttrOr("data-backdrop", ""))
	}
	return f, nil
}

// lastPage returns the highest page number in the pagination block (1 when
// there is no pagination).
func lastPage(doc *goquery.Document) int {
	last := 1
	doc.Find(".paginate-pages a, .paginate-nextprev a").Each(func(_ int, s *goquery.Selection) {
		if m := pageNumRe.FindStringSubmatch(s.AttrOr("href", "")); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > last {
				last = n
			}
		}
	})
	return last
}

// looksPrivate detects pages whose content has been hidden by their owner.
func looksPrivate(doc *goquery.Document) bool {
	content := doc.Find("#content").Clone()
	content.Find("script, style, noscript").Remove()
	text := strings.ToLower(content.Text())
	return strings.Contains(text, "is private") || strings.Contains(text, "are private") ||
		strings.Contains(text, "has been hidden") || strings.Contains(text, "not public")
}

func parseCount(s string) int {
	m := digitsRe.FindString(s)
	n, _ := strconv.Atoi(strings.ReplaceAll(m, ",", ""))
	return n
}
