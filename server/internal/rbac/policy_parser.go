package rbac

import (
	"errors"
	"fmt"
	"strings"
)

// Policy parsing errors
var (
	ErrInvalidPolicyFormat  = errors.New("invalid policy format")
	ErrInvalidFieldCount    = errors.New("invalid field count")
	ErrInvalidEffect        = errors.New("invalid effect")
	ErrEmptyField           = errors.New("empty policy field")
	ErrInvalidSubjectType   = errors.New("invalid subject type")
	ErrInvalidSubjectFormat = errors.New("invalid subject format")
	ErrInvalidObjectFormat  = errors.New("invalid object format")
	ErrInputTooLarge        = errors.New("input exceeds maximum size")
)

// Security constants for policy parsing input validation
const (
	// MaxPolicyStringLength limits individual policy string size to prevent DoS
	MaxPolicyStringLength = 1024
	// MaxPoliciesPerRole limits number of policies per role to prevent DoS
	MaxPoliciesPerRole = 100
	// MaxObjectPathDepth limits path segments to prevent excessive complexity
	MaxObjectPathDepth = 10
	// MaxObjectWildcards limits wildcards in object patterns
	MaxObjectWildcards = 5
)

// ValidSubjectTypes defines allowed subject type prefixes
var ValidSubjectTypes = map[string]bool{
	"role":  true, // Global roles: role:serveradmin
	"proj":  true, // Project roles: proj:engineering:developer
	"user":  true, // User subjects: user:john
	"group": true, // Group subjects: group:admins
}

// PolicyFields represents parsed policy components
type PolicyFields struct {
	Subject string // e.g., "role:serveradmin", "proj:engineering:developer"
	Object  string // e.g., "projects/*", "instances/engineering/*"
	Action  string // e.g., "get", "create", "*"
	Effect  string // "allow" or "deny"
}

// ParsePolicyString parses ArgoCD-style policy strings
// Format: "p, subject, object, action, effect"
// Example: "p, proj:engineering:developer, projects/engineering, *, allow"
func ParsePolicyString(policyStr string) ([]string, error) {
	// SECURITY: Validate input size to prevent DoS
	if len(policyStr) > MaxPolicyStringLength {
		return nil, fmt.Errorf("%w: policy string exceeds %d bytes", ErrInputTooLarge, MaxPolicyStringLength)
	}

	// Remove 'p,' prefix if present
	policyStr = strings.TrimPrefix(strings.TrimSpace(policyStr), "p,")
	policyStr = strings.TrimSpace(policyStr)

	// Split by comma
	parts := strings.Split(policyStr, ",")

	// Must have exactly 4 fields: subject, object, action, effect
	if len(parts) != 4 {
		return nil, fmt.Errorf("%w: expected 4 fields (subject, object, action, effect), got %d",
			ErrInvalidFieldCount, len(parts))
	}

	// Trim whitespace from each field
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	// Extract fields
	subject := parts[0]
	object := parts[1]
	action := parts[2]
	effect := parts[3]

	// Validate fields
	if err := validatePolicyFields(subject, object, action, effect); err != nil {
		return nil, err
	}

	return []string{subject, object, action, effect}, nil
}

// validatePolicyFields validates individual policy fields
func validatePolicyFields(subject, object, action, effect string) error {
	// Validate subject
	if subject == "" {
		return fmt.Errorf("%w: subject cannot be empty", ErrEmptyField)
	}
	// SECURITY: Validate subject format and type
	if err := validateSubjectFormat(subject); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	// Validate object
	if object == "" {
		return fmt.Errorf("%w: object cannot be empty", ErrEmptyField)
	}
	if err := validateObjectFormat(object); err != nil {
		return fmt.Errorf("invalid object format: %w", err)
	}

	// Validate action
	if action == "" {
		return fmt.Errorf("%w: action cannot be empty", ErrEmptyField)
	}
	if err := validateAction(action); err != nil {
		return fmt.Errorf("invalid action: %w", err)
	}

	// Validate effect
	if effect != "allow" && effect != "deny" {
		return fmt.Errorf("%w: effect must be 'allow' or 'deny', got '%s'",
			ErrInvalidEffect, effect)
	}

	return nil
}

