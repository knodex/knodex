// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import "net/http"

// SecurityHeaders adds security headers to HTTP responses
// This middleware implements OWASP recommended security headers for production deployments:
// - HSTS: Forces HTTPS connections
// - CSP: Prevents XSS and data injection attacks
// - X-Frame-Options: Prevents clickjacking
// - X-Content-Type-Options: Prevents MIME-sniffing attacks
// - X-XSS-Protection: Enables browser XSS protection (legacy browsers)
// - Referrer-Policy: Controls referrer information
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strict-Transport-Security: Force HTTPS for 1 year (31536000 seconds)
		// includeSubDomains: Apply to all subdomains
		// preload: Allow inclusion in browser HSTS preload lists
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Content-Security-Policy: Restrict resource loading to prevent XSS
		// default-src 'self': Only load resources from same origin by default
		// script-src 'self': Only execute scripts from same origin
		// style-src 'self' 'unsafe-inline': Allow inline styles (for React components)
		// img-src 'self' data: https:: Allow images from same origin, data URIs, and HTTPS URLs
		// font-src 'self': Only load fonts from same origin
		// connect-src 'self' ws: wss:: Allow connections to same origin and WebSocket endpoints (ws:// and wss://)
		// frame-ancestors 'none': Prevent embedding in frames (redundant with X-Frame-Options but more modern)
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self'; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'")

		// X-Frame-Options: Prevent clickjacking by disallowing page embedding
		// DENY: Never allow page to be embedded in frames/iframes
		w.Header().Set("X-Frame-Options", "DENY")

		// X-Content-Type-Options: Prevent MIME-sniffing attacks
		// nosniff: Browser must respect the Content-Type header
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection: Enable browser XSS filtering (legacy browsers)
		// 1; mode=block: Enable XSS filter and block page rendering if attack detected
		// Note: Modern browsers use CSP instead, but this provides defense-in-depth
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy: Control referrer information leakage
		// strict-origin-when-cross-origin: Send full URL for same-origin, only origin for cross-origin
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// X-Permitted-Cross-Domain-Policies: Prevent Adobe products from loading cross-domain content
		// none: Disallow cross-domain data loading
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

		// Permissions-Policy: Restrict browser feature access
		// Disable hardware APIs that a Kubernetes dashboard never needs
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), usb=(), payment=()")

		next.ServeHTTP(w, r)
	})
}
