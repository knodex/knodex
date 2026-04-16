// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// knodex-dex is a wrapper binary for Dex that generates Dex configuration from
// Knodex's SSO provider settings and manages the Dex lifecycle.
//
// This follows the ArgoCD pattern: the binary reads SSO config from the
// knodex-sso-providers ConfigMap and knodex-sso-secrets Secret, generates a
// complete Dex config YAML, and exec's the official `dex serve` command.
//
// Usage:
//
//	knodex-dex rundex    # Generate config and run Dex with hot-reload
//	knodex-dex gencfg    # Generate config and write to stdout or file
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/knodex/knodex/server/internal/dex"
	utilenv "github.com/knodex/knodex/server/internal/util/env"
)

const (
	defaultConfigPath = "/tmp/dex.yaml"
	dexBinary         = "dex"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: knodex-dex <rundex|gencfg> [flags]\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "rundex":
		if err := runDex(); err != nil {
			slog.Error("knodex-dex rundex failed", "error", err)
			os.Exit(1)
		}
	case "gencfg":
		if err := genCfg(); err != nil {
			slog.Error("knodex-dex gencfg failed", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: knodex-dex <rundex|gencfg>\n", os.Args[1])
		os.Exit(1)
	}
}

// runDex generates a Dex config from Knodex SSO settings, starts the Dex server,
// and watches for ConfigMap/Secret changes to hot-reload.
func runDex() error {
	slog.Info("knodex-dex rundex starting")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	k8sClient, namespace, err := buildK8sClient()
	if err != nil {
		return fmt.Errorf("building Kubernetes client: %w", err)
	}

	cfg := loadDexConfig()
	configPath := utilenv.GetString("DEX_CONFIG_PATH", defaultConfigPath)

	// Start watching SSO providers and generate initial config
	regenCh, err := dex.WatchAndRegenerate(ctx, k8sClient, namespace, cfg, configPath)
	if err != nil {
		return fmt.Errorf("starting config watcher: %w", err)
	}

	// Run Dex in a restart loop — regenerated config triggers restart
	for {
		slog.Info("starting dex serve", "config", configPath)

		cmd := exec.CommandContext(ctx, dexBinary, "serve", configPath) //nolint:gosec // dexBinary is validated at startup
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("starting dex process: %w", err)
		}

		// Wait for either: Dex process exit, config regen signal, or context cancel
		dexDone := make(chan error, 1)
		go func() {
			dexDone <- cmd.Wait()
		}()

		select {
		case err := <-dexDone:
			if ctx.Err() != nil {
				slog.Info("dex process stopped due to context cancellation")
				return nil
			}
			slog.Error("dex process exited unexpectedly", "error", err)
			// Brief pause before restart to avoid crash loops
			time.Sleep(2 * time.Second)

		case <-regenCh:
			slog.Info("SSO config changed, restarting dex")
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				// Wait for graceful shutdown (max 10s)
				select {
				case <-dexDone:
				case <-time.After(10 * time.Second):
					slog.Warn("dex did not stop gracefully, killing")
					_ = cmd.Process.Kill()
					<-dexDone
				}
			}

		case <-ctx.Done():
			slog.Info("shutting down knodex-dex")
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-dexDone:
				case <-time.After(10 * time.Second):
					_ = cmd.Process.Kill()
				}
			}
			return nil
		}
	}
}

// genCfg generates a Dex config and writes it to stdout or a file.
func genCfg() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	k8sClient, namespace, err := buildK8sClient()
	if err != nil {
		return fmt.Errorf("building Kubernetes client: %w", err)
	}

	cfg := loadDexConfig()

	data, err := dex.GenerateDexConfigYAML(ctx, k8sClient, namespace, cfg)
	if err != nil {
		return fmt.Errorf("generating Dex config: %w", err)
	}

	outputPath := utilenv.GetString("DEX_CONFIG_PATH", "")
	if outputPath != "" {
		if writeErr := os.WriteFile(outputPath, data, 0o600); writeErr != nil {
			return fmt.Errorf("writing config to %s: %w", outputPath, writeErr)
		}
		slog.Info("Dex config written", "path", outputPath)
		return nil
	}

	// Write to stdout
	_, err = os.Stdout.Write(data)
	return err
}

// loadDexConfig reads Dex configuration from environment variables.
func loadDexConfig() dex.Config {
	return dex.Config{
		IssuerURL:          utilenv.GetString("DEX_ISSUER_URL", "http://localhost:5556"),
		KnodexRedirectURL:  utilenv.GetString("DEX_KNODEX_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		KnodexClientSecret: utilenv.GetString("DEX_KNODEX_CLIENT_SECRET", ""),
		DisableTLS:         utilenv.GetBool("DEX_DISABLE_TLS", true),
		LogLevel:           utilenv.GetString("DEX_LOG_LEVEL", "info"),
	}
}

// buildK8sClient creates a Kubernetes client, preferring in-cluster config.
func buildK8sClient() (kubernetes.Interface, string, error) {
	namespace := utilenv.GetString("KNODEX_NAMESPACE", "knodex")

	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := utilenv.GetString("KUBECONFIG", os.Getenv("HOME")+"/.kube/config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, "", fmt.Errorf("building kubeconfig: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("creating Kubernetes client: %w", err)
	}

	return client, namespace, nil
}
