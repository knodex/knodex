// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import "github.com/knodex/knodex/server/internal/models"

// mockGraphRevisionProvider implements services.GraphRevisionProvider for testing.
// Shared across history_test.go and revision_handler_test.go.
type mockGraphRevisionProvider struct {
	revisions map[string][]models.GraphRevision
}

func (m *mockGraphRevisionProvider) ListRevisions(rgdName string) models.GraphRevisionList {
	revs := m.revisions[rgdName]
	if revs == nil {
		return models.GraphRevisionList{Items: []models.GraphRevision{}, TotalCount: 0}
	}
	return models.GraphRevisionList{Items: revs, TotalCount: len(revs)}
}

func (m *mockGraphRevisionProvider) GetRevision(rgdName string, revision int) (*models.GraphRevision, bool) {
	for _, r := range m.revisions[rgdName] {
		if r.RevisionNumber == revision {
			return &r, true
		}
	}
	return nil, false
}

func (m *mockGraphRevisionProvider) GetLatestRevision(rgdName string) (*models.GraphRevision, bool) {
	revs := m.revisions[rgdName]
	if len(revs) == 0 {
		return nil, false
	}
	return &revs[0], true
}
