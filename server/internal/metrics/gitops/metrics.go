// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package gitops provides Prometheus metrics for GitOps commit operations.
package gitops

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CommitDuration tracks the duration of GitOps commits
	// AC-METRIC-01: gitops_commit_duration_seconds histogram (labels: repo, success)
	CommitDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gitops_commit_duration_seconds",
			Help:    "Duration of GitOps commits to VCS providers",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repo", "success"},
	)

	// CommitErrors counts the total number of GitOps commit errors
	// AC-METRIC-02: gitops_commit_errors_total counter (labels: repo, error_type)
	CommitErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitops_commit_errors_total",
			Help: "Total number of GitOps commit errors",
		},
		[]string{"repo", "error_type"},
	)

	// RateLimitRemaining tracks the remaining VCS API rate limit
	// AC-METRIC-03: gitops_rate_limit_remaining gauge (labels: repo)
	RateLimitRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gitops_rate_limit_remaining",
			Help: "Remaining VCS API rate limit",
		},
		[]string{"repo"},
	)

	// CommitRetries counts the total number of GitOps commit retries
	// AC-METRIC-04: gitops_commit_retries_total counter (labels: repo, attempt)
	CommitRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitops_commit_retries_total",
			Help: "Total number of GitOps commit retries",
		},
		[]string{"repo", "attempt"},
	)
)

// Error type constants for metrics labels
const (
	ErrorTypeRateLimit    = "rate_limit"
	ErrorTypeServerError  = "server_error"
	ErrorTypeClientError  = "client_error"
	ErrorTypeNetwork      = "network"
	ErrorTypeTimeout      = "timeout"
	ErrorTypeIdempotent   = "idempotent_skip"
	ErrorTypeValidation   = "validation"
	ErrorTypeUnauthorized = "unauthorized"
)
