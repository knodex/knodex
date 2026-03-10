// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package swagger

import (
	"embed"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/response"
)

//go:embed openapi.yaml
var openapiFS embed.FS

// swaggerUIVersion is the Swagger UI version loaded from CDN.
// Pinned to a specific version to reduce supply chain risk.
const swaggerUIVersion = "5.18.2"

// swaggerCDN is the CDN origin used to load Swagger UI assets.
const swaggerCDN = "https://cdn.jsdelivr.net"

// swaggerCSP is a Content-Security-Policy tailored for the Swagger UI page.
// It relaxes the global CSP only for /swagger/ to allow loading the Swagger UI
// bundle from the jsdelivr CDN. The "Try it out" feature needs connect-src 'self'
// to call the API. This CSP is applied only to the Swagger HTML page, not to
// other routes.
const swaggerCSP = "default-src 'self'; " +
	"script-src 'self' " + swaggerCDN + "; " +
	"style-src 'self' 'unsafe-inline' " + swaggerCDN + "; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' " + swaggerCDN + "; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'"

// swaggerHTML is the HTML page that loads Swagger UI from CDN and points it
// at the embedded openapi.yaml served at /swagger/openapi.yaml.
const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Knodex API Documentation</title>
  <link rel="stylesheet" href="` + swaggerCDN + `/npm/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui.css" />
  <style>
    body { margin: 0; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="` + swaggerCDN + `/npm/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/swagger/openapi.yaml",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIBundle.SwaggerUIStandalonePreset
      ],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`

// Handler returns an http.Handler that serves Swagger UI at the given prefix.
// The prefix should be "/swagger/" (with trailing slash).
//
// Security: The Swagger HTML page overrides the global CSP to allow loading
// the Swagger UI bundle from cdn.jsdelivr.net. The openapi.yaml endpoint
// inherits the global CSP (no override needed for YAML).
//
// Routes served:
//   - GET /swagger/          → Swagger UI HTML page (with relaxed CSP)
//   - GET /swagger/index.html → same as above
//   - GET /swagger/openapi.yaml → embedded OpenAPI spec
func Handler(prefix string) http.Handler {
	mux := http.NewServeMux()

	// Serve the Swagger UI HTML page with a tailored CSP
	mux.HandleFunc("GET "+prefix, func(w http.ResponseWriter, r *http.Request) {
		// Only serve the index at the exact prefix path
		if r.URL.Path != prefix && r.URL.Path != prefix+"index.html" {
			http.NotFound(w, r)
			return
		}
		// Override the global CSP to allow CDN resources for Swagger UI
		w.Header().Set("Content-Security-Policy", swaggerCSP)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(swaggerHTML)) //nolint:errcheck
	})

	// Serve the embedded OpenAPI spec
	mux.HandleFunc("GET "+prefix+"openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		data, err := openapiFS.ReadFile("openapi.yaml")
		if err != nil {
			response.InternalError(w, "openapi.yaml not found")
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data) //nolint:errcheck
	})

	return mux
}
