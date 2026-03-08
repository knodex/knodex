// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package history provides deployment history tracking and retrieval
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// historyKeyPrefix is the Redis key prefix for deployment history
	historyKeyPrefix = "deployment:history:"
	// historyTTL is how long to keep history (90 days)
	historyTTL = 90 * 24 * time.Hour
	// deletedHistoryKeyPrefix is the Redis key prefix for deleted instance history
	deletedHistoryKeyPrefix = "deployment:history:deleted:"
	// maxEventsPerInstance is the maximum number of events to store per instance
	maxEventsPerInstance = 1000
)

// Service handles deployment history tracking and retrieval
type Service struct {
	redisClient *redis.Client
	mu          sync.RWMutex
	// inMemoryCache is used when Redis is not available
	inMemoryCache map[string]*models.DeploymentHistory
}

// NewService creates a new history service
func NewService(redisClient *redis.Client) *Service {
	return &Service{
		redisClient:   redisClient,
		inMemoryCache: make(map[string]*models.DeploymentHistory),
	}
}

// historyKey returns the Redis key for an instance's history
func historyKey(namespace, kind, name string) string {
	// Sanitize inputs to prevent Redis pattern injection
	return fmt.Sprintf("%s%s/%s/%s", historyKeyPrefix, sanitize.RedisKey(namespace), sanitize.RedisKey(kind), sanitize.RedisKey(name))
}

// deletedHistoryKey returns the Redis key for a deleted instance's history
func deletedHistoryKey(instanceID string) string {
	// Sanitize input to prevent Redis pattern injection
	return fmt.Sprintf("%s%s", deletedHistoryKeyPrefix, sanitize.RedisKey(instanceID))
}

// RecordEvent records a deployment event for an instance
func (s *Service) RecordEvent(ctx context.Context, namespace, kind, name string, event models.DeploymentEvent) error {
	// Generate event ID if not set
	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Get existing history or create new
	history, err := s.GetHistory(ctx, namespace, kind, name)
	if err != nil {
		// Create new history if not found
		history = &models.DeploymentHistory{
			InstanceID:   fmt.Sprintf("%s/%s/%s", namespace, kind, name),
			InstanceName: name,
			Namespace:    namespace,
			Events:       []models.DeploymentEvent{},
			CreatedAt:    event.Timestamp,
		}
	}

	// Add event to history
	history.Events = append(history.Events, event)

	// Trim events if exceeding max
	if len(history.Events) > maxEventsPerInstance {
		history.Events = history.Events[len(history.Events)-maxEventsPerInstance:]
	}

	// Update current status and metadata
	history.CurrentStatus = event.Status
	if event.DeploymentMode != "" {
		history.DeploymentMode = event.DeploymentMode
	}
	if event.GitCommitSHA != "" {
		history.LastGitCommit = event.GitCommitSHA
	}

	// Save history
	return s.saveHistory(ctx, namespace, kind, name, history)
}

// RecordCreation records an instance creation event
func (s *Service) RecordCreation(ctx context.Context, namespace, kind, name, rgdName, user string, deploymentMode models.DeploymentMode) error {
	event := models.DeploymentEvent{
		Timestamp:      time.Now().UTC(),
		EventType:      models.EventTypeCreated,
		Status:         "Pending",
		User:           user,
		DeploymentMode: deploymentMode,
		Message:        fmt.Sprintf("Instance %s created", name),
		Details: map[string]interface{}{
			"rgdName":   rgdName,
			"namespace": namespace,
		},
	}

	// Also set RGDName on the history
	history, err := s.GetHistory(ctx, namespace, kind, name)
	if err != nil || history == nil {
		history = &models.DeploymentHistory{
			InstanceID:     fmt.Sprintf("%s/%s/%s", namespace, kind, name),
			InstanceName:   name,
			Namespace:      namespace,
			RGDName:        rgdName,
			Events:         []models.DeploymentEvent{},
			CreatedAt:      event.Timestamp,
			DeploymentMode: deploymentMode,
		}
	}
	history.RGDName = rgdName

	// Add event
	history.Events = append(history.Events, event)
	history.CurrentStatus = event.Status
	history.DeploymentMode = deploymentMode

	return s.saveHistory(ctx, namespace, kind, name, history)
}

// RecordGitPush records a Git push event
func (s *Service) RecordGitPush(ctx context.Context, namespace, kind, name, commitSHA, repository, branch, user string) error {
	event := models.DeploymentEvent{
		Timestamp:     time.Now().UTC(),
		EventType:     models.EventTypePushedToGit,
		Status:        "PushedToGit",
		User:          user,
		GitCommitSHA:  commitSHA,
		GitRepository: repository,
		GitBranch:     branch,
		Message:       fmt.Sprintf("Manifest pushed to %s", repository),
		Details: map[string]interface{}{
			"commit":     commitSHA,
			"repository": repository,
			"branch":     branch,
		},
	}

	return s.RecordEvent(ctx, namespace, kind, name, event)
}

