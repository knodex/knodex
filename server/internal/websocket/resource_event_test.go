// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResourceEventMessage_Structure(t *testing.T) {
	t.Parallel()
	msg, err := NewResourceEventMessage("inst-123", "Deployment", "my-deploy", "created", "", "acme")
	require.NoError(t, err)

	assert.Equal(t, MessageTypeResourceEvent, msg.Type)
	assert.False(t, msg.Timestamp.IsZero())

	var data ResourceEventData
	require.NoError(t, json.Unmarshal(msg.Data, &data))
	assert.Equal(t, "inst-123", data.InstanceID)
	assert.Equal(t, "Deployment", data.ResourceKind)
	assert.Equal(t, "my-deploy", data.ResourceName)
	assert.Equal(t, "created", data.Status)
	assert.Empty(t, data.Message)
	assert.Equal(t, "acme", data.ProjectID)
}

func TestNewResourceEventMessage_WithErrorMessage(t *testing.T) {
	t.Parallel()
	msg, err := NewResourceEventMessage("inst-123", "Service", "my-svc", "failed", "connection timeout", "")
	require.NoError(t, err)

	var data ResourceEventData
	require.NoError(t, json.Unmarshal(msg.Data, &data))
	assert.Equal(t, "failed", data.Status)
	assert.Equal(t, "connection timeout", data.Message)
}

func TestResourceEventMessage_Serializable(t *testing.T) {
	t.Parallel()
	msg, err := NewResourceEventMessage("inst-789", "ConfigMap", "my-cm", "creating", "", "")
	require.NoError(t, err)

	bytes, err := msg.Bytes()
	require.NoError(t, err)

	// Verify it round-trips through JSON
	var parsed Message
	require.NoError(t, json.Unmarshal(bytes, &parsed))
	assert.Equal(t, MessageTypeResourceEvent, parsed.Type)
}

func TestResourceEventType_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, MessageType("resource_event"), MessageTypeResourceEvent)
	assert.Equal(t, MessageType("deploy_progress"), MessageTypeDeployProgress)
}

func TestExistingInstanceStatus_Unchanged(t *testing.T) {
	t.Parallel()
	// Verify existing message types are still present and unchanged
	assert.Equal(t, MessageType("instance_update"), MessageTypeInstanceUpdate)
	assert.Equal(t, MessageType("rgd_update"), MessageTypeRGDUpdate)
	assert.Equal(t, MessageType("violation_update"), MessageTypeViolationUpdate)
	assert.Equal(t, MessageType("counts_update"), MessageTypeCountsUpdate)
	assert.Equal(t, MessageType("drift_update"), MessageTypeDriftUpdate)
}
