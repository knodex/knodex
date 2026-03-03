package sso

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared K8s fake client
// and direct mutation of SSOWatcher internal fields (mu, lastValidProviders, running, stopOnce).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestWatcher() (*SSOWatcher, *fake.Clientset) {
	cs := fake.NewSimpleClientset()
	w := NewSSOWatcher(cs, testNamespace, slog.Default())
	return w, cs
}

func newTestWatcherWithObjects(cm *corev1.ConfigMap, secret *corev1.Secret) (*SSOWatcher, *fake.Clientset) {
	cs := fake.NewSimpleClientset()
	ctx := context.Background()

	if cm != nil {
		cs.CoreV1().ConfigMaps(testNamespace).Create(ctx, cm, metav1.CreateOptions{})
	}
	if secret != nil {
		cs.CoreV1().Secrets(testNamespace).Create(ctx, secret, metav1.CreateOptions{})
	}

	w := NewSSOWatcher(cs, testNamespace, slog.Default())
	return w, cs
}

func makeProviderConfigJSON(providers ...providerConfig) string {
	data, _ := json.Marshal(providers)
	return string(data)
}

func TestSSOWatcher_LoadProviders_NoConfigMap(t *testing.T) {
	w, _ := newTestWatcher()

	providers := w.loadProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers when no ConfigMap exists, got %d", len(providers))
	}
}

func TestSSOWatcher_LoadProviders_EmptyConfigMap(t *testing.T) {
	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{},
		},
		nil,
	)

	providers := w.loadProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers for empty ConfigMap, got %d", len(providers))
	}
}

func TestSSOWatcher_LoadProviders_EmptyProvidersJSON(t *testing.T) {
	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: "[]"},
		},
		nil,
	)

	providers := w.loadProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers for empty JSON array, got %d", len(providers))
	}
}

func TestSSOWatcher_LoadProviders_WithProviders(t *testing.T) {
	configs := []providerConfig{
		{Name: "google", IssuerURL: "https://accounts.google.com", RedirectURL: "https://app/callback", Scopes: []string{"openid", "profile"}},
		{Name: "keycloak", IssuerURL: "https://kc.example.com/realms/master", RedirectURL: "https://app/callback", Scopes: []string{"openid"}},
	}

	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: makeProviderConfigJSON(configs...)},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: testNamespace},
			Data: map[string][]byte{
				"google.client-id":       []byte("g-id"),
				"google.client-secret":   []byte("g-secret"),
				"keycloak.client-id":     []byte("kc-id"),
				"keycloak.client-secret": []byte("kc-secret"),
			},
		},
	)

	providers := w.loadProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	// Verify first provider
	google := providers[0]
	if google.Name != "google" {
		t.Errorf("expected name 'google', got %q", google.Name)
	}
	if google.IssuerURL != "https://accounts.google.com" {
		t.Errorf("expected issuerURL 'https://accounts.google.com', got %q", google.IssuerURL)
	}
	if google.ClientID != "g-id" {
		t.Errorf("expected clientID 'g-id', got %q", google.ClientID)
	}
	if google.ClientSecret != "g-secret" {
		t.Errorf("expected clientSecret 'g-secret', got %q", google.ClientSecret)
	}

	// Verify second provider
	kc := providers[1]
	if kc.Name != "keycloak" {
		t.Errorf("expected name 'keycloak', got %q", kc.Name)
	}
	if kc.ClientID != "kc-id" {
		t.Errorf("expected clientID 'kc-id', got %q", kc.ClientID)
	}
}

func TestSSOWatcher_LoadProviders_NoSecret(t *testing.T) {
	configs := []providerConfig{
		{Name: "google", IssuerURL: "https://accounts.google.com", RedirectURL: "https://app/callback", Scopes: []string{"openid"}},
	}

	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: makeProviderConfigJSON(configs...)},
		},
		nil, // no secret
	)

	providers := w.loadProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}

	// Provider should have config but no credentials
	if providers[0].Name != "google" {
		t.Errorf("expected name 'google', got %q", providers[0].Name)
	}
	if providers[0].ClientID != "" {
		t.Errorf("expected empty clientID without secret, got %q", providers[0].ClientID)
	}
	if providers[0].ClientSecret != "" {
		t.Errorf("expected empty clientSecret without secret, got %q", providers[0].ClientSecret)
	}
}

func TestSSOWatcher_LoadProviders_MalformedJSON(t *testing.T) {
	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: "{invalid json}"},
		},
		nil,
	)

	// Set a "last valid" state to verify fallback
	w.mu.Lock()
	w.lastValidProviders = []SSOProvider{
		{Name: "cached-provider", IssuerURL: "https://cached.example.com"},
	}
	w.mu.Unlock()

	providers := w.loadProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider (last valid), got %d", len(providers))
	}
	if providers[0].Name != "cached-provider" {
		t.Errorf("expected fallback to cached-provider, got %q", providers[0].Name)
	}
}

func TestSSOWatcher_LoadProviders_MalformedJSON_NoLastValid(t *testing.T) {
	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: "not-json"},
		},
		nil,
	)

	// No last valid state — should return empty
	providers := w.loadProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers on first malformed JSON, got %d", len(providers))
	}
}

