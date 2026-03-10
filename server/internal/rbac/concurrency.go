// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// MaxRetries for optimistic concurrency control
	MaxRetries = 5
	// RetryDelay between retry attempts
	RetryDelay = 100 * time.Millisecond
)

// RetryOnConflict retries an update operation when there's a conflict
// This implements optimistic concurrency control using Kubernetes ResourceVersion
func RetryOnConflict(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if this is a conflict error (optimistic locking failure)
		if apierrors.IsConflict(err) {
			lastErr = err
			// Wait before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(RetryDelay):
				// Continue to next attempt
				continue
			}
		}

		// Non-conflict error, fail immediately
		return err
	}

	// Exhausted all retries
	return fmt.Errorf("failed after %d attempts due to conflicts: %w", MaxRetries, lastErr)
}
