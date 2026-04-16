// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package vcs

import (
	"fmt"
	"time"
)

// RateLimitThreshold is the fraction of rate limit at which to start warning/pausing.
const RateLimitThreshold = 0.1

// RateLimitError represents a rate limit error from a VCS provider.
type RateLimitError struct {
	Remaining int
	Reset     time.Time
	WaitTime  time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit reached: %d remaining, resets at %s",
		e.Remaining, e.Reset.Format(time.RFC3339))
}
