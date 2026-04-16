// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGraphRevision_JSON(t *testing.T) {
	rev := GraphRevision{
		RevisionNumber:  3,
		RGDName:         "my-webapp",
		Namespace:       "default",
		ContentHash:     "abc123",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ResourceVersion: "12345",
		Conditions: []GraphRevisionCondition{
			{Type: "GraphVerified", Status: "True"},
			{Type: "Ready", Status: "True"},
		},
	}

	data, err := json.Marshal(rev)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded GraphRevision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.RevisionNumber != 3 {
		t.Errorf("RevisionNumber = %d, want 3", decoded.RevisionNumber)
	}
	if decoded.RGDName != "my-webapp" {
		t.Errorf("RGDName = %q, want %q", decoded.RGDName, "my-webapp")
	}
	if len(decoded.Conditions) != 2 {
		t.Errorf("Conditions len = %d, want 2", len(decoded.Conditions))
	}
}

func TestGraphRevision_SnapshotOmitEmpty(t *testing.T) {
	rev := GraphRevision{
		RevisionNumber: 1,
		RGDName:        "test",
	}

	data, _ := json.Marshal(rev)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["snapshot"]; ok {
		t.Error("snapshot should be omitted when nil")
	}
}

func TestGraphRevisionList_EmptyItems(t *testing.T) {
	list := GraphRevisionList{
		Items:      []GraphRevision{},
		TotalCount: 0,
	}

	data, _ := json.Marshal(list)
	var decoded GraphRevisionList
	json.Unmarshal(data, &decoded)

	if decoded.Items == nil {
		t.Error("Items should be empty slice, not nil")
	}
	if decoded.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", decoded.TotalCount)
	}
}
