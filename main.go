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
	flag.Parse()

	om := omdb.New(*omdbKey)
	if om == nil {
		log.Print("OMDB_API_KEY not set: IMDb ratings are disabled")
	}
	srv := server.New(letterboxd.New(*cacheTTL), om, web.FS)

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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
