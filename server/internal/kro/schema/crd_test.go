// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	fakeapiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knodex/knodex/server/internal/models"
)

// testLogger returns a no-op logger for tests
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestExtractCRDInfo(t *testing.T) {
	extractor := &Extractor{
		logger: testLogger(),
	}

	tests := []struct {
		name        string
		rgd         *models.CatalogRGD
		wantGroup   string
		wantKind    string
		wantVersion string
		wantErr     bool
	}{
		{
			name: "pre-computed APIVersion with group/version",
			rgd: &models.CatalogRGD{
				Name:       "aso-credential",
				APIVersion: "kro.run/v1alpha1",
				Kind:       "ASOCredential",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
						"kind":       "ASOCredential",
					},
				},
			},
			wantGroup:   "kro.run",
			wantKind:    "ASOCredential",
			wantVersion: "v1alpha1",
		},
		{
			name: "short apiVersion in rawSpec but pre-computed has group",
			rgd: &models.CatalogRGD{
				Name:       "simple-app",
				APIVersion: "kro.run/v1alpha1",
				Kind:       "SimpleApp",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
						"kind":       "SimpleApp",
					},
				},
			},
			wantGroup:   "kro.run",
			wantKind:    "SimpleApp",
			wantVersion: "v1alpha1",
		},
		{
			name: "full apiVersion in schema",
			rgd: &models.CatalogRGD{
				Name:       "some-rgd",
				APIVersion: "custom.example.com/v1beta1",
				Kind:       "MyResource",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "custom.example.com/v1beta1",
						"kind":       "MyResource",
					},
				},
			},
			wantGroup:   "custom.example.com",
			wantKind:    "MyResource",
			wantVersion: "v1beta1",
		},
		{
			name: "fallback to rawSpec when pre-computed empty",
			rgd: &models.CatalogRGD{
				Name: "fallback-rgd",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "example.io/v1",
						"kind":       "FallbackKind",
					},
				},
			},
			wantGroup:   "example.io",
			wantKind:    "FallbackKind",
			wantVersion: "v1",
		},
		{
			name: "rawSpec with separate group field",
			rgd: &models.CatalogRGD{
				Name: "group-field-rgd",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"group":   "explicit.group.io",
						"kind":    "GroupKind",
						"version": "v2",
					},
				},
			},
			wantGroup:   "explicit.group.io",
			wantKind:    "GroupKind",
			wantVersion: "v2",
		},
		{
			name: "missing group fails",
			rgd: &models.CatalogRGD{
				Name: "no-group",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
						"kind":       "NoGroupKind",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing kind fails",
			rgd: &models.CatalogRGD{
				Name:       "no-kind",
				APIVersion: "kro.run/v1alpha1",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "nil rawSpec with pre-computed values",
			rgd: &models.CatalogRGD{
				Name:       "no-rawspec",
				APIVersion: "kro.run/v1alpha1",
				Kind:       "NoRawSpecKind",
			},
			wantGroup:   "kro.run",
			wantKind:    "NoRawSpecKind",
			wantVersion: "v1alpha1",
		},
		{
			name: "nil rawSpec without pre-computed values fails",
			rgd: &models.CatalogRGD{
				Name: "empty-rgd",
			},
			wantErr: true,
		},
		{
			name: "default version when not specified",
			rgd: &models.CatalogRGD{
				Name: "no-version",
				Kind: "NoVersionKind",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"group": "example.io",
						"kind":  "NoVersionKind",
					},
				},
			},
			wantGroup:   "example.io",
			wantKind:    "NoVersionKind",
			wantVersion: "v1alpha1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, kind, version, err := extractor.extractCRDInfo(tt.rgd)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if group != tt.wantGroup {
				t.Errorf("group = %q, want %q", group, tt.wantGroup)
			}
			if kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", kind, tt.wantKind)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
		})
	}
}

func TestDegradedCacheExpiresFaster(t *testing.T) {
	// Use a fake client with NO CRDs — ExtractSchema will get a 404
	fakeClient := fakeapiext.NewSimpleClientset()
	extractor := NewExtractorWithClient(fakeClient)

	rgd := &models.CatalogRGD{
		Name:       "test-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1alpha1",
		Kind:       "TestApp",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "example.com/v1alpha1",
				"kind":       "TestApp",
				"spec": map[string]interface{}{
					"name": "string",
				},
			},
		},
	}

	// First call: triggers CRD lookup → 404 → cached with short TTL
	_, err := extractor.ExtractSchema(context.Background(), rgd)
	if err == nil {
		t.Fatal("expected error for missing CRD")
	}

	cacheKey := "default/test-rgd"
	extractor.cacheMu.RLock()
	cached, ok := extractor.cache[cacheKey]
	extractor.cacheMu.RUnlock()

	if !ok {
		t.Fatal("expected cache entry after ExtractSchema")
	}
	if !cached.Degraded {
		t.Error("cache entry should be marked as degraded for 404")
	}

	// Verify degraded entry expires within ~30s (not 5min)
	timeUntilExpiry := time.Until(cached.ExpiresAt)
	if timeUntilExpiry > 35*time.Second {
		t.Errorf("degraded cache TTL should be ~30s, got %v", timeUntilExpiry)
	}
	if timeUntilExpiry > 5*time.Minute-30*time.Second {
		t.Errorf("degraded cache TTL (%v) is too close to normal TTL (5min)", timeUntilExpiry)
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "generic error",
			err:  fmt.Errorf("something went wrong"),
			want: false,
		},
		{
			name: "wrapped not-found error",
			err: fmt.Errorf("CRD not found: test: %w",
				apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "test")),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFoundError(tt.err)
			if got != tt.want {
				t.Errorf("IsNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simpleapp", "simpleapps"},
		{"asocredential", "asocredentials"},
		{"policy", "policys"},
		{"address", "addresss"},
		{"status", "statuss"},
		{"deployment", "deployments"},
		{"ingress", "ingresss"},
		{"alzhubaddongateway", "alzhubaddongateways"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := pluralize(tt.input)
			if got != tt.want {
				t.Errorf("pluralize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
