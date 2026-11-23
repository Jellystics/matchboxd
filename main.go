// Command matchboxd serves a small web app that picks a random movie in
// common between two Letterboxd accounts.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Jellystics/matchboxd/internal/letterboxd"
	"github.com/Jellystics/matchboxd/internal/omdb"
	"github.com/Jellystics/matchboxd/internal/server"
	"github.com/Jellystics/matchboxd/web"
)

func main() {
	port := flag.String("port", envOr("PORT", "8080"), "HTTP port (env PORT)")
	omdbKey := flag.String("omdb-key", os.Getenv("OMDB_API_KEY"), "OMDb API key for IMDb ratings (env OMDB_API_KEY, optional)")
	cacheTTL := flag.Duration("cache-ttl", 10*time.Minute, "how long Letterboxd pages stay in the memory cache")
	sessionFile := flag.String("session", defaultSessionFile(), "file where the Letterboxd browser session (cookies) is kept (env MATCHBOXD_SESSION)")
	browser := flag.String("browser", envOr("MATCHBOXD_BROWSER", "chrome_152"), "Chrome build whose TLS fingerprint is imitated, e.g. chrome_152 (env MATCHBOXD_BROWSER)")
	flag.Parse()

	om := omdb.New(*omdbKey)
	if om == nil {
		log.Print("OMDB_API_KEY not set: IMDb ratings are disabled")
	}
	lb := letterboxd.New(*cacheTTL, letterboxd.WithSessionFile(*sessionFile), letterboxd.WithBrowserProfile(*browser))
	defer lb.Close()
	srv := server.New(lb, om, web.FS)

	httpSrv := &http.Server{
		Addr:              ":" + *port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("matchboxd listening on http://localhost:%s", *port)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// defaultSessionFile returns where the cookie jar is stored between runs:
// MATCHBOXD_SESSION, or <user cache dir>/matchboxd/session.json.
func defaultSessionFile() string {
	if v := os.Getenv("MATCHBOXD_SESSION"); v != "" {
		return v
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "matchboxd", "session.json")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
