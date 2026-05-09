// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/util/hash"
)

const (
	// DefaultResyncPeriod is the default resync period for SSO informers.
	DefaultResyncPeriod = 30 * time.Second
)

// ProvidersChangedFunc is called when SSO providers change.
// The function receives the new set of merged providers (config + credentials).
type ProvidersChangedFunc func(providers []SSOProvider)

// SSOWatcher watches the knodex-sso-providers ConfigMap and knodex-sso-secrets Secret
// for changes using typed K8s informers and triggers hot-reload of OIDC providers.
type SSOWatcher struct {
	k8sClient kubernetes.Interface
	namespace string
	logger    *slog.Logger

	mu                 sync.RWMutex
	lastValidProviders []SSOProvider
	lastProvidersHash  string // SHA-256 of last merged providers, used to suppress no-op reloads
	callbacks          []ProvidersChangedFunc
	ctx                context.Context // stored from Start() for use in K8s API calls

	stopCh   chan struct{}
	stopOnce sync.Once
	running  bool
}

// NewSSOWatcher creates a new SSO configuration watcher.
func NewSSOWatcher(k8sClient kubernetes.Interface, namespace string, logger *slog.Logger) *SSOWatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &SSOWatcher{
		k8sClient:          k8sClient,
		namespace:          namespace,
		logger:             logger,
		ctx:                context.Background(),
		lastValidProviders: []SSOProvider{},
	}
}

// OnProvidersChanged registers a callback that is invoked whenever providers change.
func (w *SSOWatcher) OnProvidersChanged(fn ProvidersChangedFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, fn)
}

// Providers returns the last valid set of providers (thread-safe deep copy).
func (w *SSOWatcher) Providers() []SSOProvider {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]SSOProvider, len(w.lastValidProviders))
	for i, p := range w.lastValidProviders {
		result[i] = p
		if p.Scopes != nil {
			result[i].Scopes = make([]string, len(p.Scopes))
			copy(result[i].Scopes, p.Scopes)
		}
	}
	return result
}

// Start begins watching ConfigMap and Secret for SSO config changes.
// It performs an initial sync, then watches for updates via informers.
// Blocks until ctx is canceled or Stop() is called.
func (w *SSOWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.stopOnce = sync.Once{}
	w.ctx = ctx
	w.mu.Unlock()

	w.logger.Info("starting SSO watcher",
		"namespace", w.namespace,
		"configmap", ConfigMapName,
		"secret", SecretName,
	)

	// Perform initial sync
	providers := w.loadProviders()
	w.mu.Lock()
	w.lastValidProviders = providers
	w.lastProvidersHash = providersHash(providers)
	w.mu.Unlock()

	if len(providers) > 0 {
		w.logger.Info("SSO watcher started, monitoring ConfigMap "+ConfigMapName,
			"providers_count", len(providers),
		)
	} else {
		w.logger.Warn("no SSO ConfigMap found, starting with zero OIDC providers")
	}

	// Create typed informer factory scoped to our namespace
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.k8sClient,
		DefaultResyncPeriod,
		informers.WithNamespace(w.namespace),
	)

	// Watch ConfigMaps (filtered to our specific ConfigMap)
	cmInformer := factory.Core().V1().ConfigMaps().Informer()
	cmInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			cm, ok := obj.(*corev1.ConfigMap)
			return ok && cm.Name == ConfigMapName
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(_ interface{}) { w.onConfigChange("ConfigMap", "add") },
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldCM := oldObj.(*corev1.ConfigMap)
				newCM := newObj.(*corev1.ConfigMap)
				if oldCM.ResourceVersion == newCM.ResourceVersion {
					return
				}
				w.onConfigChange("ConfigMap", "update")
			},
			DeleteFunc: func(_ interface{}) { w.onConfigMapDelete() },
		},
	})

	// Watch Secrets (filtered to our specific Secret)
	secretInformer := factory.Core().V1().Secrets().Informer()
	secretInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			s, ok := obj.(*corev1.Secret)
			return ok && s.Name == SecretName
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(_ interface{}) { w.onConfigChange("Secret", "add") },
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldSecret := oldObj.(*corev1.Secret)
				newSecret := newObj.(*corev1.Secret)
				if oldSecret.ResourceVersion == newSecret.ResourceVersion {
					return
				}
				w.onConfigChange("Secret", "update")
			},
			DeleteFunc: func(_ interface{}) { w.onConfigChange("Secret", "delete") },
		},
	})

	// Start informers
	factory.Start(w.stopCh)

	// Wait for initial cache sync
	if !cache.WaitForCacheSync(w.stopCh, cmInformer.HasSynced, secretInformer.HasSynced) {
		w.logger.Error("failed to sync SSO informer caches")
		w.setNotRunning()
		return fmt.Errorf("failed to sync SSO informer caches")
	}

	w.logger.Info("SSO informer caches synced, watching for changes")

	// Block until stopped
	select {
	case <-ctx.Done():
		w.logger.Info("SSO watcher stopping due to context cancellation")
	case <-w.stopCh:
		w.logger.Info("SSO watcher stopping due to stop signal")
	}

	w.setNotRunning()
	return nil
}

