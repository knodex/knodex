// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// DeploymentEventType represents the type of deployment event
type DeploymentEventType string

const (
	// EventTypeCreated when instance is first created
	EventTypeCreated DeploymentEventType = "Created"
	// EventTypeManifestGenerated when manifest is generated for GitOps
	EventTypeManifestGenerated DeploymentEventType = "ManifestGenerated"
	// EventTypePushedToGit when manifest is pushed to Git repository
	EventTypePushedToGit DeploymentEventType = "PushedToGit"
	// EventTypeWaitingForSync when waiting for GitOps tool to sync
	EventTypeWaitingForSync DeploymentEventType = "WaitingForSync"
	// EventTypeSynced when GitOps tool synced the manifest
	EventTypeSynced DeploymentEventType = "Synced"
	// EventTypeCreating when resources are being created
	EventTypeCreating DeploymentEventType = "Creating"
	// EventTypeReady when instance is fully ready
	EventTypeReady DeploymentEventType = "Ready"
	// EventTypeDegraded when instance has degraded health
	EventTypeDegraded DeploymentEventType = "Degraded"
	// EventTypeFailed when deployment failed
	EventTypeFailed DeploymentEventType = "Failed"
	// EventTypeDeleted when instance is deleted
	EventTypeDeleted DeploymentEventType = "Deleted"
	// EventTypeUpdated when instance spec is updated
	EventTypeUpdated DeploymentEventType = "Updated"
	// EventTypeStatusChanged when instance status changes
	EventTypeStatusChanged DeploymentEventType = "StatusChanged"
)

// DeploymentMode represents how the instance was deployed
type DeploymentMode string

const (
	// DeploymentModeDirect deploys directly to the cluster
	DeploymentModeDirect DeploymentMode = "direct"
	// DeploymentModeGitOps deploys via GitOps (push to Git only)
	DeploymentModeGitOps DeploymentMode = "gitops"
	// DeploymentModeHybrid deploys to cluster and pushes to Git
	DeploymentModeHybrid DeploymentMode = "hybrid"
)

// DeploymentEvent represents a single event in the deployment history
type DeploymentEvent struct {
	// ID is the unique identifier for this event
	ID string `json:"id"`
	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`
	// EventType is the type of event
	EventType DeploymentEventType `json:"eventType"`
	// Status is the instance status at this point
	Status string `json:"status"`
	// User who triggered the event (email or system)
	User string `json:"user,omitempty"`
	// DeploymentMode is how the instance was deployed
	DeploymentMode DeploymentMode `json:"deploymentMode,omitempty"`
	// GitCommitSHA is the Git commit SHA (for GitOps/Hybrid)
	GitCommitSHA string `json:"gitCommitSha,omitempty"`
	// GitRepository is the repository URL (for GitOps/Hybrid)
	GitRepository string `json:"gitRepository,omitempty"`
	// GitBranch is the Git branch (for GitOps/Hybrid)
	GitBranch string `json:"gitBranch,omitempty"`
	// Details contains additional event details
	Details map[string]interface{} `json:"details,omitempty"`
	// Message is a human-readable description of the event
	Message string `json:"message,omitempty"`
}

// DeploymentHistory contains the full deployment history for an instance
type DeploymentHistory struct {
	// InstanceID is the unique identifier for the instance
	InstanceID string `json:"instanceId"`
	// InstanceName is the name of the instance
	InstanceName string `json:"instanceName"`
	// Namespace is the instance namespace
	Namespace string `json:"namespace"`
	// RGDName is the name of the RGD
	RGDName string `json:"rgdName"`
	// Events is the list of deployment events, ordered by timestamp
	Events []DeploymentEvent `json:"events"`
	// CreatedAt is when the instance was first created
	CreatedAt time.Time `json:"createdAt"`
	// CurrentStatus is the current status of the instance
	CurrentStatus string `json:"currentStatus"`
	// DeploymentMode is the deployment mode used
	DeploymentMode DeploymentMode `json:"deploymentMode"`
	// LastGitCommit is the most recent Git commit SHA (if any)
	LastGitCommit string `json:"lastGitCommit,omitempty"`
}

// GetGitHubCommitURL returns the GitHub commit URL if repository and SHA are available
func (e *DeploymentEvent) GetGitHubCommitURL() string {
	if e.GitCommitSHA == "" || e.GitRepository == "" {
		return ""
	}
	// Format: https://github.com/{owner}/{repo}/commit/{sha}
	return fmt.Sprintf("%s/commit/%s", e.GitRepository, e.GitCommitSHA)
}

// HistoryExportFormat represents the export format for history
type HistoryExportFormat string

const (
	ExportFormatCSV  HistoryExportFormat = "csv"
	ExportFormatJSON HistoryExportFormat = "json"
)

// ExportToCSV writes the deployment history to CSV format
func (h *DeploymentHistory) ExportToCSV(w io.Writer) error {
	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	// Write header
	header := []string{
		"Timestamp",
		"Event",
		"Status",
		"User",
		"Deployment Mode",
		"Git Commit",
		"Git Repository",
		"Git Branch",
		"Message",
		"Details",
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write events
	for _, event := range h.Events {
		detailsJSON := ""
		if event.Details != nil {
			detailsBytes, _ := json.Marshal(event.Details)
			detailsJSON = string(detailsBytes)
		}

		row := []string{
			event.Timestamp.Format(time.RFC3339),
			string(event.EventType),
			event.Status,
			event.User,
			string(event.DeploymentMode),
			event.GitCommitSHA,
			event.GitRepository,
			event.GitBranch,
			event.Message,
			detailsJSON,
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// ExportToJSON writes the deployment history to JSON format
func (h *DeploymentHistory) ExportToJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(h)
}

// TimelineEntry represents a simplified timeline entry for UI display
type TimelineEntry struct {
	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`
	// EventType is the type of event
	EventType DeploymentEventType `json:"eventType"`
	// Status is the instance status at this point
	Status string `json:"status"`
	// User who triggered the event
	User string `json:"user,omitempty"`
	// Message is a human-readable description
	Message string `json:"message,omitempty"`
	// GitCommitURL is the link to the Git commit (if applicable)
	GitCommitURL string `json:"gitCommitUrl,omitempty"`
	// IsCompleted indicates if this step is completed
	IsCompleted bool `json:"isCompleted"`
	// IsCurrent indicates if this is the current state
	IsCurrent bool `json:"isCurrent"`
}

// GetTimeline returns a simplified timeline for UI display
func (h *DeploymentHistory) GetTimeline() []TimelineEntry {
	timeline := make([]TimelineEntry, 0, len(h.Events))

	for i, event := range h.Events {
		entry := TimelineEntry{
			Timestamp:   event.Timestamp,
			EventType:   event.EventType,
			Status:      event.Status,
			User:        event.User,
			Message:     event.Message,
			IsCompleted: true,
			IsCurrent:   i == len(h.Events)-1,
		}

		// Add Git commit URL if available
		if event.GitCommitSHA != "" {
			entry.GitCommitURL = event.GetGitHubCommitURL()
		}

		timeline = append(timeline, entry)
	}

	return timeline
}