// RecordStatusChange records a status change event
func (s *Service) RecordStatusChange(ctx context.Context, namespace, kind, name, oldStatus, newStatus string) error {
	// Determine event type based on new status
	eventType := models.EventTypeStatusChanged
	switch newStatus {
	case "Ready":
		eventType = models.EventTypeReady
	case "Creating", "Progressing":
		eventType = models.EventTypeCreating
	case "Degraded":
		eventType = models.EventTypeDegraded
	case "Failed":
		eventType = models.EventTypeFailed
	}

	event := models.DeploymentEvent{
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		Status:    newStatus,
		User:      "system",
		Message:   fmt.Sprintf("Status changed from %s to %s", oldStatus, newStatus),
		Details: map[string]interface{}{
			"previousStatus": oldStatus,
			"newStatus":      newStatus,
		},
	}

	return s.RecordEvent(ctx, namespace, kind, name, event)
}

// RecordDeletion records an instance deletion event and preserves history
func (s *Service) RecordDeletion(ctx context.Context, namespace, kind, name, user string) error {
	// Get existing history
	history, err := s.GetHistory(ctx, namespace, kind, name)
	if err != nil {
		slog.Warn("Failed to get history for deletion record",
			"namespace", namespace,
			"kind", kind,
			"name", name,
			"error", err,
		)
		// Still clean up the active key even when history doesn't exist,
		// to prevent orphaned Redis keys with ~90-day TTL.
		s.deleteActiveKey(ctx, namespace, kind, name)
		return nil // Don't fail deletion if history retrieval fails
	}

	// Add deletion event
	event := models.DeploymentEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		EventType: models.EventTypeDeleted,
		Status:    "Deleted",
		User:      user,
		Message:   fmt.Sprintf("Instance %s deleted", name),
	}
	history.Events = append(history.Events, event)
	history.CurrentStatus = "Deleted"

	// Move to deleted history storage (persisted)
	if s.redisClient != nil {
		data, err := json.Marshal(history)
		if err != nil {
			return fmt.Errorf("failed to marshal deleted history: %w", err)
		}

		// Store in deleted history with extended TTL
		err = s.redisClient.Set(ctx, deletedHistoryKey(history.InstanceID), data, historyTTL).Err()
		if err != nil {
			slog.Error("Failed to store deleted history in Redis",
				"instanceId", history.InstanceID,
				"error", err,
			)
		}

		// Delete the active history key
		s.redisClient.Del(ctx, historyKey(namespace, kind, name))
	} else {
		// In-memory: move to deleted cache
		s.mu.Lock()
		delete(s.inMemoryCache, historyKey(namespace, kind, name))
		s.inMemoryCache[deletedHistoryKey(history.InstanceID)] = history
		s.mu.Unlock()
	}

	return nil
}

// deleteActiveKey removes the active history Redis key for an instance.
// This is used as a safety net to prevent orphaned keys when history retrieval fails.
func (s *Service) deleteActiveKey(ctx context.Context, namespace, kind, name string) {
	key := historyKey(namespace, kind, name)
	if s.redisClient != nil {
		s.redisClient.Del(ctx, key)
	} else {
		s.mu.Lock()
		delete(s.inMemoryCache, key)
		s.mu.Unlock()
	}
}

// GetHistory retrieves the deployment history for an instance
func (s *Service) GetHistory(ctx context.Context, namespace, kind, name string) (*models.DeploymentHistory, error) {
	key := historyKey(namespace, kind, name)

	if s.redisClient != nil {
		data, err := s.redisClient.Get(ctx, key).Bytes()
		if err != nil {
			if err == redis.Nil {
				return nil, fmt.Errorf("history not found for %s/%s/%s", namespace, kind, name)
			}
			return nil, fmt.Errorf("failed to get history from Redis: %w", err)
		}

		var history models.DeploymentHistory
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, fmt.Errorf("failed to unmarshal history: %w", err)
		}

		return &history, nil
	}

	// Fall back to in-memory cache
	s.mu.RLock()
	defer s.mu.RUnlock()

	if history, ok := s.inMemoryCache[key]; ok {
		return history, nil
	}

	return nil, fmt.Errorf("history not found for %s/%s/%s", namespace, kind, name)
}

