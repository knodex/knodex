// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"encoding/json"
	"testing"
)

func TestNewMessage(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		data     interface{}
		wantType MessageType
	}{
		{
			name:     "pong message",
			msgType:  MessageTypePong,
			data:     nil,
			wantType: MessageTypePong,
		},
		{
			name:     "error message",
			msgType:  MessageTypeError,
			data:     map[string]string{"error": "test"},
			wantType: MessageTypeError,
		},
		{
			name:     "instance update",
			msgType:  MessageTypeInstanceUpdate,
			data:     map[string]string{"name": "test"},
			wantType: MessageTypeInstanceUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewMessage(tt.msgType, tt.data)
			if err != nil {
				t.Fatalf("NewMessage failed: %v", err)
			}

			if msg.Type != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, msg.Type)
			}
		})
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg, err := NewErrorMessage("TEST_CODE", "Test error message")
	if err != nil {
		t.Fatalf("NewErrorMessage failed: %v", err)
	}

	if msg.Type != MessageTypeError {
		t.Errorf("expected type %s, got %s", MessageTypeError, msg.Type)
	}

	var errData ErrorData
	if err := json.Unmarshal(msg.Data, &errData); err != nil {
		t.Fatalf("failed to unmarshal error data: %v", err)
	}

	if errData.Code != "TEST_CODE" {
		t.Errorf("expected code TEST_CODE, got %s", errData.Code)
	}

	if errData.Message != "Test error message" {
		t.Errorf("expected message 'Test error message', got %s", errData.Message)
	}
}

func TestNewInstanceUpdateMessage(t *testing.T) {
	testInstance := map[string]interface{}{
		"name":      "test-instance",
		"namespace": "default",
	}

	msg, err := NewInstanceUpdateMessage(ActionAdd, "default", "WebApp", "test-instance", testInstance, "project-a")
	if err != nil {
		t.Fatalf("NewInstanceUpdateMessage failed: %v", err)
	}

	if msg.Type != MessageTypeInstanceUpdate {
		t.Errorf("expected type %s, got %s", MessageTypeInstanceUpdate, msg.Type)
	}

	var updateData InstanceUpdateData
	if err := json.Unmarshal(msg.Data, &updateData); err != nil {
		t.Fatalf("failed to unmarshal update data: %v", err)
	}

	if updateData.Action != ActionAdd {
		t.Errorf("expected action %s, got %s", ActionAdd, updateData.Action)
	}

	if updateData.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", updateData.Namespace)
	}

	if updateData.Kind != "WebApp" {
		t.Errorf("expected kind WebApp, got %s", updateData.Kind)
	}

	if updateData.Name != "test-instance" {
		t.Errorf("expected name test-instance, got %s", updateData.Name)
	}

	if updateData.ProjectID != "project-a" {
		t.Errorf("expected projectId project-a, got %s", updateData.ProjectID)
	}
}

func TestNewRGDUpdateMessage(t *testing.T) {
	testRGD := map[string]interface{}{
		"name": "test-rgd",
	}

	msg, err := NewRGDUpdateMessage(ActionUpdate, "test-rgd", testRGD, "project-b")
	if err != nil {
		t.Fatalf("NewRGDUpdateMessage failed: %v", err)
	}

	if msg.Type != MessageTypeRGDUpdate {
		t.Errorf("expected type %s, got %s", MessageTypeRGDUpdate, msg.Type)
	}

	var updateData RGDUpdateData
	if err := json.Unmarshal(msg.Data, &updateData); err != nil {
		t.Fatalf("failed to unmarshal update data: %v", err)
	}

	if updateData.Action != ActionUpdate {
		t.Errorf("expected action %s, got %s", ActionUpdate, updateData.Action)
	}

	if updateData.Name != "test-rgd" {
		t.Errorf("expected name test-rgd, got %s", updateData.Name)
	}

	if updateData.ProjectID != "project-b" {
		t.Errorf("expected projectId project-b, got %s", updateData.ProjectID)
	}
}

func TestMessage_Bytes(t *testing.T) {
	msg, err := NewMessage(MessageTypePong, nil)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	bytes, err := msg.Bytes()
	if err != nil {
		t.Fatalf("Bytes failed: %v", err)
	}

	// Should be valid JSON
	var decoded Message
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal bytes: %v", err)
	}

	if decoded.Type != MessageTypePong {
		t.Errorf("expected type %s, got %s", MessageTypePong, decoded.Type)
	}
}

