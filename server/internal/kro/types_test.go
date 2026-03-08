package kro

import (
	"testing"

	"github.com/kubernetes-sigs/kro/api/v1alpha1"
)

func TestRGDConstants(t *testing.T) {
	if RGDGroup != v1alpha1.KRODomainName {
		t.Errorf("RGDGroup = %q, want %q", RGDGroup, v1alpha1.KRODomainName)
	}
	if RGDGroup != "kro.run" {
		t.Errorf("RGDGroup = %q, want %q", RGDGroup, "kro.run")
	}
	if RGDVersion != "v1alpha1" {
		t.Errorf("RGDVersion = %q, want %q", RGDVersion, "v1alpha1")
	}
	if RGDResource != "resourcegraphdefinitions" {
		t.Errorf("RGDResource = %q, want %q", RGDResource, "resourcegraphdefinitions")
	}
	if RGDKind != "ResourceGraphDefinition" {
		t.Errorf("RGDKind = %q, want %q", RGDKind, "ResourceGraphDefinition")
	}
}

func TestRGDGVR(t *testing.T) {
	gvr := RGDGVR()
	if gvr.Group != v1alpha1.KRODomainName {
		t.Errorf("RGDGVR().Group = %q, want %q", gvr.Group, v1alpha1.KRODomainName)
	}
	if gvr.Version != "v1alpha1" {
		t.Errorf("RGDGVR().Version = %q, want %q", gvr.Version, "v1alpha1")
	}
	if gvr.Resource != "resourcegraphdefinitions" {
		t.Errorf("RGDGVR().Resource = %q, want %q", gvr.Resource, "resourcegraphdefinitions")
	}
}

func TestAnnotationConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"CatalogAnnotation", CatalogAnnotation, "knodex.io/catalog"},
		{"DescriptionAnnotation", DescriptionAnnotation, "knodex.io/description"},
		{"TagsAnnotation", TagsAnnotation, "knodex.io/tags"},
		{"CategoryAnnotation", CategoryAnnotation, "knodex.io/category"},
		{"IconAnnotation", IconAnnotation, "knodex.io/icon"},
		{"VersionAnnotation", VersionAnnotation, "knodex.io/version"},
		{"TitleAnnotation", TitleAnnotation, "knodex.io/title"},
		{"DeploymentModesAnnotation", DeploymentModesAnnotation, "knodex.io/deployment-modes"},
	}
	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
		}
	}
}

func TestLabelConstants(t *testing.T) {
	if RGDProjectLabel != "knodex.io/project" {
		t.Errorf("RGDProjectLabel = %q, want %q", RGDProjectLabel, "knodex.io/project")
	}
	if RGDOrganizationLabel != "knodex.io/organization" {
		t.Errorf("RGDOrganizationLabel = %q, want %q", RGDOrganizationLabel, "knodex.io/organization")
	}
}
