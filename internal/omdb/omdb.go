// Package omdb fetches IMDb ratings from the OMDb API (https://www.omdbapi.com).
package omdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Rating is the IMDb rating of a title.
type Rating struct {
	Value float64 `json:"value"` // out of 10
	Votes int     `json:"votes"`
}

// Client queries OMDb. A nil *Client is valid and always returns no rating,
// which is how the app behaves when no API key is configured.
type Client struct {
	key     string
	baseURL string
	http    *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	r       *Rating
	expires time.Time
}

// New returns a client, or nil if key is empty.
func New(key string) *Client {
	if key == "" {
		return nil
	}
	return &Client{
		key:     key,
		baseURL: "https://www.omdbapi.com/",
		http:    &http.Client{Timeout: 6 * time.Second},
		cache:   map[string]cached{},
	}
}

// Rating returns the IMDb rating for an IMDb ID (tt1234567). It returns
// (nil, nil) when the title has no rating.
func (c *Client) Rating(ctx context.Context, imdbID string) (*Rating, error) {
	if c == nil || imdbID == "" {
		return nil, nil
	}
	c.mu.Lock()
	if e, ok := c.cache[imdbID]; ok && time.Now().Before(e.expires) {
		c.mu.Unlock()
		return e.r, nil
	}
	c.mu.Unlock()

	q := url.Values{"i": {imdbID}, "apikey": {c.key}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("omdb: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Response   string `json:"Response"`
		Error      string `json:"Error"`
		IMDbRating string `json:"imdbRating"`
		IMDbVotes  string `json:"imdbVotes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("omdb: HTTP %d: %w", resp.StatusCode, err)
	}
	if body.Response != "True" {
		return nil, errors.New("omdb: " + body.Error)
	}

	var r *Rating
	if v, err := strconv.ParseFloat(body.IMDbRating, 64); err == nil {
		votes, _ := strconv.Atoi(strings.ReplaceAll(body.IMDbVotes, ",", ""))
		r = &Rating{Value: v, Votes: votes}
	}
	c.mu.Lock()
	c.cache[imdbID] = cached{r: r, expires: time.Now().Add(12 * time.Hour)}
	c.mu.Unlock()
	return r, nil
}
