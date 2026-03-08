// Mock OIDC Server for E2E testing
//
// This server implements a minimal OIDC provider for deterministic E2E testing.
// It should NOT be used in production.
//
// Usage:
//
//	./mock-oidc --port 8081
//	./mock-oidc --port 8081 --issuer http://mock-oidc.knodex-test.svc.cluster.local:8081
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/knodex/knodex/server/test/mocks/oidc"
)

func main() {
	var (
		port         int
		issuerURL    string
		clientID     string
		clientSecret string
	)

	flag.IntVar(&port, "port", 8081, "Port to listen on")
	flag.StringVar(&issuerURL, "issuer", "", "Issuer URL (default: http://localhost:<port>)")
	flag.StringVar(&clientID, "client-id", "test-client-id", "Expected client_id")
	flag.StringVar(&clientSecret, "client-secret", "test-client-secret", "Expected client_secret")
	flag.Parse()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create server with options
	opts := []oidc.Option{
		oidc.WithPort(port),
		oidc.WithClientCredentials(clientID, clientSecret),
	}
	if issuerURL != "" {
		opts = append(opts, oidc.WithIssuerURL(issuerURL))
	}

	server, err := oidc.NewServer(opts...)
	if err != nil {
		logger.Error("Failed to create mock OIDC server", "error", err)
		os.Exit(1)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		logger.Error("Failed to start mock OIDC server", "error", err)
		os.Exit(1)
	}

	logger.Info("Mock OIDC server started",
		"port", port,
		"issuer", server.IssuerURL(),
		"client_id", clientID,
	)

	// Log available test users
	for _, user := range oidc.DefaultTestUsers() {
		logger.Info("Test user available",
			"email", user.Email,
			"groups", user.Groups,
		)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down mock OIDC server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Mock OIDC server stopped")
}
