// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package response

import (
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
)

// ErrorCode represents standardized API error codes
type ErrorCode string

const (
	// ErrCodeNotFound indicates the requested resource was not found
	ErrCodeNotFound ErrorCode = "NOT_FOUND"
	// ErrCodeBadRequest indicates invalid request parameters
	ErrCodeBadRequest ErrorCode = "BAD_REQUEST"
	// ErrCodeUnauthorized indicates authentication is required or failed
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	// ErrCodeForbidden indicates the request is forbidden
	ErrCodeForbidden ErrorCode = "FORBIDDEN"
	// ErrCodeServiceUnavailable indicates the service is temporarily unavailable
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	// ErrCodeInternalError indicates an internal server error
	ErrCodeInternalError ErrorCode = "INTERNAL_ERROR"
	// ErrCodeValidationFailed indicates input validation failed
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	// ErrCodeRateLimit indicates too many requests from the same source
	ErrCodeRateLimit ErrorCode = "RATE_LIMIT_EXCEEDED"
	// ErrCodeMethodNotAllowed indicates the HTTP method is not allowed for this endpoint
	ErrCodeMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"
	// ErrCodeLicenseRequired indicates a valid enterprise license is required for the feature
	ErrCodeLicenseRequired ErrorCode = "LICENSE_REQUIRED"
	// ErrCodeConflict indicates a resource was modified concurrently and the request should be retried
	ErrCodeConflict ErrorCode = "CONFLICT"
)

// ErrorResponse represents a standardized API error response
type ErrorResponse struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// WriteError writes a standardized error response.
// HTML-encodes the message and detail values to prevent stored XSS
// when error responses are rendered in browser contexts.
func WriteError(w http.ResponseWriter, statusCode int, code ErrorCode, message string, details map[string]string) {
	// HTML-encode message to prevent XSS when rendered in browser
	safeMessage := html.EscapeString(message)

	// HTML-encode all detail keys and values
	var safeDetails map[string]string
	if details != nil {
		safeDetails = make(map[string]string, len(details))
		for k, v := range details {
			safeDetails[html.EscapeString(k)] = html.EscapeString(v)
		}
	}

	errResp := ErrorResponse{
		Code:    code,
		Message: safeMessage,
		Details: safeDetails,
	}
	WriteJSON(w, statusCode, errResp)
}

// NotFound writes a 404 error response
func NotFound(w http.ResponseWriter, resource, identifier string) {
	WriteError(w, http.StatusNotFound, ErrCodeNotFound,
		resource+" not found: "+identifier,
		map[string]string{"resource": resource, "identifier": identifier})
}

// BadRequest writes a 400 error response
func BadRequest(w http.ResponseWriter, message string, details map[string]string) {
	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, message, details)
}

// Unauthorized writes a 401 error response
func Unauthorized(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

// Forbidden writes a 403 error response
func Forbidden(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusForbidden, ErrCodeForbidden, message, nil)
}

// ServiceUnavailable writes a 503 error response
func ServiceUnavailable(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusServiceUnavailable, ErrCodeServiceUnavailable, message, nil)
}

// InternalError writes a 500 error response
func InternalError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, message, nil)
}

// MethodNotAllowed writes a 405 error response
func MethodNotAllowed(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusMethodNotAllowed, ErrCodeMethodNotAllowed, message, nil)
}

// setNoCacheHeaders sets cache-control headers to prevent caching of auth responses.
// This prevents JWT tokens from being replayed from browser disk cache on shared workstations.
func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// WriteAuthJSON writes a JSON response with no-cache headers for authentication endpoints.
func WriteAuthJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	setNoCacheHeaders(w)
	WriteJSON(w, statusCode, data)
}

// WriteAuthError writes a standardized error response with no-cache headers for authentication endpoints.
func WriteAuthError(w http.ResponseWriter, statusCode int, code ErrorCode, message string, details map[string]string) {
	setNoCacheHeaders(w)
	WriteError(w, statusCode, code, message, details)
}

// AuthBadRequest writes a 400 error response with no-cache headers for authentication endpoints.
func AuthBadRequest(w http.ResponseWriter, message string, details map[string]string) {
	WriteAuthError(w, http.StatusBadRequest, ErrCodeBadRequest, message, details)
}

// AuthUnauthorized writes a 401 error response with no-cache headers for authentication endpoints.
func AuthUnauthorized(w http.ResponseWriter, message string) {
	WriteAuthError(w, http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

// AuthInternalError writes a 500 error response with no-cache headers for authentication endpoints.
func AuthInternalError(w http.ResponseWriter, message string) {
	WriteAuthError(w, http.StatusInternalServerError, ErrCodeInternalError, message, nil)
}