// validateObjectFormat validates object string format
// Valid formats:
//   - "projects/*" (wildcard)
//   - "projects/engineering" (specific)
//   - "instances/engineering/*" (nested wildcard)
//   - "*" (match all)
func validateObjectFormat(object string) error {
	if object == "*" {
		return nil // Wildcard match all
	}

	// SECURITY: Check for path traversal attempts
	if strings.Contains(object, "..") {
		return fmt.Errorf("%w: path traversal patterns (..) not allowed", ErrInvalidObjectFormat)
	}

	// SECURITY: Check for URL-encoded traversal
	if strings.Contains(strings.ToLower(object), "%2e") {
		return fmt.Errorf("%w: URL-encoded path traversal not allowed", ErrInvalidObjectFormat)
	}

	// SECURITY: Limit number of wildcards to prevent ReDoS
	wildcardCount := strings.Count(object, "*")
	if wildcardCount > MaxObjectWildcards {
		return fmt.Errorf("%w: too many wildcards (max %d, got %d)",
			ErrInvalidObjectFormat, MaxObjectWildcards, wildcardCount)
	}

	// SECURITY: Limit path depth to prevent excessive complexity
	pathSegments := strings.Split(object, "/")
	if len(pathSegments) > MaxObjectPathDepth {
		return fmt.Errorf("%w: path too deep (max %d segments, got %d)",
			ErrInvalidObjectFormat, MaxObjectPathDepth, len(pathSegments))
	}

	// Must contain at least one '/' or be a wildcard
	if !strings.Contains(object, "/") && !strings.Contains(object, "*") {
		return fmt.Errorf("object must be in format 'resource/name' or contain wildcard")
	}

	return nil
}

// validateAction validates action string
// Valid actions: get, create, update, delete, list, sync, or *
func validateAction(action string) error {
	validActions := map[string]bool{
		"*":      true,
		"get":    true,
		"create": true,
		"update": true,
		"delete": true,
		"list":   true,
		"sync":   true, // ArgoCD-specific
	}

	if !validActions[action] {
		return fmt.Errorf("action must be one of: get, create, update, delete, list, sync, or *")
	}

	return nil
}

// ValidatePolicyFormat performs comprehensive policy validation
func ValidatePolicyFormat(policy []string) error {
	if len(policy) != 4 {
		return fmt.Errorf("%w: policy must have 4 elements", ErrInvalidFieldCount)
	}

	return validatePolicyFields(policy[0], policy[1], policy[2], policy[3])
}

// ParseProjectRole parses a complete Project role into Casbin policies
func ParseProjectRole(projectName string, role ProjectRole) ([]PolicyFields, error) {
	// SECURITY: Validate project name
	if err := ValidateSubjectIdentifier(projectName); err != nil {
		return nil, fmt.Errorf("invalid project name: %w", err)
	}

	// SECURITY: Validate role name
	if err := ValidateSubjectIdentifier(role.Name); err != nil {
		return nil, fmt.Errorf("invalid role name: %w", err)
	}

	// SECURITY: Limit number of policies per role to prevent DoS
	if len(role.Policies) > MaxPoliciesPerRole {
		return nil, fmt.Errorf("%w: role has %d policies (max %d)",
			ErrInputTooLarge, len(role.Policies), MaxPoliciesPerRole)
	}

	policies := make([]PolicyFields, 0, len(role.Policies))

	for _, policyStr := range role.Policies {
		// Parse policy string
		parts, err := ParsePolicyString(policyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse policy '%s': %w", policyStr, err)
		}

		// Normalize subject to proj:project:role format
		subject := fmt.Sprintf("proj:%s:%s", projectName, role.Name)

		policies = append(policies, PolicyFields{
			Subject: subject,
			Object:  parts[1],
			Action:  parts[2],
			Effect:  parts[3],
		})
	}

	return policies, nil
}

// NormalizePolicySubject normalizes a policy subject
// Converts: "proj:myproject:developer" or "role:serveradmin"
// Ensures consistent format for Casbin
// Returns the original value if already normalized (for backwards compatibility)
func NormalizePolicySubject(subject string) string {
	subject = strings.TrimSpace(subject)

	// Already normalized with valid prefix
	if strings.HasPrefix(subject, "proj:") || strings.HasPrefix(subject, "role:") {
		return subject
	}

	// Check for other valid prefixes
	if strings.HasPrefix(subject, "user:") || strings.HasPrefix(subject, "group:") {
		return subject
	}

	// Default to role: prefix if not specified
	return "role:" + subject
}

// NormalizePolicySubjectSafe normalizes a policy subject with validation
// Returns an error if the subject format is invalid
// Use this for untrusted input; use NormalizePolicySubject for trusted input
func NormalizePolicySubjectSafe(subject string) (string, error) {
	subject = strings.TrimSpace(subject)

	if subject == "" {
		return "", fmt.Errorf("%w: subject cannot be empty", ErrEmptyField)
	}

	// SECURITY: Validate input length
	if len(subject) > MaxSubjectIDLength {
		return "", fmt.Errorf("%w: subject exceeds maximum length of %d", ErrInputTooLarge, MaxSubjectIDLength)
	}

	// Check if already has a valid prefix
	for prefix := range ValidSubjectTypes {
		if strings.HasPrefix(subject, prefix+":") {
			// Validate the rest of the subject
			if err := validateSubjectFormat(subject); err != nil {
				return "", err
			}
			return subject, nil
		}
	}

	// SECURITY: Validate the identifier before prefixing
	if err := ValidateSubjectIdentifier(subject); err != nil {
		return "", fmt.Errorf("invalid subject identifier: %w", err)
	}

	// Default to role: prefix
	return "role:" + subject, nil
}

