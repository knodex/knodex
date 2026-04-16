// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package dex

import (
	"context"
	"fmt"
	"log/slog"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/sso"
	"github.com/knodex/knodex/server/internal/util/rand"
)

const (
	// DefaultHTTPPort is the default HTTP port for Dex.
	DefaultHTTPPort = 5556
	// DefaultGRPCPort is the default gRPC port for Dex.
	DefaultGRPCPort = 5557
	// DefaultTelemetryPort is the default telemetry port for Dex.
	DefaultTelemetryPort = 5558

	// KnodexClientID is the static client ID for Knodex itself.
	KnodexClientID = "knodex"
	// KnodexClientName is the display name for the Knodex static client.
	KnodexClientName = "Knodex"
)

// Config holds the parameters for generating a Dex configuration.
type Config struct {
	// IssuerURL is the public URL of the Dex instance (e.g., https://dex.knodex-cloud.example.com).
	IssuerURL string

	// KnodexRedirectURL is the OAuth2 callback URL for Knodex (e.g., https://knodex.example.com/auth/callback).
	KnodexRedirectURL string

	// KnodexClientSecret is the OAuth2 client secret for the Knodex static client.
	// If empty, a random secret is generated.
	KnodexClientSecret string

	// DisableTLS runs Dex in HTTP mode instead of HTTPS.
	DisableTLS bool

	// LogLevel is the Dex log level (debug, info, warn, error).
	LogLevel string
}

// GenerateDexConfigYAML reads SSO providers from the Knodex ConfigMap/Secret
// and generates a complete Dex configuration YAML.
//
// This follows the ArgoCD pattern: the user configures SSO providers in Knodex,
// and this function translates them into Dex connectors with system defaults injected.
func GenerateDexConfigYAML(ctx context.Context, k8sClient kubernetes.Interface, namespace string, cfg Config) ([]byte, error) {
	providers, err := loadSSOProviders(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("loading SSO providers: %w", err)
	}

	dexCfg := buildDexConfig(providers, cfg)

	data, err := yaml.Marshal(dexCfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling Dex config: %w", err)
	}

	return data, nil
}

// buildDexConfig constructs the full Dex configuration map from SSO providers and config.
func buildDexConfig(providers []sso.SSOProvider, cfg Config) map[string]any {
	dexCfg := make(map[string]any)

	// Core settings
	dexCfg["issuer"] = cfg.IssuerURL
	dexCfg["storage"] = map[string]any{"type": "memory"}

	// Web configuration
	if cfg.DisableTLS {
		dexCfg["web"] = map[string]any{"http": fmt.Sprintf("0.0.0.0:%d", DefaultHTTPPort)}
	} else {
		dexCfg["web"] = map[string]any{
			"https":   fmt.Sprintf("0.0.0.0:%d", DefaultHTTPPort),
			"tlsCert": "/etc/dex/tls/tls.crt",
			"tlsKey":  "/etc/dex/tls/tls.key",
		}
	}

	// gRPC and telemetry
	dexCfg["grpc"] = map[string]any{"addr": fmt.Sprintf("0.0.0.0:%d", DefaultGRPCPort)}
	dexCfg["telemetry"] = map[string]any{"http": fmt.Sprintf("0.0.0.0:%d", DefaultTelemetryPort)}

	// OAuth2 settings
	dexCfg["oauth2"] = map[string]any{"skipApprovalScreen": true}

	// Logger
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	dexCfg["logger"] = map[string]any{"level": logLevel, "format": "json"}

	// Connectors — each Knodex SSO provider becomes a Dex OIDC connector
	connectors := buildConnectors(providers, cfg.IssuerURL)
	if len(connectors) > 0 {
		dexCfg["connectors"] = connectors
	}

	// Static clients — Knodex is always the first client
	clientSecret := cfg.KnodexClientSecret
	if clientSecret == "" {
		clientSecret = rand.GenerateRandomHex(32)
	}

	knodexClient := map[string]any{
		"id":           KnodexClientID,
		"name":         KnodexClientName,
		"secret":       clientSecret,
		"redirectURIs": []string{cfg.KnodexRedirectURL},
	}

	dexCfg["staticClients"] = []any{knodexClient}

	return dexCfg
}

// buildConnectors converts Knodex SSO providers into Dex connector configurations.
func buildConnectors(providers []sso.SSOProvider, issuerURL string) []map[string]any {
	dexRedirectURI := issuerURL + "/callback"

	connectors := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		connector := map[string]any{
			"type": "oidc",
			"id":   p.Name,
			"name": p.Name,
			"config": map[string]any{
				"issuer":       p.IssuerURL,
				"clientID":     p.ClientID,
				"clientSecret": p.ClientSecret,
				"redirectURI":  dexRedirectURI,
			},
		}

		// Add scopes if specified (default OIDC scopes are always included by Dex)
		if len(p.Scopes) > 0 {
			connector["config"].(map[string]any)["scopes"] = p.Scopes
		}

		connectors = append(connectors, connector)
	}

	return connectors
}

