package server

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Jellystics/matchboxd/internal/letterboxd"
)

func refs(slugs ...string) []letterboxd.FilmRef {
	out := make([]letterboxd.FilmRef, len(slugs))
	for i, s := range slugs {
		out[i] = letterboxd.FilmRef{Slug: s}
	}
	return out
}

func TestIntersect(t *testing.T) {
	got := Intersect(refs("a", "b", "c", "b", "d"), refs("d", "b", "x"))
	if !slices.Equal(got, []string{"b", "d"}) {
		t.Errorf("got %v, want [b d]", got)
	}
	if got := Intersect(refs("a"), refs("b")); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestPickRandom_NeverRepeatsExcluded(t *testing.T) {
	slugs := []string{"a", "b"}
	for range 200 {
		if PickRandom(slugs, "a") != "b" {
			t.Fatal("excluded slug was drawn")
		}
	}
	if PickRandom([]string{"only"}, "only") != "only" {
		t.Error("single film should still be returned")
	}
}

func TestHandlers_Validation(t *testing.T) {
	s := New(letterboxd.New(time.Minute), nil, fstest.MapFS{"index.html": {Data: []byte("<h1>ok</h1>")}})
	h := s.Handler()

	cases := []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/", "", http.StatusOK},
		{"GET", "/api/sources?user=../etc", "", http.StatusBadRequest},
		{"POST", "/api/pick", "not json", http.StatusBadRequest},
		{"POST", "/api/pick", `{"a":{"user":"x","source":"diary"},"b":{"user":"y","source":"films"}}`, http.StatusBadRequest},
		{"POST", "/api/pick", `{"a":{"user":"x","source":"list:../../a"},"b":{"user":"y","source":"films"}}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: status %d, want %d (%s)", tc.method, tc.path, rec.Code, tc.want, rec.Body)
		}
	}
}
