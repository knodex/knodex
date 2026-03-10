// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package cookie

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		jwt        string
		maxAge     time.Duration
		cfg        Config
		wantSecure bool
		wantDomain string
		wantMaxAge int
	}{
		{
			name:       "sets HttpOnly cookie with Secure flag",
			jwt:        "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test",
			maxAge:     1 * time.Hour,
			cfg:        Config{Secure: true, Domain: "example.com"},
			wantSecure: true,
			wantDomain: "example.com",
			wantMaxAge: 3600,
		},
		{
			name:       "works without Secure flag for development",
			jwt:        "dev-token",
			maxAge:     30 * time.Minute,
			cfg:        Config{Secure: false, Domain: ""},
			wantSecure: false,
			wantDomain: "",
			wantMaxAge: 1800,
		},
		{
			name:       "handles short TTL",
			jwt:        "short-lived-token",
			maxAge:     30 * time.Second,
			cfg:        Config{Secure: true, Domain: "app.example.com"},
			wantSecure: true,
			wantDomain: "app.example.com",
			wantMaxAge: 30,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()

			SetSession(w, tt.jwt, tt.maxAge, tt.cfg)

			resp := w.Result()
			cookies := resp.Cookies()

			if len(cookies) != 1 {
				t.Fatalf("expected 1 cookie, got %d", len(cookies))
			}

			c := cookies[0]

			if c.Name != SessionCookieName {
				t.Errorf("cookie name = %q, want %q", c.Name, SessionCookieName)
			}
			if c.Value != tt.jwt {
				t.Errorf("cookie value = %q, want %q", c.Value, tt.jwt)
			}
			if c.Path != CookiePath {
				t.Errorf("cookie path = %q, want %q", c.Path, CookiePath)
			}
			if c.Domain != tt.wantDomain {
				t.Errorf("cookie domain = %q, want %q", c.Domain, tt.wantDomain)
			}
			if c.MaxAge != tt.wantMaxAge {
				t.Errorf("cookie MaxAge = %d, want %d", c.MaxAge, tt.wantMaxAge)
			}
			if !c.HttpOnly {
				t.Error("cookie must be HttpOnly")
			}
			if c.Secure != tt.wantSecure {
				t.Errorf("cookie Secure = %v, want %v", c.Secure, tt.wantSecure)
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("cookie SameSite = %v, want SameSiteStrictMode", c.SameSite)
			}
		})
	}
}

func TestClearSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        Config
		wantSecure bool
		wantDomain string
	}{
		{
			name:       "clears cookie with Max-Age=0 header (delete immediately)",
			cfg:        Config{Secure: true, Domain: "example.com"},
			wantSecure: true,
			wantDomain: "example.com",
		},
		{
			name:       "clears cookie without domain in development",
			cfg:        Config{Secure: false, Domain: ""},
			wantSecure: false,
			wantDomain: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()

			ClearSession(w, tt.cfg)

			resp := w.Result()
			cookies := resp.Cookies()

			if len(cookies) != 1 {
				t.Fatalf("expected 1 cookie, got %d", len(cookies))
			}

			c := cookies[0]

			if c.Name != SessionCookieName {
				t.Errorf("cookie name = %q, want %q", c.Name, SessionCookieName)
			}
			if c.Value != "" {
				t.Errorf("cookie value should be empty, got %q", c.Value)
			}
			if c.Path != CookiePath {
				t.Errorf("cookie path = %q, want %q", c.Path, CookiePath)
			}
			// Go's cookie parser returns MaxAge: -1 when the HTTP header contains
			// "Max-Age=0" (delete immediately). Go's http.Cookie{MaxAge: -1} produces
			// "Max-Age=0" in the Set-Cookie header. This is NOT the same as Go's
			// MaxAge: 0 which omits the Max-Age attribute entirely (session cookie).
			if c.MaxAge != -1 {
				t.Errorf("cookie MaxAge = %d, want -1 (parsed from Max-Age=0 header = delete immediately)", c.MaxAge)
			}
			if !c.HttpOnly {
				t.Error("cookie must be HttpOnly even when clearing")
			}
			if c.Secure != tt.wantSecure {
				t.Errorf("cookie Secure = %v, want %v", c.Secure, tt.wantSecure)
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("cookie SameSite = %v, want SameSiteStrictMode", c.SameSite)
			}
			if c.Domain != tt.wantDomain {
				t.Errorf("cookie domain = %q, want %q", c.Domain, tt.wantDomain)
			}
		})
	}
}

func TestSessionCookieName(t *testing.T) {
	t.Parallel()
	if SessionCookieName != "knodex_session" {
		t.Errorf("SessionCookieName = %q, want %q", SessionCookieName, "knodex_session")
	}
}

func TestCookiePath(t *testing.T) {
	t.Parallel()
	if CookiePath != "/api" {
		t.Errorf("CookiePath = %q, want %q", CookiePath, "/api")
	}
}
