// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package logger provides structured logging with sensitive data redaction.
package logger

import (
	"context"
	"log/slog"
	"regexp"
	"sync"
)

// redactionPattern defines a pattern for redacting sensitive data
type redactionPattern struct {
	name    string
	pattern *regexp.Regexp
}

// Default patterns for common sensitive data
var (
	redactionPatterns = []redactionPattern{
		{
			name:    "GitHub Personal Access Token",
			pattern: regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
		},
		{
			name:    "GitHub OAuth Token",
			pattern: regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),
		},
		{
			name:    "GitHub User Token",
			pattern: regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`),
		},
		{
			name:    "GitHub Server Token",
			pattern: regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),
		},
		{
			name:    "GitHub Refresh Token",
			pattern: regexp.MustCompile(`ghr_[a-zA-Z0-9]{76}`),
		},
		{
			name:    "Bearer Token",
			pattern: regexp.MustCompile(`Bearer\s+[a-zA-Z0-9\-._~+/]+`),
		},
		{
			name:    "API Key",
			pattern: regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?token)\s*[:=]\s*['"]?[a-zA-Z0-9\-._~+/]{20,}['"]?`),
		},
	}
	redactionMutex sync.RWMutex
)

// redactionHandler wraps an slog.Handler to redact sensitive data
type redactionHandler struct {
	handler slog.Handler
}

// NewRedactionHandler creates a new handler that redacts sensitive data
func NewRedactionHandler(handler slog.Handler) slog.Handler {
	return &redactionHandler{handler: handler}
}

// Enabled implements slog.Handler
func (h *redactionHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler
func (h *redactionHandler) Handle(ctx context.Context, r slog.Record) error {
	// Redact message
	r.Message = RedactSensitiveData(r.Message)

	// Redact attributes
	var redactedAttrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		redactedAttrs = append(redactedAttrs, slog.Attr{
			Key:   a.Key,
			Value: RedactValue(a.Value),
		})
		return true
	})

	// Create new record with redacted data
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newRecord.AddAttrs(redactedAttrs...)

	return h.handler.Handle(ctx, newRecord)
}

// WithAttrs implements slog.Handler
func (h *redactionHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Redact attributes before passing to wrapped handler
	redactedAttrs := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		redactedAttrs[i] = slog.Attr{
			Key:   attr.Key,
			Value: RedactValue(attr.Value),
		}
	}
	return &redactionHandler{
		handler: h.handler.WithAttrs(redactedAttrs),
	}
}

// WithGroup implements slog.Handler
func (h *redactionHandler) WithGroup(name string) slog.Handler {
	return &redactionHandler{
		handler: h.handler.WithGroup(name),
	}
}

// RedactValue redacts sensitive data from an slog.Value
func RedactValue(v slog.Value) slog.Value {
	switch v.Kind() {
	case slog.KindString:
		return slog.StringValue(RedactSensitiveData(v.String()))
	case slog.KindGroup:
		attrs := v.Group()
		redactedAttrs := make([]slog.Attr, len(attrs))
		for i, attr := range attrs {
			redactedAttrs[i] = slog.Attr{
				Key:   attr.Key,
				Value: RedactValue(attr.Value),
			}
		}
		return slog.GroupValue(redactedAttrs...)
	default:
		return v
	}
}

// RedactSensitiveData redacts sensitive data from a string
func RedactSensitiveData(s string) string {
	redactionMutex.RLock()
	// Create a copy of the slice to avoid race on append
	patterns := make([]redactionPattern, len(redactionPatterns))
	copy(patterns, redactionPatterns)
	redactionMutex.RUnlock()

	result := s
	for _, pattern := range patterns {
		result = pattern.pattern.ReplaceAllString(result, "[REDACTED-"+pattern.name+"]")
	}
	return result
}
