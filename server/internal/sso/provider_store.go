// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// ConfigMapName is the name of the ConfigMap storing SSO provider config
	ConfigMapName = "knodex-sso-providers"
	// SecretName is the name of the Secret storing SSO provider credentials
	SecretName = "knodex-sso-secrets"
	// ConfigMapKey is the key in the ConfigMap that holds the providers JSON
	ConfigMapKey = "providers.json"

	// Labels applied to managed resources
	LabelManagedBy    = "app.kubernetes.io/managed-by"
	LabelManagedByVal = "knodex"
	LabelConfigType   = "knodex.io/config-type"
	LabelConfigTypeV  = "sso"

	// MaxProviderNameLength is the maximum allowed length for a provider name
	MaxProviderNameLength = 63
)

// nameRegex validates DNS label format for provider names
var nameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// SSOProvider represents a configured OIDC provider
type SSOProvider struct {
	Name         string   `json:"name"`
	IssuerURL    string   `json:"issuerURL"`
	ClientID     string   `json:"clientID"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURL  string   `json:"redirectURL"`
	Scopes       []string `json:"scopes"`
	// TokenEndpointAuthMethod selects the OAuth2 token endpoint authentication method.
	// Empty (legacy default) is treated as "client_secret_basic" downstream when a
	// secret is present, or "none" (public client / PKCE) when no secret is present.
	TokenEndpointAuthMethod string `json:"tokenEndpointAuthMethod,omitempty"`

	// AuthorizationURL, TokenURL, and JWKSURL bypass OIDC discovery when all three
	// are set. Use for IdPs that serve an incomplete /.well-known/openid-configuration
	// (e.g., Supabase GoTrue). When any is empty, standard discovery runs.
	AuthorizationURL string `json:"authorizationURL,omitempty"`
	TokenURL         string `json:"tokenURL,omitempty"`
	JWKSURL          string `json:"jwksURL,omitempty"`
}

// providerConfig is the non-sensitive portion stored in the ConfigMap
type providerConfig struct {
	Name                    string   `json:"name"`
	IssuerURL               string   `json:"issuerURL"`
	RedirectURL             string   `json:"redirectURL"`
	Scopes                  []string `json:"scopes"`
	TokenEndpointAuthMethod string   `json:"tokenEndpointAuthMethod,omitempty"`
	AuthorizationURL        string   `json:"authorizationURL,omitempty"`
	TokenURL                string   `json:"tokenURL,omitempty"`
	JWKSURL                 string   `json:"jwksURL,omitempty"`
}

// ProviderStore manages SSO provider configuration in Kubernetes ConfigMaps and Secrets
type ProviderStore struct {
	k8sClient kubernetes.Interface
	namespace string
}

// NewProviderStore creates a new ProviderStore
func NewProviderStore(k8sClient kubernetes.Interface, namespace string) *ProviderStore {
	return &ProviderStore{
		k8sClient: k8sClient,
		namespace: namespace,
	}
}

// ValidateName checks that a provider name is a valid DNS label
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if len(name) > MaxProviderNameLength {
		return fmt.Errorf("provider name must be %d characters or less, got %d", MaxProviderNameLength, len(name))
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("provider name must match DNS label format: lowercase letters, numbers, and hyphens only (regex: %s)", nameRegex.String())
	}
	return nil
}

// List returns all SSO providers by merging ConfigMap config with Secret credentials
func (s *ProviderStore) List(ctx context.Context) ([]SSOProvider, error) {
	configs, err := s.readConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing SSO providers: %w", err)
	}

	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("reading SSO Secret: %w", err)
	}

	providers := make([]SSOProvider, 0, len(configs))
	for _, cfg := range configs {
		p := SSOProvider{
			Name:                    cfg.Name,
			IssuerURL:               cfg.IssuerURL,
			RedirectURL:             cfg.RedirectURL,
			Scopes:                  cfg.Scopes,
			TokenEndpointAuthMethod: cfg.TokenEndpointAuthMethod,
			AuthorizationURL:        cfg.AuthorizationURL,
			TokenURL:                cfg.TokenURL,
			JWKSURL:                 cfg.JWKSURL,
		}
		if secret != nil {
			if clientID, ok := secret.Data[cfg.Name+".client-id"]; ok {
				p.ClientID = string(clientID)
			}
			if clientSecret, ok := secret.Data[cfg.Name+".client-secret"]; ok {
				p.ClientSecret = string(clientSecret)
			}
		}
		providers = append(providers, p)
	}

	return providers, nil
}

// Get returns a single SSO provider by name
func (s *ProviderStore) Get(ctx context.Context, name string) (*SSOProvider, error) {
	providers, err := s.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting SSO provider %q: %w", name, err)
	}
	for _, p := range providers {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, &NotFoundError{Name: name}
}

// Create adds a new SSO provider. Writes ConfigMap first, then Secret.
// On Secret failure, rolls back the ConfigMap change.
func (s *ProviderStore) Create(ctx context.Context, provider SSOProvider) error {
	if err := ValidateName(provider.Name); err != nil {
		return fmt.Errorf("creating SSO provider: %w", err)
	}

	configs, err := s.readConfigs(ctx)
	if err != nil {
		return fmt.Errorf("creating SSO provider %q: %w", provider.Name, err)
	}

	// Check for duplicate name
	for _, c := range configs {
		if c.Name == provider.Name {
			return &ConflictError{Name: provider.Name}
		}
	}

	// Append new config
	newConfig := providerConfig{
		Name:                    provider.Name,
		IssuerURL:               provider.IssuerURL,
		RedirectURL:             provider.RedirectURL,
		Scopes:                  provider.Scopes,
		TokenEndpointAuthMethod: provider.TokenEndpointAuthMethod,
		AuthorizationURL:        provider.AuthorizationURL,
		TokenURL:                provider.TokenURL,
		JWKSURL:                 provider.JWKSURL,
	}
	configs = append(configs, newConfig)

	// Write ConfigMap first
	if err := s.writeConfigs(ctx, configs); err != nil {
		return fmt.Errorf("creating SSO provider %q: %w", provider.Name, err)
	}

	// Write Secret. For public clients (tokenEndpointAuthMethod=none), suppress
	// the client-secret entirely so it never lands in the Secret.
	clientSecret := provider.ClientSecret
	if provider.TokenEndpointAuthMethod == "none" {
		clientSecret = ""
	}
	if err := s.writeSecretKeys(ctx, provider.Name, provider.ClientID, clientSecret); err != nil {
		// Rollback ConfigMap
		rollbackConfigs := configs[:len(configs)-1]
		if rbErr := s.writeConfigs(ctx, rollbackConfigs); rbErr != nil {
			slog.Error("failed to rollback SSO ConfigMap after Secret write failure",
				"original_error", err,
				"rollback_error", rbErr,
				"provider", provider.Name,
			)
		}
		return fmt.Errorf("creating SSO provider %q: %w", provider.Name, err)
	}

	return nil
}

// Update modifies an existing SSO provider. Writes ConfigMap first, then Secret.
// On Secret failure, rolls back the ConfigMap change.
func (s *ProviderStore) Update(ctx context.Context, name string, provider SSOProvider) error {
	configs, err := s.readConfigs(ctx)
	if err != nil {
		return fmt.Errorf("updating SSO provider %q: %w", name, err)
	}

	found := false
	var previousConfig providerConfig
	for i, c := range configs {
		if c.Name == name {
			previousConfig = c
			configs[i] = providerConfig{
				Name:                    name,
				IssuerURL:               provider.IssuerURL,
				RedirectURL:             provider.RedirectURL,
				Scopes:                  provider.Scopes,
				TokenEndpointAuthMethod: provider.TokenEndpointAuthMethod,
				AuthorizationURL:        provider.AuthorizationURL,
				TokenURL:                provider.TokenURL,
				JWKSURL:                 provider.JWKSURL,
			}
			found = true
			break
		}
	}
	if !found {
		return &NotFoundError{Name: name}
	}

	// Write ConfigMap first
	if err := s.writeConfigs(ctx, configs); err != nil {
		return fmt.Errorf("updating SSO provider %q: %w", name, err)
	}

	rollback := func() {
		for i, c := range configs {
			if c.Name == name {
				configs[i] = previousConfig
				break
			}
		}
		if rbErr := s.writeConfigs(ctx, configs); rbErr != nil {
			slog.Error("failed to rollback SSO ConfigMap after Secret update failure",
				"rollback_error", rbErr,
				"provider", name,
			)
		}
	}

	// When flipping to a public client, remove any stored client-secret key so
	// downstream consumers (auth service, GitOps reconcilers) do not see a stale value.
	if provider.TokenEndpointAuthMethod == "none" {
		if err := s.deleteSecretKey(ctx, name+".client-secret"); err != nil {
			rollback()
			return fmt.Errorf("updating SSO provider %q: %w", name, err)
		}
	}

	// Write Secret (only if credentials provided). For public clients, suppress
	// the client-secret regardless of whether one was passed in.
	clientSecret := provider.ClientSecret
	if provider.TokenEndpointAuthMethod == "none" {
		clientSecret = ""
	}
	if provider.ClientID != "" || clientSecret != "" {
		if err := s.writeSecretKeys(ctx, name, provider.ClientID, clientSecret); err != nil {
			rollback()
			return fmt.Errorf("updating SSO provider %q: %w", name, err)
		}
	}

	return nil
}

// Delete removes an SSO provider from ConfigMap and Secret.
// Removes from ConfigMap first, then Secret keys.
func (s *ProviderStore) Delete(ctx context.Context, name string) error {
	configs, err := s.readConfigs(ctx)
	if err != nil {
		return fmt.Errorf("deleting SSO provider %q: %w", name, err)
	}

	found := false
	var removedConfig providerConfig
	var removedIndex int
	newConfigs := make([]providerConfig, 0, len(configs))
	for i, c := range configs {
		if c.Name == name {
			found = true
			removedConfig = c
			removedIndex = i
			continue
		}
		newConfigs = append(newConfigs, c)
	}
	if !found {
		return &NotFoundError{Name: name}
	}

	// Write ConfigMap first
	if err := s.writeConfigs(ctx, newConfigs); err != nil {
		return fmt.Errorf("deleting SSO provider %q: %w", name, err)
	}

	// Remove Secret keys
	if err := s.removeSecretKeys(ctx, name); err != nil {
		// Rollback ConfigMap - reinsert removed config at original position
		restored := slices.Insert(slices.Clone(newConfigs), removedIndex, removedConfig)
		if rbErr := s.writeConfigs(ctx, restored); rbErr != nil {
			slog.Error("failed to rollback SSO ConfigMap after Secret delete failure",
				"original_error", err,
				"rollback_error", rbErr,
				"provider", name,
			)
		}
		return fmt.Errorf("deleting SSO provider %q: %w", name, err)
	}

	return nil
}

// readConfigs reads the provider configurations from the ConfigMap.
// Returns an empty list when the ConfigMap does not yet exist.
func (s *ProviderStore) readConfigs(ctx context.Context) ([]providerConfig, error) {
	cm, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return []providerConfig{}, nil
		}
		return nil, fmt.Errorf("reading SSO ConfigMap: %w", err)
	}

	data, ok := cm.Data[ConfigMapKey]
	if !ok || data == "" {
		return []providerConfig{}, nil
	}

	var configs []providerConfig
	if err := json.Unmarshal([]byte(data), &configs); err != nil {
		return nil, fmt.Errorf("parsing SSO providers JSON: %w", err)
	}

	return configs, nil
}

// writeConfigs writes the provider configurations to the ConfigMap (create or update)
func (s *ProviderStore) writeConfigs(ctx context.Context, configs []providerConfig) error {
	data, err := json.Marshal(configs)
	if err != nil {
		return fmt.Errorf("marshaling SSO providers JSON: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: s.namespace,
			Labels: map[string]string{
				LabelManagedBy:  LabelManagedByVal,
				LabelConfigType: LabelConfigTypeV,
			},
		},
		Data: map[string]string{
			ConfigMapKey: string(data),
		},
	}

	existing, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, createErr := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{})
			if createErr != nil {
				return fmt.Errorf("creating SSO ConfigMap: %w", createErr)
			}
			return nil
		}
		return fmt.Errorf("reading existing SSO ConfigMap: %w", err)
	}

	existing.Data = cm.Data
	existing.Labels = cm.Labels
	if _, err = s.k8sClient.CoreV1().ConfigMaps(s.namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating SSO ConfigMap: %w", err)
	}
	return nil
}

// writeSecretKeys writes client credentials to the Secret
func (s *ProviderStore) writeSecretKeys(ctx context.Context, name, clientID, clientSecret string) error {
	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new secret. Skip empty values so public clients (no secret)
			// don't get a blank client-secret key emitted.
			data := map[string][]byte{}
			if clientID != "" {
				data[name+".client-id"] = []byte(clientID)
			}
			if clientSecret != "" {
				data[name+".client-secret"] = []byte(clientSecret)
			}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: s.namespace,
					Labels: map[string]string{
						LabelManagedBy:  LabelManagedByVal,
						LabelConfigType: LabelConfigTypeV,
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: data,
			}
			_, createErr := s.k8sClient.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
			if createErr != nil {
				return fmt.Errorf("creating SSO Secret: %w", createErr)
			}
			return nil
		}
		return fmt.Errorf("reading SSO Secret: %w", err)
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	// Always write the key when a value is provided; preserve existing value otherwise.
	// This ensures Create (which always has both) and Update (which may omit secret) are consistent.
	if clientID != "" {
		secret.Data[name+".client-id"] = []byte(clientID)
	}
	if clientSecret != "" {
		secret.Data[name+".client-secret"] = []byte(clientSecret)
	}

	if _, err = s.k8sClient.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating SSO Secret: %w", err)
	}
	return nil
}

// deleteSecretKey removes a single keyed entry from the SSO Secret.
// No-op when the Secret or key does not exist.
func (s *ProviderStore) deleteSecretKey(ctx context.Context, key string) error {
	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading SSO Secret for key removal: %w", err)
	}
	if _, exists := secret.Data[key]; !exists {
		return nil
	}
	delete(secret.Data, key)
	if _, err = s.k8sClient.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("removing SSO Secret key %q: %w", key, err)
	}
	return nil
}

// removeSecretKeys removes the client credentials for a provider from the Secret
func (s *ProviderStore) removeSecretKeys(ctx context.Context, name string) error {
	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // Nothing to delete
		}
		return fmt.Errorf("reading SSO Secret for removal: %w", err)
	}

	delete(secret.Data, name+".client-id")
	delete(secret.Data, name+".client-secret")

	if _, err = s.k8sClient.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("removing SSO Secret keys: %w", err)
	}
	return nil
}

// ConflictError is returned when creating a provider with a duplicate name
type ConflictError struct {
	Name string
	Err  error
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("SSO provider %q already exists", e.Name)
}

func (e *ConflictError) Unwrap() error { return e.Err }

// NotFoundError is returned when a provider is not found
type NotFoundError struct {
	Name string
	Err  error
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("SSO provider %q not found", e.Name)
}

func (e *NotFoundError) Unwrap() error { return e.Err }