func TestSSOWatcher_OnConfigChange_InvokesCallbacks(t *testing.T) {
	configs := []providerConfig{
		{Name: "google", IssuerURL: "https://accounts.google.com", RedirectURL: "https://app/callback", Scopes: []string{"openid"}},
	}

	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: makeProviderConfigJSON(configs...)},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: testNamespace},
			Data: map[string][]byte{
				"google.client-id":     []byte("g-id"),
				"google.client-secret": []byte("g-secret"),
			},
		},
	)

	var callbackInvoked atomic.Bool
	var receivedProviders []SSOProvider
	var mu sync.Mutex

	w.OnProvidersChanged(func(providers []SSOProvider) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked.Store(true)
		receivedProviders = providers
	})

	w.onConfigChange("ConfigMap", "update")

	if !callbackInvoked.Load() {
		t.Error("expected callback to be invoked on config change")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedProviders) != 1 {
		t.Fatalf("expected 1 provider in callback, got %d", len(receivedProviders))
	}
	if receivedProviders[0].Name != "google" {
		t.Errorf("expected provider name 'google', got %q", receivedProviders[0].Name)
	}
	if receivedProviders[0].ClientID != "g-id" {
		t.Errorf("expected clientID 'g-id', got %q", receivedProviders[0].ClientID)
	}
}

func TestSSOWatcher_OnConfigChange_MultipleCallbacks(t *testing.T) {
	w, _ := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: "[]"},
		},
		nil,
	)

	var count1, count2 atomic.Int32

	w.OnProvidersChanged(func(_ []SSOProvider) {
		count1.Add(1)
	})
	w.OnProvidersChanged(func(_ []SSOProvider) {
		count2.Add(1)
	})

	w.onConfigChange("Secret", "update")

	if count1.Load() != 1 {
		t.Errorf("expected callback 1 invoked once, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("expected callback 2 invoked once, got %d", count2.Load())
	}
}

func TestSSOWatcher_OnConfigMapDelete_KeepsLastValid(t *testing.T) {
	w, _ := newTestWatcher()

	// Set a "last valid" state
	w.mu.Lock()
	w.lastValidProviders = []SSOProvider{
		{Name: "provider-a", IssuerURL: "https://a.example.com", ClientID: "a-id"},
	}
	w.mu.Unlock()

	// Simulate ConfigMap deletion
	w.onConfigMapDelete()

	// Providers should still be available
	providers := w.Providers()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider after ConfigMap delete, got %d", len(providers))
	}
	if providers[0].Name != "provider-a" {
		t.Errorf("expected provider-a, got %q", providers[0].Name)
	}
}

func TestSSOWatcher_Providers_ReturnsCopy(t *testing.T) {
	w, _ := newTestWatcher()

	w.mu.Lock()
	w.lastValidProviders = []SSOProvider{
		{Name: "original"},
	}
	w.mu.Unlock()

	// Get a copy
	copy1 := w.Providers()
	copy1[0].Name = "modified"

	// Original should be unchanged
	copy2 := w.Providers()
	if copy2[0].Name != "original" {
		t.Errorf("Providers() should return a copy, but original was modified to %q", copy2[0].Name)
	}
}

func TestSSOWatcher_Providers_ThreadSafe(t *testing.T) {
	configs := []providerConfig{
		{Name: "google", IssuerURL: "https://accounts.google.com", RedirectURL: "https://app/callback", Scopes: []string{"openid"}},
	}

	w, cs := newTestWatcherWithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
			Data:       map[string]string{ConfigMapKey: makeProviderConfigJSON(configs...)},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: testNamespace},
			Data: map[string][]byte{
				"google.client-id":     []byte("g-id"),
				"google.client-secret": []byte("g-secret"),
			},
		},
	)

	// Set initial state
	w.mu.Lock()
	w.lastValidProviders = w.loadProviders()
	w.mu.Unlock()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				providers := w.Providers()
				_ = providers
			}
		}()
	}

	// Concurrent writers (simulate config changes)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				// Update ConfigMap to simulate change
				newConfigs := []providerConfig{
					{Name: "provider-" + string(rune('a'+idx)), IssuerURL: "https://example.com", RedirectURL: "https://app/callback", Scopes: []string{"openid"}},
				}
				data, _ := json.Marshal(newConfigs)
				cs.CoreV1().ConfigMaps(testNamespace).Update(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
					Data:       map[string]string{ConfigMapKey: string(data)},
				}, metav1.UpdateOptions{})

				w.onConfigChange("ConfigMap", "update")
			}
		}(i)
	}

	// Should complete without panic or deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent access test timed out — possible deadlock")
	}
}

func TestSSOWatcher_StopMultipleTimes(t *testing.T) {
	w, _ := newTestWatcher()

	// Set up as if running
	w.mu.Lock()
	w.running = true
	w.stopCh = make(chan struct{})
	w.stopOnce = sync.Once{}
	w.mu.Unlock()

	// Stop multiple times — should not panic
	w.Stop()
	w.Stop()
	w.Stop()
}

func TestSSOWatcher_StopWhenNotRunning(t *testing.T) {
	w, _ := newTestWatcher()

	// Stop when not running — should be a no-op
	w.Stop()
}

func TestNewSSOWatcher_NilLogger(t *testing.T) {
	cs := fake.NewSimpleClientset()
	w := NewSSOWatcher(cs, "default", nil)
	if w.logger == nil {
		t.Error("expected default logger when nil passed")
	}
}
