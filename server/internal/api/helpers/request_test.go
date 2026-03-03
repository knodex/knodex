package helpers

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSON(t *testing.T) {
	type TestPayload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name        string
		body        string
		maxSize     int64
		expectError bool
		errorMsg    string
		expected    *TestPayload
	}{
		{
			name:        "valid JSON",
			body:        `{"name":"test","value":42}`,
			expectError: false,
			expected:    &TestPayload{Name: "test", Value: 42},
		},
		{
			name:        "empty JSON object",
			body:        `{}`,
			expectError: false,
			expected:    &TestPayload{},
		},
		{
			name:        "malformed JSON syntax",
			body:        `{"name": invalid}`,
			expectError: true,
			errorMsg:    "Malformed JSON syntax",
		},
		{
			name:        "incomplete JSON payload",
			body:        `{"name": "test"`,
			expectError: true,
			errorMsg:    "Incomplete JSON payload",
		},
		{
			name:        "invalid JSON structure - wrong type",
			body:        `{"name":"test","value":"not-an-int"}`,
			expectError: true,
			errorMsg:    "Invalid JSON structure",
		},
		{
			name:        "body too large",
			body:        strings.Repeat("x", 1024),
			maxSize:     512,
			expectError: true,
			errorMsg:    "Malformed JSON syntax", // MaxBytesReader error occurs during decode, triggering JSON error first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			result, err := DecodeJSON[TestPayload](req, w, tt.maxSize)

			if tt.expectError {
				require.Error(t, err)
				var jsonErr *JSONDecodeError
				require.True(t, errors.As(err, &jsonErr))
				assert.Contains(t, jsonErr.Message, tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Value, result.Value)
			}
		})
	}
}

func TestDecodeJSON_DefaultMaxSize(t *testing.T) {
	type TestPayload struct {
		Data string `json:"data"`
	}

	// Create a payload just under 1MB (default max)
	data := strings.Repeat("a", 1<<19) // 512KB
	body := `{"data":"` + data + `"}`

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result, err := DecodeJSON[TestPayload](req, w, 0) // 0 uses default

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, data, result.Data)
}

func TestSanitizeJSONError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "Unknown error",
		},
		{
			name:     "request body too large",
			err:      errors.New("request body too large"),
			expected: "Request body too large (max 1MB)",
		},
		{
			name:     "cannot unmarshal",
			err:      errors.New("json: cannot unmarshal string into Go value"),
			expected: "Invalid JSON structure",
		},
		{
			name:     "unexpected EOF",
			err:      io.ErrUnexpectedEOF,
			expected: "Incomplete JSON payload",
		},
		{
			name:     "unexpected end",
			err:      errors.New("unexpected end of JSON input"),
			expected: "Incomplete JSON payload",
		},
		{
			name:     "invalid character",
			err:      errors.New("invalid character 'x' looking for beginning of value"),
			expected: "Malformed JSON syntax",
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeJSONError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONDecodeError(t *testing.T) {
	originalErr := errors.New("original error")
	jsonErr := &JSONDecodeError{
		Message: "User friendly message",
		Err:     originalErr,
	}

	assert.Equal(t, "User friendly message", jsonErr.Error())
	assert.Equal(t, originalErr, jsonErr.Unwrap())
	assert.True(t, errors.Is(jsonErr, originalErr))
}
