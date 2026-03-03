package schema

import (
	"io"
	"log/slog"
	"testing"

	"github.com/provops-org/knodex/server/internal/models"
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
