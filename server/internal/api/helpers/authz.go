package helpers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// CheckAccess performs Casbin authorization check.
// Returns (allowed, error). Does not write response.
// Uses Authorizer interface for ISP compliance.
func CheckAccess(
	ctx context.Context,
	enforcer rbac.Authorizer,
	userCtx *middleware.UserContext,
	object, action string,
) (bool, error) {
	if enforcer == nil {
		return false, nil // Fail closed when enforcer unavailable
	}
	return enforcer.CanAccessWithGroups(
		ctx,
		userCtx.UserID,
		userCtx.Groups,
		object,
		action,
	)
}

// RequireAccess checks authorization and writes 403/500 response if denied.
// Returns true if access granted, false if denied (response already written).
// Uses Authorizer interface for ISP compliance.
func RequireAccess(
	w http.ResponseWriter,
	ctx context.Context,
	enforcer rbac.Authorizer,
	userCtx *middleware.UserContext,
	object, action string,
	requestID string,
) bool {
	allowed, err := CheckAccess(ctx, enforcer, userCtx, object, action)
	if err != nil {
		slog.Error("authorization check failed",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"object", object,
			"action", action,
			"error", err,
		)
		response.InternalError(w, "Failed to check authorization")
		return false
	}
	if !allowed {
		slog.Warn("access denied",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"object", object,
			"action", action,
		)
		response.Forbidden(w, "Insufficient permissions")
		return false
	}
	return true
}
