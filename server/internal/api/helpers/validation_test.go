// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationErrors(t *testing.T) {
	t.Run("NewValidationErrors creates empty map", func(t *testing.T) {
		errs := NewValidationErrors()
		assert.NotNil(t, errs)
		assert.Empty(t, errs)
		assert.False(t, errs.HasErrors())
	})

	t.Run("Add adds field error", func(t *testing.T) {
		errs := NewValidationErrors()
		errs.Add("name", "name is required")
		assert.True(t, errs.HasErrors())
		assert.Equal(t, "name is required", errs["name"])
	})

	t.Run("AddIndexed adds indexed field error", func(t *testing.T) {
		errs := NewValidationErrors()
		errs.AddIndexed("items", 0, "item is invalid")
		assert.True(t, errs.HasErrors())
		assert.Equal(t, "item is invalid", errs["items[0]"])
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := NewValidationErrors()
		errs.Add("name", "name is required")
		errs.Add("email", "invalid email format")
		errs.AddIndexed("tags", 2, "tag too long")

		assert.True(t, errs.HasErrors())
		assert.Len(t, errs, 3)
		assert.Equal(t, "name is required", errs["name"])
		assert.Equal(t, "invalid email format", errs["email"])
		assert.Equal(t, "tag too long", errs["tags[2]"])
	})
}

func TestValidationErrors_WriteResponse(t *testing.T) {
	t.Run("no errors returns false without writing", func(t *testing.T) {
		errs := NewValidationErrors()
		w := httptest.NewRecorder()

		result := errs.WriteResponse(w)

		assert.False(t, result)
		assert.Equal(t, 200, w.Code) // Default status code, nothing written
		assert.Empty(t, w.Body.String())
	})

	t.Run("with errors writes 400 response", func(t *testing.T) {
		errs := NewValidationErrors()
		errs.Add("name", "name is required")
		errs.Add("email", "invalid email format")
		w := httptest.NewRecorder()

		result := errs.WriteResponse(w)

		assert.True(t, result)
		assert.Equal(t, 400, w.Code)

		var resp struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "BAD_REQUEST", resp.Code)
		assert.Equal(t, "Validation failed", resp.Message)
		assert.Equal(t, "name is required", resp.Details["name"])
		assert.Equal(t, "invalid email format", resp.Details["email"])
	})
}
