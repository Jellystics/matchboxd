# matchboxd

A small web app that randomly picks **a movie for two people to watch together**, based on their [Letterboxd](https://letterboxd.com) accounts.

<img width="3478" height="2089" alt="image" src="https://github.com/user-attachments/assets/50aa05f3-84f0-4927-96da-fd7da159d132" />

1. Enter two Letterboxd usernames.
2. For each person, pick a source: watchlist, watched films, liked films or a custom list.
3. Click **Pick a movie**: the app shows a movie that appears in **both** sources (poster, director, runtime, genres, synopsis, Letterboxd rating, and IMDb rating when an OMDb key is set).
4. **Pick another** draws again from the movies in common, without repeating the one just shown.

The interface is available in **English and French** (FR | EN switch in the top-right corner; defaults to the browser language and remembers your choice). Synopses stay in the language Letterboxd provides (English).

Everything ships as a single Go executable: the JSON API and the web interface (HTML/CSS/JS embedded with `go:embed`).

## Build and run

Requirements: Go ≥ 1.27 (or `nix develop`, thanks to the repository's `flake.nix`).

```sh
go build -o app .
./app
```

Then open <http://localhost:8080>.

With [Task](https://taskfile.dev) (provided by the flake), the binary also embeds its version (taken from `git describe`):

```sh
task build   # versioned static binary ./app
task run     # build and run
./app -version
```

Prebuilt binaries (Linux, macOS, Windows, FreeBSD; amd64, arm64, arm…) are available on the [GitHub Releases](https://github.com/Jellystics/matchboxd/releases) page.

### Configuration

| Environment variable | Flag         | Default      | Purpose                                              |
|----------------------|--------------|--------------|------------------------------------------------------|
| `PORT`               | `-port`      | `8080`       | HTTP port                                            |
| `OMDB_API_KEY`       | `-omdb-key`  | —            | OMDb key to show the IMDb rating (optional)          |
| —                    | `-cache-ttl` | `10m`        | How long fetched pages stay in the memory cache      |
| `MATCHBOXD_SESSION`  | `-session`   | user cache   | File where the Letterboxd session (cookies) is kept  |
| `MATCHBOXD_BROWSER`  | `-browser`   | `chrome_152` | Chrome version whose TLS fingerprint is imitated     |

The session file defaults to `~/Library/Caches/matchboxd/session.json` (macOS),
`~/.cache/matchboxd/session.json` (Linux) or `%LocalAppData%\matchboxd\session.json` (Windows).
Delete it to start over with a fresh session.

```sh
OMDB_API_KEY=abcd1234 PORT=3000 ./app
```

Without an OMDb key, the IMDb rating is simply hidden (the IMDb link is still shown).

## Getting a free OMDb key

1. Go to <https://www.omdbapi.com/apikey.aspx>.
2. Choose the **FREE** plan (1,000 requests per day), enter your email and a use case ("personal project" is fine).
3. Click the activation link sent to you by email.
4. Start the app with `OMDB_API_KEY=<your key> ./app`.

## Docker

Multi-architecture image published on GHCR: `linux/amd64`, `arm64`, `arm/v7`, `arm/v6`, `386`, `ppc64le`, `s390x`, `riscv64`.

```sh
docker run -d --name matchboxd -p 8080:8080 \
  -e OMDB_API_KEY=abcd1234 \
  -v matchboxd-data:/data \
  ghcr.io/jellystics/matchboxd:latest
```

- The image is built `FROM scratch` (~19 MB) and runs as a non-root user (`65532`).
- The `/data` volume keeps the Letterboxd session (`/data/session.json`) across restarts.
- Available tags: `latest` and `1.2.3` / `1.2` / `1` for releases, `edge` for the `main` branch, `sha-abc1234` for each commit.

To build the image locally: `task docker` (or `docker build -t matchboxd .`).

## Versioning and releases

The project follows [semantic versioning](https://semver.org) through `vX.Y.Z` git tags. The GitHub Actions workflow (`.github/workflows/ci.yml`) does the following:

| Event            | Tests | Docker image                             | GitHub release          |
|------------------|-------|------------------------------------------|-------------------------|
| Pull request     | ✅    | built, not published                     | —                       |
| Push to `main`   | ✅    | published as `edge` and `sha-…`          | —                       |
| Tag `v1.2.3`     | ✅    | published as `1.2.3`, `1.2`, `1`, `latest` | binaries + SHA-256 sums |

To publish a release:

```sh
task release -- v1.2.3     # creates the annotated tag and pushes it; CI does the rest
```

A tag with a suffix (`v1.2.3-rc.1`) produces a pre-release. `v0.x` releases don't get the `0` major Docker tag.

## Tests

```sh
go test ./...
```

The HTML parsing unit tests use real Letterboxd pages saved in `internal/letterboxd/testdata/`.

A live test queries the real website (public accounts `davidehrlich` and `karsten`) to check that parsing still works. It is disabled by default:

```sh
MATCHBOXD_LIVE=1 go test ./internal/letterboxd -run Live -v
```

If Letterboxd changes its HTML, save new pages in `testdata/` and adjust `parse.go`.

## How it works

- Letterboxd has no public API: the Go server reads the public pages (`/<user>/watchlist/`, `/<user>/films/`, `/<user>/likes/films/`, `/<user>/lists/`, `/<user>/list/<list>/`, `/film/<film>/`) and parses them with [goquery](https://github.com/PuerkitoBio/goquery).
- Requests imitate a Chrome tab: Chrome 152 TLS and HTTP/2 fingerprint ([tls-client](https://github.com/bogdanfinn/tls-client)), headers in browser order (`sec-ch-ua`, `Sec-Fetch-*`, `Referer`, `Accept-Language`…).
- Every run resumes the same session: a warm-up navigation opens the home page, then the Cloudflare cookies (`__cf_bm`, `cf_clearance`) are saved to the session file and reloaded on the next start.
- Every page of a source is fetched (a few requests in parallel, randomly spaced by 120 to 320 ms).
- Movies are compared by their Letterboxd *slug*, not by title.
- Parsed pages are kept in memory for a few minutes, so subsequent draws are instant.

### Known limitations

Letterboxd is protected by Cloudflare. The Chrome TLS fingerprint and the persistent session are enough for most pages (watchlists, watched films, liked films, lists and film pages go through in practice), but Cloudflare may still serve a **JavaScript challenge** depending on the network or IP. That challenge can only be solved by running JavaScript in a real browser: the app doesn't do that and shows a clear message instead ("Letterboxd is temporarily blocking our requests…"). For the lists index, it offers the lists from the first page along with a warning.

## Project structure

```
main.go                      entry point: configuration, HTTP server
internal/letterboxd/         Letterboxd client: requests, cache, HTML parsing
  testdata/                  saved Letterboxd pages used by the tests
internal/omdb/               IMDb rating through the OMDb API
internal/server/             JSON API (/api/sources, /api/pick) and static files
web/                         interface: index.html, style.css, app.js (embedded)
Dockerfile                   multi-architecture image (Go cross-compilation, scratch base)
Taskfile.yml                 versioned build, tests, Docker image, releases
.github/workflows/ci.yml     tests, GHCR images, releases
```

### API

- `GET /api/sources?user=<username>` → available sources for an account.
- `POST /api/pick` with `{"a":{"user":"…","source":"watchlist"},"b":{"user":"…","source":"list:<slug>"},"exclude":"<slug>"}` → `{"count": 126, "film": {…}, "imdb": {…}, "sizes": [1196, 632]}`.
- On error: `{"code": "user_not_found", "user": "…", "error": "…"}`. The interface translates the message from `code` (`user_not_found`, `list_not_found`, `private`, `blocked`, `timeout`, `upstream`, …).
