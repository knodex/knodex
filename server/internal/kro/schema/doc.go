// Package schema provides a dual-source schema pipeline for building deploy forms.
//
// It extracts types and validation from CRDs (crd.go), parses RGD intent using
// KRO's simpleschema (rgd.go), and merges both sources with resource graph metadata
// into enriched FormSchema objects (enricher.go).
//
// Migrated from internal/schema/ as part of STORY-267.
package schema
