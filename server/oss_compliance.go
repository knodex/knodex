// Package main provides OSS (non-enterprise) compliance stubs.
package main

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/websocket"
)

// InitComplianceService returns nil for OSS builds.
// The compliance feature requires an Enterprise license.
func InitComplianceService(_ context.Context, _ *config.Kubernetes, _ *websocket.Hub, _ *redis.Client, _ *config.Compliance) services.ComplianceService {
	return nil
}

// InitViolationHistoryService returns nil for OSS builds.
func InitViolationHistoryService() services.ViolationHistoryService {
	return nil
}
