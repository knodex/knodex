// Package sanitize provides input sanitization utilities for security-critical
// operations including Redis keys, Kubernetes names, filenames, and path components.
//
// This package consolidates 13+ scattered sanitize functions that were duplicated
// across rbac, compliance, audit, handlers, and deployment packages, following
// the ArgoCD util/ package structure.
//
// All functions are designed to prevent injection attacks (command injection,
// path traversal, log injection) while preserving valid input.
package sanitize

import (
	"fmt"
	"regexp"
	"strings"
)

// Pre-compiled regexes for performance.
var (
	k8sNameRegex       = regexp.MustCompile(`[^a-z0-9-]`)
	k8sNameMultiDash   = regexp.MustCompile(`-+`)
	redisKeyRegex      = regexp.MustCompile(`[^a-zA-Z0-9._/-]`)
	filenameRegex      = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	pathComponentRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

	// globReplacer escapes glob wildcard characters.
	globReplacer = strings.NewReplacer(
		"*", `\*`,
		"?", `\?`,
		"[", `\[`,
		"]", `\]`,
		"{", `\{`,
		"}", `\}`,
	)
)

// RemoveControlChars removes non-printable control characters from the input,
// keeping only characters with code point >= 32 (space) and excluding DEL (127).
func RemoveControlChars(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	for _, r := range s {
		if r >= 32 && r != 127 {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// RedisKey sanitizes a string for use as a Redis key component.
// Replaces characters that are not alphanumeric, period, underscore, hyphen,
// or forward slash with underscores. Truncates to 512 characters.
func RedisKey(s string) string {
	sanitized := redisKeyRegex.ReplaceAllString(s, "_")
	if len(sanitized) > 512 {
		sanitized = sanitized[:512]
	}
	return sanitized
}

// CommitMessage sanitizes a git commit message by removing null bytes and
// non-printable control characters (except newline, tab, carriage return).
// Returns an error if the message is empty or contains only invalid characters.
func CommitMessage(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")

	var sanitized strings.Builder
	sanitized.Grow(len(s))
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' || r == '\r' {
			sanitized.WriteRune(r)
		}
	}

	result := sanitized.String()
	if result == "" {
		return "", fmt.Errorf("commit message contains only invalid characters")
	}
	return result, nil
}

// GlobCharacters escapes glob wildcard characters in the input string
// to prevent glob pattern injection in Casbin policies.
func GlobCharacters(s string) string {
	return globReplacer.Replace(s)
}

// K8sName sanitizes a string for use as a Kubernetes resource name.
// Converts to lowercase, replaces invalid characters with hyphens,
// collapses multiple hyphens, trims hyphens from edges, and
// truncates to 40 characters.
func K8sName(s string) string {
	name := strings.ToLower(s)
	name = k8sNameRegex.ReplaceAllString(name, "-")
	name = k8sNameMultiDash.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

// PathParam sanitizes an HTTP path parameter by removing control characters
// and trimming whitespace. This is an alias for RemoveControlChars.
func PathParam(s string) string {
	return RemoveControlChars(s)
}

// Filename sanitizes a string for use as a filename in HTTP headers.
// Replaces invalid characters with underscores, prevents path traversal
// via "..", and truncates to 200 characters. Returns "export" for empty input.
func Filename(s string) string {
	sanitized := filenameRegex.ReplaceAllString(s, "_")
	sanitized = strings.ReplaceAll(sanitized, "..", "_")
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}
	if sanitized == "" {
		sanitized = "export"
	}
	return sanitized
}

// PathComponent sanitizes a path component to prevent path traversal (CWE-22).
// Removes ".." sequences and replaces path separators with hyphens.
// Returns an error if the component is empty or invalid after sanitization.
func PathComponent(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("path component cannot be empty")
	}
	component := strings.ReplaceAll(s, "..", "")
	component = strings.ReplaceAll(component, "/", "-")
	component = strings.ReplaceAll(component, "\\", "-")
	component = strings.TrimSpace(component)

	// Validate the result contains valid characters
	cleaned := pathComponentRegex.ReplaceAllString(component, "-")
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return "", fmt.Errorf("path component %q contains only invalid characters", s)
	}
	return cleaned, nil
}
