package logger

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSensitiveData_GitHubTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "GitHub Personal Access Token",
			input:    "Token: ghp_1234567890abcdefABCDEF1234567890WXYZ",
			expected: "Token: [REDACTED-GitHub Personal Access Token]",
		},
		{
			name:     "GitHub OAuth Token",
			input:    "OAuth token is gho_1234567890abcdefABCDEF1234567890WXYZ",
			expected: "OAuth token is [REDACTED-GitHub OAuth Token]",
		},
		{
			name:     "GitHub User Token",
			input:    "User token: ghu_1234567890abcdefABCDEF1234567890WXYZ",
			expected: "User token: [REDACTED-GitHub User Token]",
		},
		{
			name:     "GitHub Server Token",
			input:    "Server: ghs_1234567890abcdefABCDEF1234567890WXYZ",
			expected: "Server: [REDACTED-GitHub Server Token]",
		},
		{
			name:     "GitHub Refresh Token",
			input:    "Refresh: ghr_1234567890123456789012345678901234567890123456789012345678901234567890123456",
			expected: "Refresh: [REDACTED-GitHub Refresh Token]",
		},
		{
			name:     "Multiple tokens in one string",
			input:    "ghp_1234567890abcdefABCDEF1234567890WXYZ and gho_1234567890abcdefABCDEF1234567890WXYZ",
			expected: "[REDACTED-GitHub Personal Access Token] and [REDACTED-GitHub OAuth Token]",
		},
		{
			name:     "No sensitive data",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitiveData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactSensitiveData_BearerTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bearer token with typical JWT",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123",
			expected: "Authorization: [REDACTED-Bearer Token]",
		},
		{
			name:     "Bearer token in middle of string",
			input:    "Request with Bearer abc123def456ghi789 failed",
			expected: "Request with [REDACTED-Bearer Token] failed",
		},
		{
			name:     "Multiple bearer tokens",
			input:    "Bearer token1 and Bearer token2",
			expected: "[REDACTED-Bearer Token] and [REDACTED-Bearer Token]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitiveData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactSensitiveData_APIKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API key with equals",
			input:    "api_key=sk_test_1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "[REDACTED-API Key]",
		},
		{
			name:     "API key with colon",
			input:    "apikey: 1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "[REDACTED-API Key]",
		},
		{
			name:     "API token with quotes",
			input:    "api-token='sk_live_abcdefghijklmnopqrstuvwxyz1234567890'",
			expected: "[REDACTED-API Key]",
		},
		{
			name:     "API key case insensitive",
			input:    "API_KEY=secret123456789012345678901234567890",
			expected: "[REDACTED-API Key]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitiveData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactValue_StringValue(t *testing.T) {
	input := slog.StringValue("Token: ghp_1234567890abcdefABCDEF1234567890WXYZ")
	expected := slog.StringValue("Token: [REDACTED-GitHub Personal Access Token]")

	result := RedactValue(input)
	assert.Equal(t, expected.String(), result.String())
}

func TestRedactValue_GroupValue(t *testing.T) {
	input := slog.GroupValue(
		slog.String("token", "ghp_1234567890abcdefABCDEF1234567890WXYZ"),
		slog.String("message", "Normal message"),
	)

	result := RedactValue(input)

	// Extract the group attributes
	attrs := result.Group()
	require.Len(t, attrs, 2)

	// Check redacted token
	assert.Equal(t, "token", attrs[0].Key)
	assert.Equal(t, "[REDACTED-GitHub Personal Access Token]", attrs[0].Value.String())

	// Check normal message unchanged
	assert.Equal(t, "message", attrs[1].Key)
	assert.Equal(t, "Normal message", attrs[1].Value.String())
}

func TestRedactValue_NonStringValue(t *testing.T) {
	tests := []struct {
		name  string
		value slog.Value
	}{
		{"Int", slog.IntValue(42)},
		{"Bool", slog.BoolValue(true)},
		{"Float", slog.Float64Value(3.14)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactValue(tt.value)
			assert.Equal(t, tt.value, result, "Non-string values should not be modified")
		})
	}
}

func TestRedactionHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler)

	// Log a message with sensitive data
	logger.Info("Authentication successful",
		"token", "ghp_1234567890abcdefABCDEF1234567890WXYZ",
		"user", "testuser",
	)

	output := buf.String()

	// Verify token is redacted
	assert.Contains(t, output, "[REDACTED-GitHub Personal Access Token]")
	assert.NotContains(t, output, "ghp_1234567890abcdefABCDEF1234567890WXYZ")

	// Verify normal data is preserved
	assert.Contains(t, output, "testuser")
	assert.Contains(t, output, "Authentication successful")
}

func TestRedactionHandler_MessageRedaction(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler)

	// Log message with token in the message itself
	logger.Info("Token ghp_1234567890abcdefABCDEF1234567890WXYZ received")

	output := buf.String()

	// Verify token in message is redacted
	assert.Contains(t, output, "Token [REDACTED-GitHub Personal Access Token] received")
	assert.NotContains(t, output, "ghp_1234567890abcdefABCDEF1234567890WXYZ")
}

func TestRedactionHandler_MultipleTokenTypes(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler)

	// Log with multiple types of sensitive data
	logger.Info("Multiple credentials detected",
		"github_token", "ghp_1234567890abcdefABCDEF1234567890WXYZ",
		"bearer", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"api_key", "api_key=sk_test_1234567890abcdefghij",
		"normal_field", "safe_value",
	)

	output := buf.String()

	// Verify all sensitive data is redacted
	assert.Contains(t, output, "[REDACTED-GitHub Personal Access Token]")
	assert.Contains(t, output, "[REDACTED-Bearer Token]")
	assert.Contains(t, output, "[REDACTED-API Key]")

	// Verify no actual tokens in output
	assert.NotContains(t, output, "ghp_1234567890abcdefABCDEF1234567890WXYZ")
	assert.NotContains(t, output, "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	assert.NotContains(t, output, "sk_test_1234567890abcdefghij")

	// Verify normal data preserved
	assert.Contains(t, output, "safe_value")
}

