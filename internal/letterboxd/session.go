package letterboxd

import (
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

// sessionStore persists the browser cookie jar so that Cloudflare's session
// cookies (__cf_bm, cf_clearance) survive restarts: each run continues the
// same visit instead of looking like a brand-new visitor.
type sessionStore struct {
	path    string
	profile string // browser profile the cookies were acquired with

	mu   sync.Mutex
	last time.Time
}

// sessionSnapshot is the on-disk format of the cookie jar.
type sessionSnapshot struct {
	Version int                        `json:"version"`
	Profile string                     `json:"profile,omitempty"`
	SavedAt time.Time                  `json:"savedAt"`
	Cookies map[string][]sessionCookie `json:"cookies"`
}

type sessionCookie struct {
	Name     string         `json:"name"`
	Value    string         `json:"value"`
	Domain   string         `json:"domain,omitempty"`
	Path     string         `json:"path,omitempty"`
	Expires  time.Time      `json:"expires,omitempty"`
	MaxAge   int            `json:"maxAge,omitempty"`
	Secure   bool           `json:"secure,omitempty"`
	HttpOnly bool           `json:"httpOnly,omitempty"`
	SameSite fhttp.SameSite `json:"sameSite,omitempty"`
}

func (f *fetcher) loadSession() {
	snap, err := readSession(f.sess.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return
	case err != nil:
		log.Printf("letterboxd: cannot read session %s: %v", f.sess.path, err)
		return
	}

	// cf_clearance is bound to the User-Agent (hence to the browser profile):
	// a session acquired with another profile is worthless.
	if snap.Profile != "" && snap.Profile != f.sess.profile {
		log.Printf("letterboxd: discarding session acquired with browser profile %q (now %q)", snap.Profile, f.sess.profile)
		return
	}

	now := time.Now()
	restored := 0
	for host, cookies := range snap.Cookies {
		u := &url.URL{Scheme: "https", Host: host, Path: "/"}
		var list []*fhttp.Cookie
		for _, c := range cookies {
			if !c.Expires.IsZero() && c.Expires.Before(now) {
				continue
			}
			list = append(list, &fhttp.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  c.Expires,
				MaxAge:   c.MaxAge,
				Secure:   c.Secure,
				HttpOnly: c.HttpOnly,
				SameSite: c.SameSite,
			})
		}
		if len(list) > 0 {
			f.jar.SetCookies(u, list)
			restored += len(list)
		}
	}
	if restored > 0 {
		log.Printf("letterboxd: restored %d session cookies from %s", restored, f.sess.path)
	}
}

// persistSession writes the cookie jar to disk. When throttled is true, the
// write happens at most once per sessionSaveInterval.
func (f *fetcher) persistSession(throttled bool) {
	if f.sess == nil {
		return
	}
	if err := f.sess.save(f.jar, throttled); err != nil {
		log.Printf("letterboxd: cannot save session %s: %v", f.sess.path, err)
	}
}

func (s *sessionStore) save(jar tls_client.CookieJar, throttled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if throttled && time.Since(s.last) < sessionSaveInterval {
		return nil
	}
	s.last = time.Now()

	snap := sessionSnapshot{
		Version: 1,
		Profile: s.profile,
		SavedAt: s.last,
		Cookies: make(map[string][]sessionCookie),
	}
	for host, cookies := range jar.GetAllCookies() {
		for _, c := range cookies {
			snap.Cookies[host] = append(snap.Cookies[host], sessionCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  c.Expires,
				MaxAge:   c.MaxAge,
				Secure:   c.Secure,
				HttpOnly: c.HttpOnly,
				SameSite: c.SameSite,
			})
		}
	}
	if len(snap.Cookies) == 0 {
		return nil
	}
	return writeSession(s.path, snap)
}

func readSession(path string) (sessionSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionSnapshot{}, err
	}
	var snap sessionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return sessionSnapshot{}, err
	}
	return snap, nil
}

// writeSession replaces the session file atomically, so a crash never leaves
// a truncated file behind.
func writeSession(path string, snap sessionSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".session-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
