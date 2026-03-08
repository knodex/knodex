package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/auth"
	utilrand "github.com/knodex/knodex/server/internal/util/rand"
)

const (
	// wsTicketPrefix is the Redis key prefix for WebSocket tickets.
	wsTicketPrefix = "ws:ticket:"

	// wsTicketTTL is how long a WebSocket ticket is valid before it expires.
	wsTicketTTL = 30 * time.Second

	// wsTicketLength is the number of random bytes used to generate a ticket.
	// 32 bytes = 64 hex characters, providing 256 bits of entropy.
	wsTicketLength = 32
)

// wsTicketResponse is the JSON response for POST /api/v1/ws/ticket.
type wsTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresAt string `json:"expiresAt"`
}

// WSTicketHandler handles WebSocket ticket generation.
type WSTicketHandler struct {
	redisClient *redis.Client
}

// NewWSTicketHandler creates a new WSTicketHandler.
func NewWSTicketHandler(redisClient *redis.Client) *WSTicketHandler {
	return &WSTicketHandler{
		redisClient: redisClient,
	}
}

// CreateTicket handles POST /api/v1/ws/ticket.
// It generates an opaque, single-use ticket that can be exchanged for a WebSocket connection.
// The ticket is stored in Redis with a 30-second TTL.
func (h *WSTicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Generate cryptographically random ticket
	ticket, err := generateTicket()
	if err != nil {
		slog.Error("failed to generate WebSocket ticket", "error", err)
		response.InternalError(w, "failed to generate ticket")
		return
	}

	// Store user context in Redis: userID|email|groups|casbinRoles|projects
	// Groups and roles are pipe-separated, projects are pipe-separated
	value := encodeTicketValue(userCtx.UserID, userCtx.Email, userCtx.Groups, userCtx.CasbinRoles, userCtx.Projects)
	key := wsTicketPrefix + ticket

	err = h.redisClient.Set(r.Context(), key, value, wsTicketTTL).Err()
	if err != nil {
		slog.Error("failed to store WebSocket ticket in Redis",
			"error", err,
			"userID", userCtx.UserID,
		)
		response.InternalError(w, "failed to generate ticket")
		return
	}

	expiresAt := time.Now().Add(wsTicketTTL)

	slog.Info("WebSocket ticket generated",
		"userID", userCtx.UserID,
		"ticketPrefix", ticket[:8],
		"expiresAt", expiresAt.Format(time.RFC3339),
	)

	response.WriteJSON(w, http.StatusOK, wsTicketResponse{
		Ticket:    ticket,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

// ExchangeTicket atomically retrieves and deletes a WebSocket ticket from Redis.
// Returns userID, email, groups, casbinRoles, projects, or error if ticket is invalid/expired/reused.
func ExchangeTicket(ctx context.Context, redisClient *redis.Client, ticket string) (userID, email string, groups, casbinRoles, projects []string, err error) {
	key := wsTicketPrefix + ticket

	// GetDel is atomic: retrieve + delete in one operation (single-use guarantee)
	value, err := redisClient.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", "", nil, nil, nil, fmt.Errorf("invalid, expired, or already-used ticket")
	}
	if err != nil {
		return "", "", nil, nil, nil, fmt.Errorf("failed to exchange ticket: %w", err)
	}

	userID, email, groups, casbinRoles, projects, decodeErr := decodeTicketValue(value)
	if decodeErr != nil {
		return "", "", nil, nil, nil, fmt.Errorf("failed to decode ticket: %w", decodeErr)
	}
	return userID, email, groups, casbinRoles, projects, nil
}

// generateTicket creates a cryptographically random opaque ticket string.
func generateTicket() (string, error) {
	return utilrand.GenerateRandomHex(wsTicketLength), nil
}

// encodeTicketValue encodes user context into a Redis-storable string.
// Format: userID\x1femail\x1fgroup1\x1egroup2\x1frole1\x1erole2\x1fproj1\x1eproj2
// Uses \x1f (unit separator) as field delimiter, \x1e (record separator) as list delimiter.
// Both are ASCII control characters that cannot appear in OIDC group names, role names, or project names.
func encodeTicketValue(userID, email string, groups, casbinRoles, projects []string) string {
	return strings.Join([]string{
		userID,
		email,
		strings.Join(groups, "\x1e"),
		strings.Join(casbinRoles, "\x1e"),
		strings.Join(projects, "\x1e"),
	}, "\x1f")
}

// decodeTicketValue decodes a Redis-stored ticket value back into user context fields.
// Returns an error if the value is malformed (fewer than 5 fields or empty userID).
func decodeTicketValue(value string) (userID, email string, groups, casbinRoles, projects []string, err error) {
	parts := strings.Split(value, "\x1f")
	if len(parts) < 5 {
		return "", "", nil, nil, nil, fmt.Errorf("malformed ticket value: expected 5 fields, got %d", len(parts))
	}

	userID = parts[0]
	if strings.TrimSpace(userID) == "" {
		return "", "", nil, nil, nil, fmt.Errorf("malformed ticket value: empty userID")
	}

	email = parts[1]

	if parts[2] != "" {
		groups = strings.Split(parts[2], "\x1e")
	}
	if parts[3] != "" {
		casbinRoles = strings.Split(parts[3], "\x1e")
	}
	if parts[4] != "" {
		projects = strings.Split(parts[4], "\x1e")
	}

	return userID, email, groups, casbinRoles, projects, nil
}

// authCodeRequest is the JSON request for POST /api/v1/auth/token-exchange.
type authCodeRequest struct {
	Code string `json:"code"`
}

// authCodeResponse is the JSON response for POST /api/v1/auth/token-exchange.
// Token is delivered via HttpOnly cookie, response contains user info only.
type authCodeResponse struct {
	ExpiresAt time.Time     `json:"expiresAt"`
	User      auth.UserInfo `json:"user"`
}

// AuthCodeHandler handles opaque auth code exchange (OIDC callback replacement).
type AuthCodeHandler struct {
	redisClient  *redis.Client
	authService  auth.ServiceInterface
	cookieConfig cookie.Config
}

// NewAuthCodeHandler creates a new AuthCodeHandler.
func NewAuthCodeHandler(redisClient *redis.Client, authService auth.ServiceInterface, cookieCfg cookie.Config) *AuthCodeHandler {
	return &AuthCodeHandler{
		redisClient:  redisClient,
		authService:  authService,
		cookieConfig: cookieCfg,
	}
}

const (
	// authCodePrefix is the Redis key prefix for OIDC auth codes.
	authCodePrefix = "auth:code:"

	// authCodeTTL is how long an auth code is valid before it expires.
	authCodeTTL = 30 * time.Second
)

// StoreAuthCode stores a JWT in Redis with an opaque code key.
// Returns the opaque code that can be exchanged for the JWT.
func StoreAuthCode(ctx context.Context, redisClient *redis.Client, jwtToken string) (string, error) {
	code, err := generateTicket()
	if err != nil {
		return "", fmt.Errorf("failed to generate auth code: %w", err)
	}

	key := authCodePrefix + code
	err = redisClient.Set(ctx, key, jwtToken, authCodeTTL).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store auth code in Redis: %w", err)
	}

	return code, nil
}

// TokenExchange handles POST /api/v1/auth/token-exchange.
// Exchanges an opaque auth code for a JWT token.
func (h *AuthCodeHandler) TokenExchange(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1KB to prevent abuse (code + JSON overhead is ~100 bytes)
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	var req authCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.AuthBadRequest(w, "invalid request body", nil)
		return
	}

	if req.Code == "" {
		response.AuthBadRequest(w, "code is required", nil)
		return
	}

	key := authCodePrefix + req.Code

	// GetDel: atomic single-use exchange
	value, err := h.redisClient.GetDel(r.Context(), key).Result()
	if err == redis.Nil {
		slog.Warn("auth code exchange failed: invalid, expired, or already-used code",
			"codePrefix", req.Code[:min(8, len(req.Code))],
		)
		response.AuthUnauthorized(w, "invalid, expired, or already-used code")
		return
	}
	if err != nil {
		slog.Error("failed to exchange auth code", "error", err)
		response.AuthInternalError(w, "failed to exchange code")
		return
	}

	jwtToken := value

	// Validate the JWT to extract claims for user info response
	claims, err := h.authService.ValidateToken(r.Context(), jwtToken)
	if err != nil {
		slog.Error("failed to validate exchanged JWT", "error", err)
		response.AuthInternalError(w, "failed to process authentication token")
		return
	}

	slog.Info("auth code exchanged for token",
		"codePrefix", req.Code[:min(8, len(req.Code))],
	)

	// Set HttpOnly session cookie
	expiresAt := time.Unix(claims.ExpiresAt, 0)
	maxAge := time.Until(expiresAt)
	cookie.SetSession(w, jwtToken, maxAge, h.cookieConfig)

	// Return user info (token is in the cookie, not the response body)
	response.WriteAuthJSON(w, http.StatusOK, authCodeResponse{
		ExpiresAt: expiresAt,
		User: auth.UserInfo{
			ID:             claims.UserID,
			Email:          claims.Email,
			DisplayName:    claims.DisplayName,
			Projects:       claims.Projects,
			DefaultProject: claims.DefaultProject,
			Groups:         claims.Groups,
			Roles:          claims.Roles,
			CasbinRoles:    claims.CasbinRoles,
			Permissions:    claims.Permissions,
		},
	})
}