// Stop gracefully stops the watcher.
// Safe to call multiple times — uses sync.Once to prevent double-close panic.
func (w *SSOWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.logger.Info("stopping SSO watcher")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.running = false
}

// onConfigChange handles ConfigMap or Secret changes by reloading providers.
// Skips notifyCallbacks when the merged provider set is byte-identical to the
// last seen state. Suppresses redundant reloads from informer cache warm-up
// (initial AddFunc events) and periodic resyncs that don't change content.
func (w *SSOWatcher) onConfigChange(resource, action string) {
	w.logger.Info("SSO config change detected",
		"resource", resource,
		"action", action,
	)

	providers := w.loadProviders()
	newHash := providersHash(providers)

	w.mu.Lock()
	unchanged := newHash != "" && newHash == w.lastProvidersHash
	w.lastValidProviders = providers
	w.lastProvidersHash = newHash
	w.mu.Unlock()

	if unchanged {
		w.logger.Debug("SSO providers unchanged, skipping reload",
			"resource", resource,
			"action", action,
		)
		return
	}

	w.notifyCallbacks(providers)
}

// providersHash returns a deterministic SHA-256 of the merged provider set.
// Empty string is returned on marshal failure so callers fall through to reload.
func providersHash(providers []SSOProvider) string {
	data, err := json.Marshal(providers)
	if err != nil {
		return ""
	}
	return hash.SHA256(data)
}

// onConfigMapDelete handles ConfigMap deletion — keeps last valid providers.
func (w *SSOWatcher) onConfigMapDelete() {
	w.logger.Error("SSO ConfigMap deleted, keeping last valid providers",
		"configmap", ConfigMapName,
	)
	// Do NOT clear providers — keep last valid state per edge case #1
}

// loadProviders reads the current ConfigMap and Secret and merges them into SSOProviders.
// Returns empty slice if ConfigMap doesn't exist (valid state per AC #2).
// Returns last valid providers on parse error (AC #10).
func (w *SSOWatcher) loadProviders() []SSOProvider {
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()

	// Read ConfigMap
	cm, err := w.k8sClient.CoreV1().ConfigMaps(w.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return []SSOProvider{}
		}
		w.logger.Error("failed to read SSO ConfigMap", "error", err)
		return w.copyLastValid()
	}

	// Parse providers JSON from ConfigMap
	data, ok := cm.Data[ConfigMapKey]
	if !ok || data == "" {
		return []SSOProvider{}
	}

	var configs []providerConfig
	if err := json.Unmarshal([]byte(data), &configs); err != nil {
		w.logger.Error("malformed SSO providers JSON in ConfigMap, keeping last valid config",
			"error", err,
		)
		return w.copyLastValid()
	}

	// Read Secret for credentials
	secret, err := w.k8sClient.CoreV1().Secrets(w.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		w.logger.Error("failed to read SSO Secret", "error", err)
	}

	// Merge config + credentials, skipping entries with missing required fields
	providers := make([]SSOProvider, 0, len(configs))
	for _, cfg := range configs {
		if cfg.Name == "" || cfg.IssuerURL == "" {
			w.logger.Warn("skipping SSO provider with missing required fields",
				"name", cfg.Name,
				"issuerURL", cfg.IssuerURL,
			)
			continue
		}
		p := SSOProvider{
			Name:                    cfg.Name,
			IssuerURL:               cfg.IssuerURL,
			RedirectURL:             cfg.RedirectURL,
			Scopes:                  cfg.Scopes,
			TokenEndpointAuthMethod: cfg.TokenEndpointAuthMethod,
			// Explicit endpoints must round-trip through hot-reload; otherwise the
			// discovery-skip path in OIDC initialization is bypassed and reload
			// fails for IdPs with incomplete /.well-known/openid-configuration.
			AuthorizationURL: cfg.AuthorizationURL,
			TokenURL:         cfg.TokenURL,
			JWKSURL:          cfg.JWKSURL,
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

	return providers
}

// copyLastValid returns a copy of the last valid providers (must hold no locks).
func (w *SSOWatcher) copyLastValid() []SSOProvider {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]SSOProvider, len(w.lastValidProviders))
	copy(result, w.lastValidProviders)
	return result
}

// notifyCallbacks invokes all registered callbacks with the new providers.
func (w *SSOWatcher) notifyCallbacks(providers []SSOProvider) {
	w.mu.RLock()
	cbs := make([]ProvidersChangedFunc, len(w.callbacks))
	copy(cbs, w.callbacks)
	w.mu.RUnlock()

	for _, cb := range cbs {
		cb(providers)
	}
}

// setNotRunning marks the watcher as not running and closes stopCh (thread-safe).
// Must close stopCh to stop informer goroutines started by factory.Start(w.stopCh).
func (w *SSOWatcher) setNotRunning() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.running = false
}
