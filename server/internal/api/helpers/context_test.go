package helpers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provops-org/knodex/server/internal/api/middleware"
)

// setUserContext is a helper function for tests to set user context
func setUserContext(r *http.Request, userCtx *middleware.UserContext) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, userCtx)
	return r.WithContext(ctx)
}

func TestRequireUserContext(t *testing.T) {
	t.Run("returns user context when present", func(t *testing.T) {
		expectedCtx := &middleware.UserContext{
			UserID: "user@example.com",
			Email:  "user@example.com",
			Groups: []string{"group1"},
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = setUserContext(req, expectedCtx)
		w := httptest.NewRecorder()

		result := RequireUserContext(w, req)

		require.NotNil(t, result)
		assert.Equal(t, expectedCtx.UserID, result.UserID)
		assert.Equal(t, expectedCtx.Email, result.Email)
		assert.Equal(t, 200, w.Code) // No response written
	})

	t.Run("returns nil and writes 401 when context missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		result := RequireUserContext(w, req)

		assert.Nil(t, result)
		assert.Equal(t, 401, w.Code)

		var resp struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "UNAUTHORIZED", resp.Code)
		assert.Equal(t, "Authentication required", resp.Message)
	})
}
