// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SystemNamespaces contains namespaces that should be excluded by default
var SystemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

// NamespaceService provides operations on Kubernetes namespaces
type NamespaceService struct {
	k8sClient      kubernetes.Interface
	projectService ProjectServiceInterface
}

// NewNamespaceService creates a new NamespaceService
func NewNamespaceService(k8sClient kubernetes.Interface, projectService ProjectServiceInterface) *NamespaceService {
	return &NamespaceService{
		k8sClient:      k8sClient,
		projectService: projectService,
	}
}

// ListNamespaces returns all namespaces on the cluster, optionally excluding system namespaces
func (s *NamespaceService) ListNamespaces(ctx context.Context, excludeSystem bool) ([]string, error) {
	namespaceList, err := s.k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		// Skip system namespaces if excludeSystem is true
		if excludeSystem && SystemNamespaces[ns.Name] {
			continue
		}
		namespaces = append(namespaces, ns.Name)
	}

	// Sort namespaces alphabetically for consistent output
	sort.Strings(namespaces)

	return namespaces, nil
}

// ListProjectNamespaces returns namespaces that match a project's destination patterns
// It queries real Kubernetes namespaces and filters them against the project's allowed destinations
func (s *NamespaceService) ListProjectNamespaces(ctx context.Context, projectName string) ([]string, error) {
	// Get the project
	project, err := s.projectService.GetProject(ctx, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectName, err)
	}

	// If project has no destinations, return empty list
	if len(project.Spec.Destinations) == 0 {
		return []string{}, nil
	}

	// Get all namespaces from the cluster (excluding system namespaces)
	allNamespaces, err := s.ListNamespaces(ctx, true)
	if err != nil {
		return nil, err
	}

	// Filter namespaces that match project destination patterns
	matchedNamespaces := make([]string, 0)
	seen := make(map[string]bool)

	for _, ns := range allNamespaces {
		for _, dest := range project.Spec.Destinations {
			// Skip destinations without namespace pattern
			if dest.Namespace == "" {
				continue
			}

			// Check if namespace matches the pattern
			if NamespaceMatchesPattern(ns, dest.Namespace) {
				if !seen[ns] {
					matchedNamespaces = append(matchedNamespaces, ns)
					seen[ns] = true
				}
				break // No need to check other destinations for this namespace
			}
		}
	}

	// Sort for consistent output
	sort.Strings(matchedNamespaces)

	return matchedNamespaces, nil
}

// ListNamespacesForUser returns namespaces that are accessible to a specific user,
// filtered by the projects they have access to. Each project's destination namespace
// patterns are used to filter the cluster namespaces.
func (s *NamespaceService) ListNamespacesForUser(ctx context.Context, authorizer Authorizer, userID string, groups []string, excludeSystem bool) ([]string, error) {
	// Get projects the user can access
	accessibleProjects, err := authorizer.GetAccessibleProjects(ctx, userID, groups)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible projects: %w", err)
	}

	if len(accessibleProjects) == 0 {
		return []string{}, nil
	}

	// Collect namespace patterns from all accessible projects
	var patterns []string
	for _, projectName := range accessibleProjects {
		project, err := s.projectService.GetProject(ctx, projectName)
		if err != nil {
			slog.Warn("skipping project in namespace listing — may have been deleted",
				"project", projectName,
				"error", err,
			)
			continue
		}
		for _, dest := range project.Spec.Destinations {
			if dest.Namespace != "" {
				patterns = append(patterns, dest.Namespace)
			}
		}
	}

	if len(patterns) == 0 {
		return []string{}, nil
	}

	// Get all cluster namespaces
	allNamespaces, err := s.ListNamespaces(ctx, excludeSystem)
	if err != nil {
		return nil, err
	}

	// Filter namespaces by collected patterns
	return FilterNamespacesByPatterns(allNamespaces, patterns), nil
}

// NamespaceMatchesPattern checks if a namespace matches a glob pattern
// Supports:
// - "*" matches any namespace
// - "dev-*" matches dev-staging, dev-production, etc.
// - "*-prod" matches team-a-prod, team-b-prod, etc.
// - "team-?-prod" matches team-a-prod, team-b-prod, but not team-aa-prod
// - Exact matches like "production"
func NamespaceMatchesPattern(namespace, pattern string) bool {
	// Full wildcard matches everything
	if pattern == "*" {
		return true
	}

	// Use path.Match for glob pattern matching
	// This handles *, ?, and [] character classes
	matched, err := path.Match(pattern, namespace)
	if err != nil {
		// If pattern is invalid, fall back to exact match
		return pattern == namespace
	}

	return matched
}

// FilterNamespacesByPatterns filters a list of namespaces against multiple patterns
// Returns namespaces that match at least one pattern
func FilterNamespacesByPatterns(namespaces []string, patterns []string) []string {
	if len(patterns) == 0 {
		return []string{}
	}

	matched := make([]string, 0)
	seen := make(map[string]bool)

	for _, ns := range namespaces {
		for _, pattern := range patterns {
			if NamespaceMatchesPattern(ns, pattern) {
				if !seen[ns] {
					matched = append(matched, ns)
					seen[ns] = true
				}
				break
			}
		}
	}

	sort.Strings(matched)
	return matched
}

// NamespaceExists checks if a namespace exists on the cluster
func (s *NamespaceService) NamespaceExists(ctx context.Context, name string) (bool, error) {
	_, err := s.k8sClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}
	return true, nil
}