// GetDeletedHistory retrieves the history for a deleted instance
func (s *Service) GetDeletedHistory(ctx context.Context, instanceID string) (*models.DeploymentHistory, error) {
	key := deletedHistoryKey(instanceID)

	if s.redisClient != nil {
		data, err := s.redisClient.Get(ctx, key).Bytes()
		if err != nil {
			if err == redis.Nil {
				return nil, fmt.Errorf("deleted history not found for %s", instanceID)
			}
			return nil, fmt.Errorf("failed to get deleted history from Redis: %w", err)
		}

		var history models.DeploymentHistory
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, fmt.Errorf("failed to unmarshal deleted history: %w", err)
		}

		return &history, nil
	}

	// Fall back to in-memory cache
	s.mu.RLock()
	defer s.mu.RUnlock()

	if history, ok := s.inMemoryCache[key]; ok {
		return history, nil
	}

	return nil, fmt.Errorf("deleted history not found for %s", instanceID)
}

// GetTimeline returns a simplified timeline for an instance
func (s *Service) GetTimeline(ctx context.Context, namespace, kind, name string) ([]models.TimelineEntry, error) {
	history, err := s.GetHistory(ctx, namespace, kind, name)
	if err != nil {
		return nil, err
	}

	return history.GetTimeline(), nil
}

// saveHistory saves the deployment history to storage
func (s *Service) saveHistory(ctx context.Context, namespace, kind, name string, history *models.DeploymentHistory) error {
	key := historyKey(namespace, kind, name)

	// Sort events by timestamp
	sort.Slice(history.Events, func(i, j int) bool {
		return history.Events[i].Timestamp.Before(history.Events[j].Timestamp)
	})

	if s.redisClient != nil {
		data, err := json.Marshal(history)
		if err != nil {
			return fmt.Errorf("failed to marshal history: %w", err)
		}

		err = s.redisClient.Set(ctx, key, data, historyTTL).Err()
		if err != nil {
			return fmt.Errorf("failed to save history to Redis: %w", err)
		}

		return nil
	}

	// Fall back to in-memory cache
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inMemoryCache[key] = history

	return nil
}

// ListAllHistories lists all deployment histories (for admin purposes)
func (s *Service) ListAllHistories(ctx context.Context) ([]*models.DeploymentHistory, error) {
	var histories []*models.DeploymentHistory

	if s.redisClient != nil {
		// Scan for all history keys
		pattern := historyKeyPrefix + "*"
		iter := s.redisClient.Scan(ctx, 0, pattern, 0).Iterator()

		for iter.Next(ctx) {
			key := iter.Val()
			data, err := s.redisClient.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}

			var history models.DeploymentHistory
			if err := json.Unmarshal(data, &history); err != nil {
				continue
			}

			histories = append(histories, &history)
		}

		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("failed to scan histories: %w", err)
		}

		return histories, nil
	}

	// Fall back to in-memory cache
	s.mu.RLock()
	defer s.mu.RUnlock()

	for key, history := range s.inMemoryCache {
		// Only include active histories, not deleted ones
		if len(key) > len(historyKeyPrefix) && key[:len(historyKeyPrefix)] == historyKeyPrefix {
			if len(key) > len(deletedHistoryKeyPrefix) && key[:len(deletedHistoryKeyPrefix)] == deletedHistoryKeyPrefix {
				continue
			}
			histories = append(histories, history)
		}
	}

	return histories, nil
}

// CreateHistoryFromInstance creates or updates history from an existing instance
// This is used to populate history for instances that were created before history tracking
func (s *Service) CreateHistoryFromInstance(ctx context.Context, instance *models.Instance, user string) error {
	// Check if history already exists
	existing, _ := s.GetHistory(ctx, instance.Namespace, instance.Kind, instance.Name)
	if existing != nil && len(existing.Events) > 0 {
		// History already exists, don't overwrite
		return nil
	}

	// Create initial history
	history := &models.DeploymentHistory{
		InstanceID:     instance.UID,
		InstanceName:   instance.Name,
		Namespace:      instance.Namespace,
		RGDName:        instance.RGDName,
		CreatedAt:      instance.CreatedAt,
		CurrentStatus:  string(instance.Health),
		DeploymentMode: models.DeploymentModeDirect, // Default to direct mode
		Events: []models.DeploymentEvent{
			{
				ID:             uuid.New().String(),
				Timestamp:      instance.CreatedAt,
				EventType:      models.EventTypeCreated,
				Status:         "Pending",
				User:           user,
				DeploymentMode: models.DeploymentModeDirect,
				Message:        fmt.Sprintf("Instance %s created", instance.Name),
			},
		},
	}

	// If instance is ready, add ready event
	if instance.Health == models.HealthHealthy {
		history.Events = append(history.Events, models.DeploymentEvent{
			ID:        uuid.New().String(),
			Timestamp: instance.UpdatedAt,
			EventType: models.EventTypeReady,
			Status:    "Ready",
			User:      "system",
			Message:   "Instance ready",
		})
		history.CurrentStatus = "Ready"
	}

	return s.saveHistory(ctx, instance.Namespace, instance.Kind, instance.Name, history)
}
