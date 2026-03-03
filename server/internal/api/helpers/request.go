package helpers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// DefaultMaxBodySize is the default request body size limit (1MB)
const DefaultMaxBodySize = 1 << 20

// JSONDecodeError contains sanitized error info for client response
type JSONDecodeError struct {
	Message string
	Err     error
}

func (e *JSONDecodeError) Error() string { return e.Message }
func (e *JSONDecodeError) Unwrap() error { return e.Err }

// DecodeJSON decodes JSON from request body with size limiting.
// Uses generics for type safety. Returns sanitized error messages.
func DecodeJSON[T any](r *http.Request, w http.ResponseWriter, maxSize int64) (*T, error) {
	if maxSize == 0 {
		maxSize = DefaultMaxBodySize
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	var result T
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		return nil, &JSONDecodeError{
			Message: SanitizeJSONError(err),
			Err:     err,
		}
	}
	return &result, nil
}

// SanitizeJSONError converts internal JSON errors to user-friendly messages.
// Exported for backward compatibility with existing handlers.
func SanitizeJSONError(err error) string {
	if err == nil {
		return "Unknown error"
	}
	errStr := err.Error()

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) || strings.Contains(errStr, "request body too large") {
		return "Request body too large (max 1MB)"
	}
	if strings.Contains(errStr, "cannot unmarshal") {
		return "Invalid JSON structure"
	}
	if strings.Contains(errStr, "unexpected EOF") || strings.Contains(errStr, "unexpected end") {
		return "Incomplete JSON payload"
	}
	if strings.Contains(errStr, "invalid character") {
		return "Malformed JSON syntax"
	}
	return "Invalid request body"
}
