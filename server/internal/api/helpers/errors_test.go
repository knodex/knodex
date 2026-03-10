// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/knodex/knodex/server/internal/rbac"
)

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "rbac.ErrNotFound",
			err:      rbac.ErrNotFound,
			expected: true,
		},
		{
			name:     "rbac.ErrProjectNotFound",
			err:      rbac.ErrProjectNotFound,
			expected: true,
		},
		{
			name:     "wrapped rbac.ErrNotFound",
			err:      errors.Join(errors.New("context"), rbac.ErrNotFound),
			expected: true,
		},
		{
			name:     "string contains 'not found'",
			err:      errors.New("resource not found"),
			expected: true,
		},
		{
			name:     "string contains 'Not Found' (case insensitive)",
			err:      errors.New("Resource Not Found"),
			expected: true,
		},
		{
			name:     "string contains 'notfound'",
			err:      errors.New("error: notfound"),
			expected: true,
		},
		{
			name:     "kubernetes not found",
			err:      errors.New(`pods "my-pod" not found`),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "empty error",
			err:      errors.New(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