func TestRedactionHandler_NestedGroups(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler)

	// Log with nested group containing sensitive data
	logger.Info("Request processed",
		slog.Group("auth",
			slog.String("token", "ghp_1234567890abcdefABCDEF1234567890WXYZ"),
			slog.String("method", "oauth"),
		),
		slog.Group("user",
			slog.String("id", "user123"),
			slog.String("email", "user@example.com"),
		),
	)

	output := buf.String()

	// Verify nested token is redacted
	assert.Contains(t, output, "[REDACTED-GitHub Personal Access Token]")
	assert.NotContains(t, output, "ghp_1234567890abcdefABCDEF1234567890WXYZ")

	// Verify normal nested data preserved
	assert.Contains(t, output, "oauth")
	assert.Contains(t, output, "user123")
	assert.Contains(t, output, "user@example.com")
}

func TestRedactionHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)

	// Create logger with default attributes containing sensitive data
	logger := slog.New(handler).With(
		"default_token", "gho_1234567890abcdefABCDEF1234567890WXYZ",
		"service", "api",
	)

	logger.Info("Processing request")

	output := buf.String()

	// Verify default attribute token is redacted
	assert.Contains(t, output, "[REDACTED-GitHub OAuth Token]")
	assert.NotContains(t, output, "gho_1234567890abcdefABCDEF1234567890WXYZ")

	// Verify normal default attribute preserved
	assert.Contains(t, output, "api")
}

func TestRedactionHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler).WithGroup("request")

	logger.Info("API call",
		"token", "ghu_1234567890abcdefABCDEF1234567890WXYZ",
		"endpoint", "/api/users",
	)

	output := buf.String()

	// Verify grouped token is redacted
	assert.Contains(t, output, "[REDACTED-GitHub User Token]")
	assert.NotContains(t, output, "ghu_1234567890abcdefABCDEF1234567890WXYZ")

	// Verify group structure preserved
	assert.Contains(t, output, "/api/users")
}

func TestAddRedactionPattern(t *testing.T) {
	// Add a custom pattern
	customPattern := regexp.MustCompile(`SECRET-[0-9]{6}`)
	AddRedactionPattern("Custom Secret", customPattern)

	input := "My secret is SECRET-123456 here"
	result := RedactSensitiveData(input)

	assert.Equal(t, "My secret is [REDACTED-Custom Secret] here", result)
}

func TestRedactionHandler_Enabled(t *testing.T) {
	baseHandler := slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})

	handler := NewRedactionHandler(baseHandler)

	// Should be enabled for WARN and above
	assert.True(t, handler.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))

	// Should be disabled for INFO and DEBUG
	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, handler.Enabled(context.Background(), slog.LevelDebug))
}

func TestRedactSensitiveData_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		mustNotContain string
	}{
		{
			name:           "GitHub API error message",
			input:          "GitHub API request failed: token ghp_1234567890abcdefABCDEF1234567890WXYZ is invalid",
			mustNotContain: "ghp_1234567890abcdefABCDEF1234567890WXYZ",
		},
		{
			name:           "JSON response with token",
			input:          `{"token":"ghp_1234567890abcdefABCDEF1234567890WXYZ","user":"john"}`,
			mustNotContain: "ghp_1234567890abcdefABCDEF1234567890WXYZ",
		},
		{
			name:           "Authorization header",
			input:          "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
			mustNotContain: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
		},
		{
			name:           "cURL command with API key",
			input:          "curl -H 'api-key: sk_live_abcdef1234567890ghijklmnopqrstuv' https://api.example.com",
			mustNotContain: "sk_live_abcdef1234567890ghijklmnopqrstuv",
		},
		{
			name:           "Log message with multiple credentials",
			input:          "User authenticated with ghp_1234567890abcdefABCDEF1234567890WXYZ and received Bearer jwt_abcdefghijklmnop",
			mustNotContain: "ghp_1234567890abcdefABCDEF1234567890WXYZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitiveData(tt.input)
			assert.NotContains(t, result, tt.mustNotContain, "Sensitive data should be redacted")
			assert.Contains(t, result, "[REDACTED-", "Should contain redaction marker")
		})
	}
}

func TestRedactionHandler_EmptyValues(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler)

	// Log with empty values
	logger.Info("", "empty_key", "", "nil_key", nil)

	output := buf.String()

	// Should not crash and should produce valid JSON
	assert.NotEmpty(t, output)
	// Verify it's valid JSON by checking it contains expected structure
	assert.Contains(t, output, "level")
}

func TestRedactionConcurrency(t *testing.T) {
	// Test that redaction is thread-safe
	const goroutines = 100
	const iterations = 10

	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				input := "Token: ghp_1234567890abcdefABCDEF1234567890WXYZ"
				result := RedactSensitiveData(input)
				if !strings.Contains(result, "[REDACTED-") {
					t.Error("Concurrent redaction failed")
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func BenchmarkRedactSensitiveData(b *testing.B) {
	input := "Processing request with token ghp_1234567890abcdefABCDEF1234567890WXYZ and Bearer jwt_token_here"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RedactSensitiveData(input)
	}
}

func BenchmarkRedactionHandler(b *testing.B) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewRedactionHandler(baseHandler)
	logger := slog.New(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		logger.Info("Processing",
			"token", "ghp_1234567890abcdefABCDEF1234567890WXYZ",
			"request_id", "req-123",
		)
	}
}
