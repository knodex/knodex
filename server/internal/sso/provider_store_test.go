package sso

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared K8s fake client
// (each test mutates ConfigMaps/Secrets via the ProviderStore backed by a fake.Clientset).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "test-ns"

func newTestStore() (*ProviderStore, *fake.Clientset) {
	cs := fake.NewSimpleClientset()
	store := NewProviderStore(cs, testNamespace)
	return store, cs
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "google", false},
		{"valid with hyphens", "my-provider", false},
		{"valid with numbers", "auth0", false},
		{"valid complex", "my-auth-provider-1", false},
		{"empty", "", true},
		{"uppercase", "INVALID", true},
		{"has spaces", "has spaces", true},
		{"has dots", "a.b.c", true},
		{"starts with hyphen", "-provider", true},
		{"ends with hyphen", "provider-", true},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},   // 64 chars
		{"max length", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // 63 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestProviderStore_CreateAndList(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	provider := SSOProvider{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "client-id-123",
		ClientSecret: "client-secret-456",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile", "email"},
	}

	if err := store.Create(ctx, provider); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	providers, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}

	got := providers[0]
	if got.Name != "google" {
		t.Errorf("expected name 'google', got %q", got.Name)
	}
	if got.IssuerURL != "https://accounts.google.com" {
		t.Errorf("expected issuerURL 'https://accounts.google.com', got %q", got.IssuerURL)
	}
	if got.ClientID != "client-id-123" {
		t.Errorf("expected clientID 'client-id-123', got %q", got.ClientID)
	}
	if got.ClientSecret != "client-secret-456" {
		t.Errorf("expected clientSecret 'client-secret-456', got %q", got.ClientSecret)
	}
	if got.RedirectURL != "https://app.example.com/callback" {
		t.Errorf("expected redirectURL 'https://app.example.com/callback', got %q", got.RedirectURL)
	}
	if len(got.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(got.Scopes))
	}
}

func TestProviderStore_Get(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	provider := SSOProvider{
		Name:         "keycloak",
		IssuerURL:    "https://keycloak.example.com/realms/master",
		ClientID:     "kc-client",
		ClientSecret: "kc-secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid"},
	}

	if err := store.Create(ctx, provider); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := store.Get(ctx, "keycloak")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "keycloak" {
		t.Errorf("expected name 'keycloak', got %q", got.Name)
	}

	// Not found — should return *NotFoundError
	_, err = store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
	notFoundErr, ok := err.(*NotFoundError)
	if !ok {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
	if notFoundErr.Name != "nonexistent" {
		t.Errorf("expected NotFoundError for 'nonexistent', got %q", notFoundErr.Name)
	}
}

func TestProviderStore_DuplicateNameRejected(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	provider := SSOProvider{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid"},
	}

	if err := store.Create(ctx, provider); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	err := store.Create(ctx, provider)
	if err == nil {
		t.Fatal("expected error on duplicate create")
	}

	conflictErr, ok := err.(*ConflictError)
	if !ok {
		t.Fatalf("expected *ConflictError, got %T: %v", err, err)
	}
	if conflictErr.Name != "google" {
		t.Errorf("expected conflict for 'google', got %q", conflictErr.Name)
	}
}

func TestProviderStore_Update(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	original := SSOProvider{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "old-id",
		ClientSecret: "old-secret",
		RedirectURL:  "https://old.example.com/callback",
		Scopes:       []string{"openid"},
	}

	if err := store.Create(ctx, original); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated := SSOProvider{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "new-id",
		ClientSecret: "new-secret",
		RedirectURL:  "https://new.example.com/callback",
		Scopes:       []string{"openid", "profile"},
	}

	if err := store.Update(ctx, "google", updated); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := store.Get(ctx, "google")
	if err != nil {
		t.Fatalf("Get() after Update() error = %v", err)
	}

	if got.ClientID != "new-id" {
		t.Errorf("expected clientID 'new-id', got %q", got.ClientID)
	}
	if got.ClientSecret != "new-secret" {
		t.Errorf("expected clientSecret 'new-secret', got %q", got.ClientSecret)
	}
	if got.RedirectURL != "https://new.example.com/callback" {
		t.Errorf("expected redirectURL 'https://new.example.com/callback', got %q", got.RedirectURL)
	}
	if len(got.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(got.Scopes))
	}
}

func TestProviderStore_UpdateNotFound(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	// Create a provider first to ensure the ConfigMap exists
	if err := store.Create(ctx, SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/callback", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := store.Update(ctx, "nonexistent", SSOProvider{
		IssuerURL: "https://example.com", ClientID: "id",
		RedirectURL: "https://app.example.com/callback", Scopes: []string{"openid"},
	})
	if err == nil {
		t.Error("expected error for nonexistent provider update")
	}
}

func TestProviderStore_Delete(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	provider := SSOProvider{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid"},
	}

	if err := store.Create(ctx, provider); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.Delete(ctx, "google"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	providers, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() after Delete() error = %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("expected 0 providers after delete, got %d", len(providers))
	}
}

func TestProviderStore_DeleteNotFound(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	// Create a provider first to ensure ConfigMap exists
	if err := store.Create(ctx, SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/callback", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider delete")
	}
}

func TestProviderStore_NamespaceUsed(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := NewProviderStore(cs, "custom-namespace")
	ctx := context.Background()

	provider := SSOProvider{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid"},
	}

	if err := store.Create(ctx, provider); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify ConfigMap is in the correct namespace
	cm, err := cs.CoreV1().ConfigMaps("custom-namespace").Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not found in custom-namespace: %v", err)
	}
	if cm.Namespace != "custom-namespace" {
		t.Errorf("expected namespace 'custom-namespace', got %q", cm.Namespace)
	}

	// Verify Secret is in the correct namespace
	secret, err := cs.CoreV1().Secrets("custom-namespace").Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Secret not found in custom-namespace: %v", err)
	}
	if secret.Namespace != "custom-namespace" {
		t.Errorf("expected namespace 'custom-namespace', got %q", secret.Namespace)
	}
}

func TestProviderStore_MultipleProviders(t *testing.T) {
	store, _ := newTestStore()
	ctx := context.Background()

	providers := []SSOProvider{
		{Name: "google", IssuerURL: "https://accounts.google.com", ClientID: "g-id", ClientSecret: "g-secret", RedirectURL: "https://app.example.com/callback", Scopes: []string{"openid"}},
		{Name: "keycloak", IssuerURL: "https://kc.example.com", ClientID: "kc-id", ClientSecret: "kc-secret", RedirectURL: "https://app.example.com/callback", Scopes: []string{"openid", "profile"}},
		{Name: "auth0", IssuerURL: "https://tenant.auth0.com", ClientID: "a0-id", ClientSecret: "a0-secret", RedirectURL: "https://app.example.com/callback", Scopes: []string{"openid", "email"}},
	}

	for _, p := range providers {
		if err := store.Create(ctx, p); err != nil {
			t.Fatalf("Create(%s) error = %v", p.Name, err)
		}
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(list))
	}

	// Delete middle one
	if err := store.Delete(ctx, "keycloak"); err != nil {
		t.Fatalf("Delete(keycloak) error = %v", err)
	}

	list, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 providers after delete, got %d", len(list))
	}

	names := make(map[string]bool)
	for _, p := range list {
		names[p.Name] = true
	}
	if names["keycloak"] {
		t.Error("keycloak should have been deleted")
	}
	if !names["google"] || !names["auth0"] {
		t.Error("google and auth0 should still exist")
	}
}
