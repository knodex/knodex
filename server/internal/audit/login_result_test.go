package audit

import (
	"net/http/httptest"
	"testing"
)

func TestSetLoginResult_WithPreparedContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/auth/callback", nil)
	req, reader := PrepareLoginResult(req)

	SetLoginResult(req, "success")

	if got := reader(); got != "success" {
		t.Errorf("reader() = %q, want %q", got, "success")
	}
}

func TestSetLoginResult_WithoutPreparedContext(t *testing.T) {
	t.Parallel()

	// SetLoginResult should be a no-op when context has no signal
	req := httptest.NewRequest("POST", "/api/v1/auth/callback", nil)
	SetLoginResult(req, "success") // should not panic
}

func TestPrepareLoginResult_DefaultEmpty(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/auth/callback", nil)
	_, reader := PrepareLoginResult(req)

	if got := reader(); got != "" {
		t.Errorf("reader() = %q, want empty string", got)
	}
}

func TestSetLoginResult_OverwritesPreviousValue(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/auth/callback", nil)
	req, reader := PrepareLoginResult(req)

	SetLoginResult(req, "denied")
	SetLoginResult(req, "success")

	if got := reader(); got != "success" {
		t.Errorf("reader() = %q, want %q", got, "success")
	}
}

func TestPrepareLoginResult_IndependentSignals(t *testing.T) {
	t.Parallel()

	req1 := httptest.NewRequest("POST", "/api/v1/auth/callback", nil)
	req1, reader1 := PrepareLoginResult(req1)

	req2 := httptest.NewRequest("POST", "/api/v1/auth/callback", nil)
	req2, reader2 := PrepareLoginResult(req2)

	SetLoginResult(req1, "success")
	SetLoginResult(req2, "denied")

	if got := reader1(); got != "success" {
		t.Errorf("reader1() = %q, want %q", got, "success")
	}
	if got := reader2(); got != "denied" {
		t.Errorf("reader2() = %q, want %q", got, "denied")
	}
}

// --- SetLoginIdentity / PrepareLoginIdentity tests ---

func TestSetLoginIdentity_WithPreparedContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback", nil)
	req, reader := PrepareLoginIdentity(req)

	SetLoginIdentity(req, "user-alice", "alice@test.local")

	userID, email := reader()
	if userID != "user-alice" {
		t.Errorf("userID = %q, want %q", userID, "user-alice")
	}
	if email != "alice@test.local" {
		t.Errorf("email = %q, want %q", email, "alice@test.local")
	}
}

func TestSetLoginIdentity_WithoutPreparedContext(t *testing.T) {
	t.Parallel()

	// SetLoginIdentity should be a no-op when context has no signal
	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback", nil)
	SetLoginIdentity(req, "user-alice", "alice@test.local") // should not panic
}

func TestPrepareLoginIdentity_DefaultEmpty(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback", nil)
	_, reader := PrepareLoginIdentity(req)

	userID, email := reader()
	if userID != "" {
		t.Errorf("userID = %q, want empty string", userID)
	}
	if email != "" {
		t.Errorf("email = %q, want empty string", email)
	}
}

func TestSetLoginIdentity_OverwritesPreviousValue(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback", nil)
	req, reader := PrepareLoginIdentity(req)

	SetLoginIdentity(req, "user-old", "old@test.local")
	SetLoginIdentity(req, "user-new", "new@test.local")

	userID, email := reader()
	if userID != "user-new" {
		t.Errorf("userID = %q, want %q", userID, "user-new")
	}
	if email != "new@test.local" {
		t.Errorf("email = %q, want %q", email, "new@test.local")
	}
}

func TestPrepareLoginIdentity_IndependentSignals(t *testing.T) {
	t.Parallel()

	req1 := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback", nil)
	req1, reader1 := PrepareLoginIdentity(req1)

	req2 := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback", nil)
	req2, reader2 := PrepareLoginIdentity(req2)

	SetLoginIdentity(req1, "user-1", "one@test.local")
	SetLoginIdentity(req2, "user-2", "two@test.local")

	id1, email1 := reader1()
	id2, email2 := reader2()
	if id1 != "user-1" || email1 != "one@test.local" {
		t.Errorf("reader1() = (%q, %q), want (%q, %q)", id1, email1, "user-1", "one@test.local")
	}
	if id2 != "user-2" || email2 != "two@test.local" {
		t.Errorf("reader2() = (%q, %q), want (%q, %q)", id2, email2, "user-2", "two@test.local")
	}
}
