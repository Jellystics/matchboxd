// Package server exposes the JSON API and serves the embedded web interface.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/Jellystics/matchboxd/internal/letterboxd"
	"github.com/Jellystics/matchboxd/internal/omdb"
)

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	lb   *letterboxd.Client
	omdb *omdb.Client // nil when no API key is configured
	web  fs.FS
}

func New(lb *letterboxd.Client, om *omdb.Client, web fs.FS) *Server {
	return &Server{lb: lb, omdb: om, web: web}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("POST /api/pick", s.handlePick)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("GET /", http.FileServerFS(s.web))
	return logRequests(mux)
}

// GET /api/sources?user=NAME
func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	if !letterboxd.ValidUsername(user) {
		writeError(w, http.StatusBadRequest, "Nom d’utilisateur invalide.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	sources, complete, err := s.lb.Sources(ctx, user)
	if err != nil {
		s.writeLetterboxdError(w, err, user, "")
		return
	}
	resp := map[string]any{"user": strings.ToLower(user), "sources": sources}
	if !complete {
		resp["warning"] = "Letterboxd a limité nos requêtes : certaines listes personnalisées peuvent manquer."
	}
	writeJSON(w, http.StatusOK, resp)
}

type pickSide struct {
	User   string `json:"user"`
	Source string `json:"source"`
}

type pickRequest struct {
	A       pickSide `json:"a"`
	B       pickSide `json:"b"`
	Exclude string   `json:"exclude"` // slug shown last, not to be drawn again
}

type pickResponse struct {
	Count int              `json:"count"`
	Film  *letterboxd.Film `json:"film,omitempty"`
	IMDb  *omdb.Rating     `json:"imdb,omitempty"`
	Sizes [2]int           `json:"sizes"` // number of films in each source
}

// POST /api/pick {"a":{"user","source"},"b":{...},"exclude":"slug"}
func (s *Server) handlePick(w http.ResponseWriter, r *http.Request) {
	var req pickRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requête invalide.")
		return
	}
	for _, side := range []pickSide{req.A, req.B} {
		if !letterboxd.ValidUsername(side.User) {
			writeError(w, http.StatusBadRequest, "Nom d’utilisateur invalide.")
			return
		}
		if _, err := letterboxd.SourcePath(side.User, side.Source); err != nil {
			writeError(w, http.StatusBadRequest, "Source invalide.")
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	// Load both sources concurrently.
	type result struct {
		films []letterboxd.FilmRef
		err   error
	}
	sides := []pickSide{req.A, req.B}
	results := make([]chan result, 2)
	for i, side := range sides {
		results[i] = make(chan result, 1)
		go func() {
			films, err := s.lb.Films(ctx, side.User, side.Source)
			results[i] <- result{films, err}
		}()
	}
	var sets [2][]letterboxd.FilmRef
	for i := range sides {
		res := <-results[i]
		if res.err != nil {
			s.writeLetterboxdError(w, res.err, sides[i].User, sides[i].Source)
			return
		}
		sets[i] = res.films
	}

	common := Intersect(sets[0], sets[1])
	resp := pickResponse{Count: len(common), Sizes: [2]int{len(sets[0]), len(sets[1])}}
	if len(common) == 0 {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	slug := PickRandom(common, req.Exclude)
	film, err := s.lb.Film(ctx, slug)
	if err != nil {
		s.writeLetterboxdError(w, err, "", "")
		return
	}
	resp.Film = &film
	if rating, err := s.omdb.Rating(ctx, film.IMDbID); err != nil {
		log.Printf("omdb %s: %v", film.IMDbID, err)
	} else {
		resp.IMDb = rating
	}
	writeJSON(w, http.StatusOK, resp)
}

// Intersect returns the slugs present in both a and b, in a's order.
func Intersect(a, b []letterboxd.FilmRef) []string {
	inB := make(map[string]bool, len(b))
	for _, f := range b {
		inB[f.Slug] = true
	}
	var out []string
	for _, f := range a {
		if inB[f.Slug] {
			out = append(out, f.Slug)
			delete(inB, f.Slug) // guard against duplicates in a
		}
	}
	return out
}

// PickRandom draws a random slug, avoiding exclude when another choice exists.
func PickRandom(slugs []string, exclude string) string {
	if len(slugs) == 1 {
		return slugs[0]
	}
	for {
		s := slugs[rand.IntN(len(slugs))]
		if s != exclude {
			return s
		}
	}
}

func (s *Server) writeLetterboxdError(w http.ResponseWriter, err error, user, source string) {
	switch {
	case errors.Is(err, letterboxd.ErrNotFound):
		switch {
		case source == "":
			if user != "" {
				writeError(w, http.StatusNotFound, "Utilisateur « "+user+" » introuvable sur Letterboxd.")
			} else {
				writeError(w, http.StatusNotFound, "Film introuvable sur Letterboxd.")
			}
		case strings.HasPrefix(source, letterboxd.SourceList+":"):
			writeError(w, http.StatusNotFound, "La liste de « "+user+" » est introuvable ou privée.")
		default:
			writeError(w, http.StatusNotFound, "Utilisateur « "+user+" » introuvable sur Letterboxd.")
		}
	case errors.Is(err, letterboxd.ErrPrivate):
		writeError(w, http.StatusForbidden, "Cette source de « "+user+" » est privée.")
	case errors.Is(err, letterboxd.ErrBlocked):
		writeError(w, http.StatusBadGateway, "Letterboxd bloque temporairement nos requêtes (protection anti-robots). Réessayez dans quelques minutes.")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "Letterboxd met trop de temps à répondre. Réessayez.")
	case errors.Is(err, context.Canceled):
		// Client went away; nothing useful to send.
	default:
		log.Printf("letterboxd error: %v", err)
		writeError(w, http.StatusBadGateway, "Impossible de joindre Letterboxd pour le moment.")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s?%s (%s)", r.Method, r.URL.Path, r.URL.RawQuery, time.Since(start).Round(time.Millisecond))
		}
	})
}
