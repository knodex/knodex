package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/provops-org/knodex/server/internal/config"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo}, // defaults to info
		{"", slog.LevelInfo},        // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSetup_JSONFormat(t *testing.T) {
	cfg := &config.Log{
		Level:  "info",
		Format: "json",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Setup returned nil logger")
	}
}

func TestSetup_TextFormat(t *testing.T) {
	cfg := &config.Log{
		Level:  "debug",
		Format: "text",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Setup returned nil logger")
	}
}

func TestSetup_WithKubernetesMetadata(t *testing.T) {
	cfg := &config.Log{
		Level:     "info",
		Format:    "json",
		PodName:   "test-pod-abc123",
		Namespace: "test-namespace",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Setup returned nil logger")
	}

	// Capture log output
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})

	// Create metadata handler with attributes
	attrs := []slog.Attr{
		slog.String("pod", cfg.PodName),
		slog.String("namespace", cfg.Namespace),
	}
	metaHandler := &metadataHandler{handler: handler, attrs: attrs}
	testLogger := slog.New(metaHandler)

	testLogger.Info("test message", "key", "value")

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	// Verify Kubernetes metadata is present
	if logEntry["pod"] != "test-pod-abc123" {
		t.Errorf("Expected pod=test-pod-abc123, got %v", logEntry["pod"])
	}
	if logEntry["namespace"] != "test-namespace" {
		t.Errorf("Expected namespace=test-namespace, got %v", logEntry["namespace"])
	}
	if logEntry["msg"] != "test message" {
		t.Errorf("Expected msg=test message, got %v", logEntry["msg"])
	}
	if logEntry["key"] != "value" {
		t.Errorf("Expected key=value, got %v", logEntry["key"])
	}
}

func TestSetup_DefaultsToJSON(t *testing.T) {
	cfg := &config.Log{
		Level:  "info",
		Format: "unknown",
	}

	logger := Setup(cfg)
	if logger == nil {
		t.Fatal("Setup returned nil logger")
	}
}

func TestMetadataHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	attrs := []slog.Attr{slog.String("pod", "test-pod")}
	handler := &metadataHandler{handler: baseHandler, attrs: attrs}

	newHandler := handler.WithAttrs([]slog.Attr{slog.String("extra", "attr")})
	if newHandler == nil {
		t.Fatal("WithAttrs returned nil")
	}

	// Verify it's still a metadataHandler
	_, ok := newHandler.(*metadataHandler)
	if !ok {
		t.Fatal("WithAttrs should return a metadataHandler")
	}
}

func TestMetadataHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	attrs := []slog.Attr{slog.String("pod", "test-pod")}
	handler := &metadataHandler{handler: baseHandler, attrs: attrs}

	newHandler := handler.WithGroup("test-group")
	if newHandler == nil {
		t.Fatal("WithGroup returned nil")
	}

	// Verify it's still a metadataHandler
	_, ok := newHandler.(*metadataHandler)
	if !ok {
		t.Fatal("WithGroup should return a metadataHandler")
	}
}

func TestMain(m *testing.M) {
	// Ensure we don't pollute the global logger during tests
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	os.Exit(m.Run())
}
