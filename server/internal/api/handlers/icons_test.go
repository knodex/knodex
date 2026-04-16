// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/icons"
)

func newTestIconsHandler(t *testing.T) *IconsHandler {
	t.Helper()
	r, err := icons.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	return NewIconsHandler(r)
}

func TestIconsHandler_GetIcon_HappyPath(t *testing.T) {
	h := newTestIconsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/icons/argocd", nil)
	req.SetPathValue("slug", "argocd")
	w := httptest.NewRecorder()

	h.GetIcon(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want public, max-age=86400", cc)
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty SVG body")
	}
}

func TestIconsHandler_GetIcon_NotFound(t *testing.T) {
	h := newTestIconsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/icons/nonexistent-icon", nil)
	req.SetPathValue("slug", "nonexistent-icon")
	w := httptest.NewRecorder()

	h.GetIcon(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIconsHandler_GetIcon_InvalidSlug(t *testing.T) {
	h := newTestIconsHandler(t)

	// These slugs fail the ^[a-z0-9-]+$ validation when set as path values.
	// Use a fixed valid URL and override PathValue to test handler logic only.
	invalidSlugs := []string{
		"../traversal",
		"icon_with_underscores",
		"UPPERCASE",
		"icon.dot",
		"",
	}

	for _, slug := range invalidSlugs {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/icons/placeholder", nil)
		req.SetPathValue("slug", slug)
		w := httptest.NewRecorder()

		h.GetIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("slug %q: status = %d, want %d", slug, w.Code, http.StatusBadRequest)
		}
	}
}

func TestIconsHandler_GetIcon_AllBuiltinSlugs(t *testing.T) {
	h := newTestIconsHandler(t)

	requiredSlugs := []string{
		"argocd", "helm", "kubernetes", "prometheus", "grafana",
		"tekton", "flux", "crossplane", "docker", "github",
		"gitlab", "amazonaws", "microsoftazure", "googlecloud",
		"vault", "keycloak", "postgresql", "redis", "kafka", "nginx",
	}

	for _, slug := range requiredSlugs {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/icons/"+slug, nil)
		req.SetPathValue("slug", slug)
		w := httptest.NewRecorder()

		h.GetIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("slug %q: status = %d, want 200", slug, w.Code)
		}
	}
}
