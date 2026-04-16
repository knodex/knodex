// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/response"
)

// ValidationErrors collects field validation errors
type ValidationErrors map[string]string

// NewValidationErrors creates an empty validation error map
func NewValidationErrors() ValidationErrors {
	return make(map[string]string)
}

// Add adds a validation error for a field
func (v ValidationErrors) Add(field, message string) {
	v[field] = message
}

// HasErrors returns true if there are any validation errors
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// WriteResponse writes a 400 response if there are errors.
// Returns true if errors were written, false if no errors.
func (v ValidationErrors) WriteResponse(w http.ResponseWriter) bool {
	if !v.HasErrors() {
		return false
	}
	response.BadRequest(w, "Validation failed", v)
	return true
}