// SplitPolicySubject splits a policy subject into components (legacy, non-validating)
// "proj:engineering:developer" -> ("proj", "engineering", "developer")
// "role:serveradmin" -> ("role", "", "serveradmin")
// Deprecated: Use SplitPolicySubjectSafe for untrusted input
func SplitPolicySubject(subject string) (string, string, string) {
	parts := strings.SplitN(subject, ":", 3)

	switch len(parts) {
	case 1:
		return "role", "", parts[0]
	case 2:
		return parts[0], "", parts[1]
	case 3:
		return parts[0], parts[1], parts[2]
	default:
		return "", "", ""
	}
}

// SplitPolicySubjectSafe splits a policy subject into components with validation
// Returns an error if the subject format is invalid or contains an unknown type
// Use this for untrusted input; use SplitPolicySubject for trusted input
func SplitPolicySubjectSafe(subject string) (subjectType, project, role string, err error) {
	if subject == "" {
		return "", "", "", fmt.Errorf("%w: subject cannot be empty", ErrEmptyField)
	}

	// SECURITY: Validate input length
	if len(subject) > MaxSubjectIDLength {
		return "", "", "", fmt.Errorf("%w: subject exceeds maximum length of %d", ErrInputTooLarge, MaxSubjectIDLength)
	}

	parts := strings.SplitN(subject, ":", 3)

	switch len(parts) {
	case 1:
		// No prefix, treat as role
		if err := ValidateSubjectIdentifier(parts[0]); err != nil {
			return "", "", "", fmt.Errorf("invalid role name: %w", err)
		}
		return "role", "", parts[0], nil
	case 2:
		// type:name format (role:serveradmin, user:john, group:admins)
		subjectType = parts[0]
		if !ValidSubjectTypes[subjectType] {
			return "", "", "", fmt.Errorf("%w: '%s' is not a valid subject type (valid: role, proj, user, group)",
				ErrInvalidSubjectType, subjectType)
		}
		if err := ValidateSubjectIdentifier(parts[1]); err != nil {
			return "", "", "", fmt.Errorf("invalid subject identifier: %w", err)
		}
		return subjectType, "", parts[1], nil
	case 3:
		// proj:project:role format
		subjectType = parts[0]
		if subjectType != "proj" {
			return "", "", "", fmt.Errorf("%w: three-part subjects must use 'proj:' prefix, got '%s'",
				ErrInvalidSubjectFormat, subjectType)
		}
		if err := ValidateSubjectIdentifier(parts[1]); err != nil {
			return "", "", "", fmt.Errorf("invalid project name: %w", err)
		}
		if err := ValidateSubjectIdentifier(parts[2]); err != nil {
			return "", "", "", fmt.Errorf("invalid role name: %w", err)
		}
		return "proj", parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("%w: invalid subject format", ErrInvalidSubjectFormat)
	}
}

// validateSubjectFormat validates the format of a complete subject string
// This is used internally to validate subjects during policy parsing
func validateSubjectFormat(subject string) error {
	if subject == "" {
		return fmt.Errorf("%w: subject cannot be empty", ErrEmptyField)
	}

	// SECURITY: Validate input length
	if len(subject) > MaxSubjectIDLength {
		return fmt.Errorf("%w: subject exceeds maximum length of %d", ErrInputTooLarge, MaxSubjectIDLength)
	}

	parts := strings.SplitN(subject, ":", 3)

	// Extract the type prefix
	var subjectType string
	switch len(parts) {
	case 1:
		// No prefix - this will be treated as a role
		return ValidateSubjectIdentifier(parts[0])
	case 2:
		subjectType = parts[0]
	case 3:
		subjectType = parts[0]
	default:
		return fmt.Errorf("%w: invalid subject format", ErrInvalidSubjectFormat)
	}

	// SECURITY: Validate subject type
	if !ValidSubjectTypes[subjectType] {
		return fmt.Errorf("%w: '%s' is not a valid subject type (valid: role, proj, user, group)",
			ErrInvalidSubjectType, subjectType)
	}

	// Validate components based on type
	switch len(parts) {
	case 2:
		// type:name format (role:serveradmin, user:john, group:admins)
		return ValidateSubjectIdentifier(parts[1])
	case 3:
		// proj:project:role format
		if subjectType != "proj" {
			return fmt.Errorf("%w: three-part subjects must use 'proj:' prefix", ErrInvalidSubjectFormat)
		}
		if err := ValidateSubjectIdentifier(parts[1]); err != nil {
			return fmt.Errorf("invalid project name: %w", err)
		}
		return ValidateSubjectIdentifier(parts[2])
	}

	return nil
}
