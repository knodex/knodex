package helpers

import (
	"errors"
	"strings"

	"github.com/provops-org/knodex/server/internal/rbac"
)

// IsNotFoundError checks if an error indicates a resource was not found.
// Consolidates the different implementations across handlers.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check specific error types first
	if errors.Is(err, rbac.ErrNotFound) || errors.Is(err, rbac.ErrProjectNotFound) {
		return true
	}
	// Fall back to string matching for k8s errors
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "notfound")
}