func TestNewCountsUpdateMessage(t *testing.T) {
	msg, err := NewCountsUpdateMessage(42, 17)
	if err != nil {
		t.Fatalf("NewCountsUpdateMessage failed: %v", err)
	}

	if msg.Type != MessageTypeCountsUpdate {
		t.Errorf("expected type %s, got %s", MessageTypeCountsUpdate, msg.Type)
	}

	if msg.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	var data CountsUpdateData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal counts data: %v", err)
	}

	if data.RGDCount != 42 {
		t.Errorf("expected rgdCount 42, got %d", data.RGDCount)
	}
	if data.InstanceCount != 17 {
		t.Errorf("expected instanceCount 17, got %d", data.InstanceCount)
	}
}

func TestCountsUpdateMessage_Deserialization(t *testing.T) {
	// Create message, serialize, then deserialize
	msg, err := NewCountsUpdateMessage(5, 10)
	if err != nil {
		t.Fatalf("NewCountsUpdateMessage failed: %v", err)
	}

	bytes, err := msg.Bytes()
	if err != nil {
		t.Fatalf("Bytes failed: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decoded.Type != MessageTypeCountsUpdate {
		t.Errorf("expected type %s, got %s", MessageTypeCountsUpdate, decoded.Type)
	}

	var data CountsUpdateData
	if err := json.Unmarshal(decoded.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal counts data: %v", err)
	}

	if data.RGDCount != 5 {
		t.Errorf("expected rgdCount 5, got %d", data.RGDCount)
	}
	if data.InstanceCount != 10 {
		t.Errorf("expected instanceCount 10, got %d", data.InstanceCount)
	}
}

func TestNewViolationUpdateMessage(t *testing.T) {
	resource := ViolationResourceData{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test-pod",
		APIGroup:  "",
	}

	msg, err := NewViolationUpdateMessage(
		ActionAdd,
		"K8sRequiredLabels",
		"require-app-label",
		resource,
		"Missing required label: app",
		"deny",
	)
	if err != nil {
		t.Fatalf("NewViolationUpdateMessage failed: %v", err)
	}

	if msg.Type != MessageTypeViolationUpdate {
		t.Errorf("expected type %s, got %s", MessageTypeViolationUpdate, msg.Type)
	}

	var updateData ViolationUpdateData
	if err := json.Unmarshal(msg.Data, &updateData); err != nil {
		t.Fatalf("failed to unmarshal update data: %v", err)
	}

	if updateData.Action != ActionAdd {
		t.Errorf("expected action %s, got %s", ActionAdd, updateData.Action)
	}

	if updateData.ConstraintKind != "K8sRequiredLabels" {
		t.Errorf("expected constraintKind K8sRequiredLabels, got %s", updateData.ConstraintKind)
	}

	if updateData.ConstraintName != "require-app-label" {
		t.Errorf("expected constraintName require-app-label, got %s", updateData.ConstraintName)
	}

	if updateData.Resource.Kind != "Pod" {
		t.Errorf("expected resource kind Pod, got %s", updateData.Resource.Kind)
	}

	if updateData.Resource.Namespace != "default" {
		t.Errorf("expected resource namespace default, got %s", updateData.Resource.Namespace)
	}

	if updateData.Resource.Name != "test-pod" {
		t.Errorf("expected resource name test-pod, got %s", updateData.Resource.Name)
	}

	if updateData.Message != "Missing required label: app" {
		t.Errorf("expected message 'Missing required label: app', got %s", updateData.Message)
	}

	if updateData.EnforcementAction != "deny" {
		t.Errorf("expected enforcement action deny, got %s", updateData.EnforcementAction)
	}
}

func TestNewViolationUpdateMessage_Resolved(t *testing.T) {
	resource := ViolationResourceData{
		Kind:      "Deployment",
		Namespace: "production",
		Name:      "web-app",
		APIGroup:  "apps",
	}

	msg, err := NewViolationUpdateMessage(
		ActionDelete, // Resolved violation
		"K8sContainerRatios",
		"container-limits",
		resource,
		"Container resources exceed allowed ratios",
		"warn",
	)
	if err != nil {
		t.Fatalf("NewViolationUpdateMessage failed: %v", err)
	}

	if msg.Type != MessageTypeViolationUpdate {
		t.Errorf("expected type %s, got %s", MessageTypeViolationUpdate, msg.Type)
	}

	var updateData ViolationUpdateData
	if err := json.Unmarshal(msg.Data, &updateData); err != nil {
		t.Fatalf("failed to unmarshal update data: %v", err)
	}

	// Verify resolved action
	if updateData.Action != ActionDelete {
		t.Errorf("expected action %s, got %s", ActionDelete, updateData.Action)
	}

	// Verify API group is included
	if updateData.Resource.APIGroup != "apps" {
		t.Errorf("expected resource apiGroup apps, got %s", updateData.Resource.APIGroup)
	}

	// Verify enforcement action
	if updateData.EnforcementAction != "warn" {
		t.Errorf("expected enforcement action warn, got %s", updateData.EnforcementAction)
	}
}
