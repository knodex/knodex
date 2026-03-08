// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package cookie

import (
	"net/http"
	"time"
)

const (
	// SessionCookieName is the name of the HttpOnly session cookie.
	SessionCookieName = "knodex_session"

	// CookiePath restricts the cookie to API endpoints only.
	CookiePath = "/api"
)

// Config holds cookie configuration values.
type Config struct {
	Secure bool
	Domain string
}

// SetSession sets the knodex_session HttpOnly cookie with the given JWT.
func SetSession(w http.ResponseWriter, jwt string, maxAge time.Duration, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    jwt,
		Path:     CookiePath,
		Domain:   cfg.Domain,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSession clears the knodex_session cookie by emitting Max-Age=0 in the
// HTTP header, which tells the browser to delete the cookie immediately.
// In Go's net/http, MaxAge: -1 produces "Max-Age=0" in the Set-Cookie header.
// (MaxAge: 0 would omit the attribute entirely, creating a session cookie.)
func ClearSession(w http.ResponseWriter, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     CookiePath,
		Domain:   cfg.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteStrictMode,
	})
}
