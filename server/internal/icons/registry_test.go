// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package icons

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}
	if r == nil {
		t.Fatal("NewRegistry() returned nil registry")
	}
}

func TestRegistry_Get_KnownSlug(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	knownSlugs := []string{"argocd", "helm", "kubernetes", "prometheus", "grafana"}
	for _, slug := range knownSlugs {
		svg, ok := r.Get(slug)
		if !ok {
			t.Errorf("Get(%q) = false, want true", slug)
		}
		if svg == "" {
			t.Errorf("Get(%q) returned empty SVG", slug)
		}
	}
}

func TestRegistry_Get_UnknownSlug(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	_, ok := r.Get("nonexistent-icon-slug")
	if ok {
		t.Error("Get(nonexistent) = true, want false")
	}
}

func TestRegistry_Get_EmptySlug(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	_, ok := r.Get("")
	if ok {
		t.Error("Get(\"\") = true, want false")
	}
}

func TestRegistry_LoadFromConfigMaps_SingleOverride(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	customSVG := `<svg><title>Custom</title></svg>`
	r.LoadFromConfigMaps([]ConfigMapEntry{
		{Name: "my-icons", Data: map[string]string{"custom-app": customSVG}},
	}, logger)

	svg, ok := r.Get("custom-app")
	if !ok {
		t.Error("Get(custom-app) = false after LoadFromConfigMaps")
	}
	if svg != customSVG {
		t.Errorf("Get(custom-app) = %q, want %q", svg, customSVG)
	}
}

func TestRegistry_LoadFromConfigMaps_TwoConfigMapsNoConflict(t *testing.T) {
	r := &Registry{icons: make(map[string]string)}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	r.LoadFromConfigMaps([]ConfigMapEntry{
		{Name: "aaa-icons", Data: map[string]string{"icon-a": "<svg>A</svg>"}},
		{Name: "bbb-icons", Data: map[string]string{"icon-b": "<svg>B</svg>"}},
	}, logger)

	if _, ok := r.Get("icon-a"); !ok {
		t.Error("icon-a not found after load")
	}
	if _, ok := r.Get("icon-b"); !ok {
		t.Error("icon-b not found after load")
	}
}

func TestRegistry_LoadFromConfigMaps_SlugCollisionFirstAlphabeticallyWins(t *testing.T) {
	r := &Registry{icons: make(map[string]string)}

	// Capture warnings
	var warnMessages []string
	handler := &testLogHandler{warnings: &warnMessages}
	logger := slog.New(handler)

	r.LoadFromConfigMaps([]ConfigMapEntry{
		{Name: "aaa-icons", Data: map[string]string{"shared-slug": "<svg>from-aaa</svg>"}},
		{Name: "bbb-icons", Data: map[string]string{"shared-slug": "<svg>from-bbb</svg>"}},
	}, logger)

	svg, ok := r.Get("shared-slug")
	if !ok {
		t.Error("shared-slug not found after load")
	}
	if svg != "<svg>from-aaa</svg>" {
		t.Errorf("collision resolution: got %q, want <svg>from-aaa</svg>", svg)
	}
	if len(warnMessages) == 0 {
		t.Error("expected warning logged for slug collision, got none")
	}
}

func TestRegistry_LoadFromConfigMaps_CustomOverridesBuiltin(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	// Verify argocd exists in built-in set
	builtinSVG, ok := r.Get("argocd")
	if !ok {
		t.Fatal("argocd not found in built-in registry")
	}

	customSVG := `<svg><title>Custom ArgoCD</title></svg>`
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r.LoadFromConfigMaps([]ConfigMapEntry{
		{Name: "my-overrides", Data: map[string]string{"argocd": customSVG}},
	}, logger)

	got, ok := r.Get("argocd")
	if !ok {
		t.Fatal("argocd not found after LoadFromConfigMaps")
	}
	if got == builtinSVG {
		t.Error("custom icon did not override built-in — built-in SVG still returned")
	}
	if got != customSVG {
		t.Errorf("Get(argocd) = %q, want custom SVG %q", got, customSVG)
	}
}

// testLogHandler captures log records for testing.
type testLogHandler struct {
	warnings *[]string
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn {
		*h.warnings = append(*h.warnings, r.Message)
	}
	return nil
}
func (h *testLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *testLogHandler) WithGroup(_ string) slog.Handler      { return h }
