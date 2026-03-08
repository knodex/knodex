// Package collection provides generic collection utilities for slices and maps.
//
// These complement Go's stdlib slices/maps packages (Go 1.21+) with utilities
// not available in stdlib, such as set operations, filtering, and mapping.
// Prefer stdlib functions (e.g., slices.Contains) when they exist.
//
// This package consolidates ~288 ad-hoc loops scattered across the codebase,
// following the ArgoCD util/ package structure.
package collection

import "slices"

// Contains returns true if the slice contains the given item.
// Delegates to slices.Contains from Go 1.21+ stdlib.
func Contains[T comparable](slice []T, item T) bool {
	return slices.Contains(slice, item)
}

// ContainsFunc returns true if any element in the slice satisfies the predicate.
func ContainsFunc[T any](slice []T, fn func(T) bool) bool {
	for _, v := range slice {
		if fn(v) {
			return true
		}
	}
	return false
}

// Filter returns a new slice containing only elements that satisfy the predicate.
// Always returns a non-nil slice (empty slice, not nil) to ensure consistent
// JSON serialization ([] instead of null) across API responses.
func Filter[T any](slice []T, fn func(T) bool) []T {
	result := make([]T, 0)
	for _, v := range slice {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Map applies fn to each element and returns a new slice of results.
func Map[T, U any](slice []T, fn func(T) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Deduplicate returns a new slice with duplicate elements removed,
// preserving the order of first occurrence.
func Deduplicate[T comparable](slice []T) []T {
	seen := make(map[T]bool, len(slice))
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// Equal returns true if two slices contain the same elements regardless of order.
// Uses counting to handle duplicates correctly.
func Equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[T]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
		if counts[v] < 0 {
			return false
		}
	}
	return true
}

// Diff computes the set difference between old and new slices.
// Returns elements added (in new but not old) and removed (in old but not new).
func Diff[T comparable](old, new []T) (added, removed []T) {
	oldSet := make(map[T]bool, len(old))
	for _, v := range old {
		oldSet[v] = true
	}
	newSet := make(map[T]bool, len(new))
	for _, v := range new {
		newSet[v] = true
	}

	for _, v := range new {
		if !oldSet[v] {
			added = append(added, v)
		}
	}
	for _, v := range old {
		if !newSet[v] {
			removed = append(removed, v)
		}
	}
	return added, removed
}
