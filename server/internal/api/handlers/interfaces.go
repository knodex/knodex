// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"

	krograph "github.com/kubernetes-sigs/kro/pkg/graph"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knodex/knodex/server/internal/models"
)

// RGDReader is the read-only interface of the RGD watcher used by handlers.
// Defining it here (consumer side) keeps handlers independent of the watcher
// implementation and makes handler unit-testing possible with simple mocks.
//
// GetRGDByKind is included to satisfy kroschema.RGDProvider (used by
// EnrichSchema/EnrichSchemaFromResources in SchemaHandler).
type RGDReader interface {
	GetRGD(namespace, name string) (*models.CatalogRGD, bool)
	GetRGDByName(name string) (*models.CatalogRGD, bool)
	GetRGDByKind(kind string) (*models.CatalogRGD, bool)
	GetGraph(namespace, name string) *krograph.Graph
}

// InstanceReader is the read/write interface of the instance tracker used by handlers.
type InstanceReader interface {
	ListInstances(opts models.InstanceListOptions) models.InstanceList
	GetInstance(group, namespace, kind, name string) (*models.Instance, bool)
	CountFilteredInstances(filter func(*models.Instance) bool) int
	DeleteInstance(ctx context.Context, namespace, name, apiVersion, kind string) error
	ResolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error)
}

// SchemaExtractor is the schema extraction interface used by SchemaHandler.
type SchemaExtractor interface {
	ExtractSchema(ctx context.Context, rgd *models.CatalogRGD) (*models.FormSchema, error)
	InvalidateCache(namespace, name string)
}
