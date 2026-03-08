// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import "context"

// ViewsService defines the interface for view operations.
// This interface is used by the router to interact with the views feature
// without importing the enterprise-only views package.
type ViewsService interface {
	// IsEnabled returns true if views are configured.
	IsEnabled() bool

	// ListViews returns all configured views with RGD counts.
	ListViews(ctx context.Context) ViewList

	// GetView returns a specific view by slug.
	// Returns nil if not found.
	GetView(slug string) *View
}

// View represents a custom view configuration.
// This is a copy of the EE views.View struct to avoid
// importing the enterprise-only package.
type View struct {
	// Name is the display name shown in the sidebar and page header
	Name string `json:"name"`

	// Slug is the URL-safe identifier used in routing
	Slug string `json:"slug"`

	// Icon is the Lucide icon name to display in the sidebar
	Icon string `json:"icon"`

	// Category is the value to match against knodex.io/category annotation
	Category string `json:"category"`

	// Order determines the sidebar display order (lower values appear first)
	Order int `json:"order"`

	// Description is an optional description shown on the view page
	Description string `json:"description,omitempty"`

	// Count is the number of RGDs matching this view's category
	Count int `json:"count"`
}

// ViewList represents the response from the views API endpoint.
type ViewList struct {
	// Views is the list of configured views with counts
	Views []View `json:"views"`
}