// loadSSOProviders reads SSO providers from the Knodex ConfigMap and Secret.
func loadSSOProviders(ctx context.Context, k8sClient kubernetes.Interface, namespace string) ([]sso.SSOProvider, error) {
	store := sso.NewProviderStore(k8sClient, namespace)
	providers, err := store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing SSO providers: %w", err)
	}

	slog.Info("loaded SSO providers for Dex config generation", "count", len(providers))
	return providers, nil
}

// AddStaticClient appends a static client to an existing Dex config.
// Used when deploying tools (ArgoCD, Grafana, etc.) that need their own OIDC client.
func AddStaticClient(dexConfigYAML []byte, clientID, clientName, clientSecret string, redirectURIs []string) ([]byte, error) {
	var dexCfg map[string]any
	if err := yaml.Unmarshal(dexConfigYAML, &dexCfg); err != nil {
		return nil, fmt.Errorf("parsing existing Dex config: %w", err)
	}

	newClient := map[string]any{
		"id":           clientID,
		"name":         clientName,
		"secret":       clientSecret,
		"redirectURIs": redirectURIs,
	}

	existingClients, _ := dexCfg["staticClients"].([]any)
	dexCfg["staticClients"] = append(existingClients, newClient)

	data, err := yaml.Marshal(dexCfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling updated Dex config: %w", err)
	}

	return data, nil
}

// WatchAndRegenerate watches the SSO ConfigMap/Secret and regenerates the Dex config
// whenever they change, writing the result to the specified file path.
// It returns a channel that receives a signal whenever the config is regenerated,
// allowing the caller to restart Dex.
func WatchAndRegenerate(ctx context.Context, k8sClient kubernetes.Interface, namespace string, cfg Config, outputPath string) (<-chan struct{}, error) {
	watcher := sso.NewSSOWatcher(k8sClient, namespace, slog.Default())

	regenCh := make(chan struct{}, 1)

	watcher.OnProvidersChanged(func(providers []sso.SSOProvider) {
		slog.Info("SSO providers changed, regenerating Dex config", "provider_count", len(providers))

		dexCfg := buildDexConfig(providers, cfg)
		data, err := yaml.Marshal(dexCfg)
		if err != nil {
			slog.Error("failed to marshal Dex config after SSO change", "error", err)
			return
		}

		if err := writeConfigFile(outputPath, data); err != nil {
			slog.Error("failed to write Dex config after SSO change", "error", err)
			return
		}

		slog.Info("Dex config regenerated", "path", outputPath)

		// Signal that config was regenerated (non-blocking)
		select {
		case regenCh <- struct{}{}:
		default:
		}
	})

	go func() {
		if err := watcher.Start(ctx); err != nil {
			slog.Error("SSO watcher failed", "error", err)
		}
	}()

	// Perform initial generation
	providers, err := loadSSOProviders(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("initial SSO provider load: %w", err)
	}

	dexCfg := buildDexConfig(providers, cfg)
	data, marshalErr := yaml.Marshal(dexCfg)
	if marshalErr != nil {
		return nil, fmt.Errorf("initial Dex config marshal: %w", marshalErr)
	}

	if err := writeConfigFile(outputPath, data); err != nil {
		return nil, fmt.Errorf("initial Dex config write: %w", err)
	}

	// Read static clients from ConfigMap if it exists
	staticClientsCM, err := k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, "knodex-dex-static-clients", metav1.GetOptions{})
	if err == nil && staticClientsCM.Data != nil {
		slog.Info("loading additional static clients from ConfigMap")
		// Merge additional static clients into the config
		currentData, readErr := readConfigFile(outputPath)
		if readErr == nil {
			for clientID, clientJSON := range staticClientsCM.Data {
				var clientCfg map[string]any
				if yamlErr := yaml.Unmarshal([]byte(clientJSON), &clientCfg); yamlErr == nil {
					currentData, _ = AddStaticClient(currentData, clientID, clientCfg["name"].(string), clientCfg["secret"].(string), toStringSlice(clientCfg["redirectURIs"]))
				}
			}
			_ = writeConfigFile(outputPath, currentData)
		}
	}

	slog.Info("initial Dex config generated", "path", outputPath, "provider_count", len(providers))

	return regenCh, nil
}
