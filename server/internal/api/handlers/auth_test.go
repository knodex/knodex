package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/provops-org/knodex/server/internal/auth"
)

// MockAuthService is a mock implementation of auth.ServiceInterface for testing
// Updated to match new interface (removed rbac.User dependency)
type MockAuthService struct {
	authenticateLocalFunc func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error)
}

func (m *MockAuthService) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
	if m.authenticateLocalFunc != nil {
		return m.authenticateLocalFunc(ctx, username, password, sourceIP)
	}
	return nil, errors.New("not implemented")
}

// Implement other ServiceInterface methods (not used by handler but required for interface)
// GenerateTokenForAccount implements auth.ServiceInterface
func (m *MockAuthService) GenerateTokenForAccount(account *auth.Account, userID string) (string, time.Time, error) {
	return "", time.Time{}, errors.New("not implemented")
}

// GenerateTokenWithGroups implements auth.ServiceInterface
func (m *MockAuthService) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	return "", time.Time{}, errors.New("not implemented")
}

func (m *MockAuthService) ValidateToken(_ context.Context, tokenString string) (*auth.JWTClaims, error) {
	return nil, errors.New("not implemented")
}

func TestLocalLogin(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        interface{}
		mockAuthFunc       func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful login",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "password123",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				return &auth.LoginResponse{
					Token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
					ExpiresAt: time.Now().Add(1 * time.Hour),
					User: auth.UserInfo{
						ID:          "user-local-admin",
						Email:       "admin@local",
						DisplayName: "Local Administrator",
						CasbinRoles: []string{"role:serveradmin"},
					},
				}, nil
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp auth.LoginResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.Token == "" {
					t.Error("expected token in response, got empty")
				}
				if resp.User.ID != "user-local-admin" {
					t.Errorf("user ID = %v, want user-local-admin", resp.User.ID)
				}

				hasGlobalAdminRole := false
				for _, role := range resp.User.CasbinRoles {
					if role == "role:serveradmin" {
						hasGlobalAdminRole = true
						break
					}
				}
				if !hasGlobalAdminRole {
					t.Error("user should have role:serveradmin in CasbinRoles")
				}
			},
		},
		{
			name: "invalid credentials",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "wrongpassword",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				return nil, errors.New("invalid credentials")
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !bytes.Contains(rec.Body.Bytes(), []byte("invalid credentials")) {
					t.Error("expected 'invalid credentials' error message in response")
				}
			},
		},
		{
			name:        "malformed JSON",
			requestBody: `{"username": "admin", "password":`,
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with malformed JSON")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !bytes.Contains(rec.Body.Bytes(), []byte("invalid request body")) {
					t.Error("expected 'invalid request body' error message")
				}
			},
		},
		{
			name: "missing username",
			requestBody: auth.LocalLoginRequest{
				Username: "",
				Password: "password123",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with missing username")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !bytes.Contains(rec.Body.Bytes(), []byte("username and password are required")) {
					t.Error("expected 'username and password are required' error message")
				}
			},
		},
		{
			name: "missing password",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with missing password")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !bytes.Contains(rec.Body.Bytes(), []byte("username and password are required")) {
					t.Error("expected 'username and password are required' error message")
				}
			},
		},
		{
			name: "empty request body",
			requestBody: auth.LocalLoginRequest{
				Username: "",
				Password: "",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with empty body")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !bytes.Contains(rec.Body.Bytes(), []byte("username and password are required")) {
					t.Error("expected 'username and password are required' error message")
				}
			},
		},
		{
			name: "rate limited",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "password123",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				return nil, &auth.ErrRateLimited{RetryAfter: 180 * time.Second}
			},
			expectedStatusCode: http.StatusTooManyRequests,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !bytes.Contains(rec.Body.Bytes(), []byte("RATE_LIMIT_EXCEEDED")) {
					t.Error("expected 'RATE_LIMIT_EXCEEDED' code in response")
				}
				if !bytes.Contains(rec.Body.Bytes(), []byte(`"retry_after":"180"`)) {
					t.Errorf("expected retry_after=180 in response details, got: %s", rec.Body.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock auth service
			mockAuthSvc := &MockAuthService{
				authenticateLocalFunc: tt.mockAuthFunc,
			}

			// Create handler with mock
			testHandler := NewAuthHandler(mockAuthSvc, nil)

			// Marshal request body
			var bodyBytes []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("failed to marshal request body: %v", err)
				}
			}

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rec := httptest.NewRecorder()

			// Call handler
			testHandler.LocalLogin(rec, req)

			// Check status code
			if rec.Code != tt.expectedStatusCode {
				t.Errorf("status code = %d, want %d", rec.Code, tt.expectedStatusCode)
			}

			// Run additional response checks
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestLocalLogin_ContentType(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		authenticateLocalFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
			return &auth.LoginResponse{
				Token:     "test-token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:          "user-test",
					Email:       "test@example.com",
					CasbinRoles: []string{},
				},
			}, nil
		},
	}

	testHandler := NewAuthHandler(mockAuthSvc, nil)

	reqBody := auth.LocalLoginRequest{
		Username: "admin",
		Password: "password123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testHandler.LocalLogin(rec, req)

	// Check Content-Type header
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", contentType)
	}
}

func TestLocalLogin_SourceIPPassedThrough(t *testing.T) {
	var capturedIP string
	mockAuthSvc := &MockAuthService{
		authenticateLocalFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
			capturedIP = sourceIP
			return &auth.LoginResponse{
				Token:     "test-token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:          "user-test",
					Email:       "test@example.com",
					CasbinRoles: []string{},
				},
			}, nil
		},
	}

	testHandler := NewAuthHandler(mockAuthSvc, nil)

	reqBody := auth.LocalLoginRequest{
		Username: "admin",
		Password: "password123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.0.0.42")

	rec := httptest.NewRecorder()
	testHandler.LocalLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedIP != "10.0.0.42" {
		t.Errorf("sourceIP = %q, want %q", capturedIP, "10.0.0.42")
	}
}
