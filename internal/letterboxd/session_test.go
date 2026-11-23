package letterboxd

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

func TestSessionStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "session.json")
	u, err := url.Parse("https://letterboxd.com/")
	if err != nil {
		t.Fatal(err)
	}

	jar := tls_client.NewCookieJar()
	jar.SetCookies(u, []*fhttp.Cookie{
		{
			Name:     "__cf_bm",
			Value:    "abc123",
			Domain:   "letterboxd.com",
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			Expires:  time.Now().Add(time.Hour),
		},
		{
			// Expired cookies must not be restored.
			Name:    "gone",
			Value:   "x",
			Domain:  "letterboxd.com",
			Path:    "/",
			Expires: time.Now().Add(-time.Hour),
		},
	})

	store := &sessionStore{path: path, profile: "chrome_152"}
	if err := store.save(jar, false); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("session file not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("session file mode = %o, want 600", perm)
	}

	restored := tls_client.NewCookieJar()
	f := &fetcher{jar: restored, sess: &sessionStore{path: path, profile: "chrome_152"}}
	f.loadSession()

	cookies := restored.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "__cf_bm" || cookies[0].Value != "abc123" {
		t.Fatalf("restored cookies = %+v, want only __cf_bm=abc123", cookies)
	}

	// A session acquired with another browser profile must be ignored: its
	// cf_clearance is tied to the User-Agent.
	other := tls_client.NewCookieJar()
	f = &fetcher{jar: other, sess: &sessionStore{path: path, profile: "chrome_131"}}
	f.loadSession()
	if n := len(other.Cookies(u)); n != 0 {
		t.Errorf("restored %d cookies from a session of another profile, want 0", n)
	}
}

func TestSessionStore_ThrottledSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	jar := tls_client.NewCookieJar()
	u, _ := url.Parse("https://letterboxd.com/")
	jar.SetCookies(u, []*fhttp.Cookie{{Name: "a", Value: "1", Domain: "letterboxd.com", Path: "/"}})

	store := &sessionStore{path: path}
	if err := store.save(jar, true); err != nil {
		t.Fatal(err)
	}
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(u, []*fhttp.Cookie{{Name: "b", Value: "2", Domain: "letterboxd.com", Path: "/"}})
	if err := store.save(jar, true); err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Error("throttled save rewrote the file")
	}
}
