// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package parser provides type-safe utilities for parsing Kubernetes objects,
// including nested field accessors, metadata helpers, object builders, and
// YAML/JSON serialization. This package consolidates common parsing patterns
// used throughout the codebase to reduce duplication and standardize error handling.
package parser

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for common parsing failures.
var (
	// ErrFieldNotFound is returned when a field doesn't exist at the specified path.
	ErrFieldNotFound = errors.New("field not found")

	// ErrTypeMismatch is returned when a field exists but has the wrong type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrNilObject is returned when a nil object is passed to a function.
	ErrNilObject = errors.New("nil object")

	// ErrEmptyPath is returned when an empty path is provided.
	ErrEmptyPath = errors.New("empty path")
)

// PathError provides detailed context about a parsing error, including
// the path where the error occurred and type information.
type PathError struct {
	// Op is the operation that failed (e.g., "GetString", "GetMap").
	Op string

	// Path is the field path where the error occurred.
	Path []string

	// ExpectedType is the type that was expected (e.g., "string", "map[string]interface{}").
	ExpectedType string

	// ActualType is the actual type found (empty if field not found).
	ActualType string

	// Err is the underlying error.
	Err error
}

// Error returns a formatted error message with full context.
func (e *PathError) Error() string {
	pathStr := strings.Join(e.Path, ".")
	if pathStr == "" {
		pathStr = "(root)"
	}

	switch {
	case errors.Is(e.Err, ErrFieldNotFound):
		return fmt.Sprintf("%s: field not found at path %q", e.Op, pathStr)
	case errors.Is(e.Err, ErrTypeMismatch):
		return fmt.Sprintf("%s: type mismatch at path %q: expected %s, got %s",
			e.Op, pathStr, e.ExpectedType, e.ActualType)
	case errors.Is(e.Err, ErrNilObject):
		return fmt.Sprintf("%s: nil object provided", e.Op)
	case errors.Is(e.Err, ErrEmptyPath):
		return fmt.Sprintf("%s: empty path provided", e.Op)
	default:
		return fmt.Sprintf("%s: %v at path %q", e.Op, e.Err, pathStr)
	}
}

// Unwrap returns the underlying error for errors.Is and errors.As support.
func (e *PathError) Unwrap() error {
	return e.Err
}

// Is reports whether any error in err's chain matches target.
func (e *PathError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// newPathError creates a new PathError with the given parameters.
func newPathError(op string, path []string, expectedType, actualType string, err error) *PathError {
	return &PathError{
		Op:           op,
		Path:         path,
		ExpectedType: expectedType,
		ActualType:   actualType,
		Err:          err,
	}
}

// IsFieldNotFound returns true if the error indicates a field was not found.
func IsFieldNotFound(err error) bool {
	return errors.Is(err, ErrFieldNotFound)
}

// IsTypeMismatch returns true if the error indicates a type mismatch.
func IsTypeMismatch(err error) bool {
	return errors.Is(err, ErrTypeMismatch)
}

// IsNilObject returns true if the error indicates a nil object was provided.
func IsNilObject(err error) bool {
	return errors.Is(err, ErrNilObject)
}

// typeName returns a human-readable name for a type.
func typeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}
