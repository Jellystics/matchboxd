package letterboxd

import "errors"

// Sentinel errors returned by the client. Callers use errors.Is to map them
// to user-facing messages.
var (
	// ErrNotFound: Letterboxd answered 404 (unknown user, list or film).
	ErrNotFound = errors.New("letterboxd: not found")
	// ErrPrivate: the page exists but its content is hidden by its owner.
	ErrPrivate = errors.New("letterboxd: private")
	// ErrBlocked: Cloudflare challenge, 403 or 429 rate limit.
	ErrBlocked = errors.New("letterboxd: request blocked")
	// ErrUpstream: any other unexpected answer (5xx, network error, ...).
	ErrUpstream = errors.New("letterboxd: upstream error")
)
