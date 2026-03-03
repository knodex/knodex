// Package logger provides structured logging with Kubernetes metadata support.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/provops-org/knodex/server/internal/config"
)

// Setup initializes the global logger with the provided configuration.
// It sets up JSON or text format, configures log levels, adds
// Kubernetes metadata (pod name, namespace), and enables sensitive data redaction.
func Setup(cfg *config.Log) *slog.Logger {
	level := parseLevel(cfg.Level)
	handler := createHandler(cfg.Format, level)

	// Wrap with redaction handler first (innermost)
	// This ensures sensitive data is redacted before any other processing
	handler = NewRedactionHandler(handler)

	// Add Kubernetes metadata as default attributes
	attrs := []slog.Attr{}
	if cfg.PodName != "" {
		attrs = append(attrs, slog.String("pod", cfg.PodName))
	}
	if cfg.Namespace != "" {
		attrs = append(attrs, slog.String("namespace", cfg.Namespace))
	}

	// If we have Kubernetes metadata, wrap the handler
	if len(attrs) > 0 {
		handler = &metadataHandler{
			handler: handler,
			attrs:   attrs,
		}
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// parseLevel converts a string log level to slog.Level
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// createHandler creates the appropriate slog.Handler based on format
func createHandler(format string, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch strings.ToLower(format) {
	case "text":
		return slog.NewTextHandler(os.Stdout, opts)
	case "json":
		fallthrough
	default:
		return slog.NewJSONHandler(os.Stdout, opts)
	}
}

// metadataHandler wraps an slog.Handler to add default attributes to all records
type metadataHandler struct {
	handler slog.Handler
	attrs   []slog.Attr
}

func (h *metadataHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *metadataHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add our metadata attributes to the record
	for _, attr := range h.attrs {
		r.AddAttrs(attr)
	}
	return h.handler.Handle(ctx, r)
}

func (h *metadataHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &metadataHandler{
		handler: h.handler.WithAttrs(attrs),
		attrs:   h.attrs,
	}
}

func (h *metadataHandler) WithGroup(name string) slog.Handler {
	return &metadataHandler{
		handler: h.handler.WithGroup(name),
		attrs:   h.attrs,
	}
}
