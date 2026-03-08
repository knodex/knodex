package rbac

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	utilhash "github.com/knodex/knodex/server/internal/util/hash"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// Validation constraints
const (
	MaxDisplayNameLength = 255
	MaxUserIDLength      = 253
	MaxMemberCount       = 1000
	// RFC 5321 email length limits
	MaxEmailLength     = 320
	MaxEmailLocalPart  = 64
	MaxEmailDomainPart = 255
)

var (
	// RFC 5322 compliant email regex (simplified but secure)
	// Allows: alphanumeric, dots, plus, hyphens, underscores in local part
	// Requires: valid domain with at least one dot
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
)

// ValidateEmail performs comprehensive email validation per RFC 5321/5322
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// RFC 5321: Maximum total length
	if len(email) > MaxEmailLength {
		return fmt.Errorf("email exceeds maximum length of %d characters", MaxEmailLength)
	}

	// Check for valid UTF-8 and ASCII-only (prevent homograph attacks)
	if !utf8.ValidString(email) {
		return fmt.Errorf("email contains invalid UTF-8")
	}
	for _, r := range email {
		if r > 127 {
			return fmt.Errorf("email must contain only ASCII characters")
		}
	}

	// Use stdlib parser for basic validation
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}

	// Re-validate with strict regex
	if !emailRegex.MatchString(addr.Address) {
		return fmt.Errorf("email contains invalid characters")
	}

	// Validate local part and domain length
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid email structure")
	}

	if len(parts[0]) > MaxEmailLocalPart {
		return fmt.Errorf("email local part exceeds %d characters", MaxEmailLocalPart)
	}
	if len(parts[1]) > MaxEmailDomainPart {
		return fmt.Errorf("email domain exceeds %d characters", MaxEmailDomainPart)
	}

	// Ensure local part and domain are not empty
	if parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("email local part and domain cannot be empty")
	}

	return nil
}

// ValidateDisplayName validates project display name
func ValidateDisplayName(displayName string) error {
	if displayName == "" {
		return fmt.Errorf("displayName cannot be empty")
	}
	if len(displayName) > MaxDisplayNameLength {
		return fmt.Errorf("displayName exceeds maximum length of %d characters", MaxDisplayNameLength)
	}
	// Check for control characters
	for _, r := range displayName {
		if r < 32 || r == 127 {
			return fmt.Errorf("displayName contains invalid control characters")
		}
	}
	return nil
}

// ValidateUserID validates user ID format
func ValidateUserID(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	if len(userID) > MaxUserIDLength {
		return fmt.Errorf("userID exceeds maximum length of %d characters", MaxUserIDLength)
	}
	// User IDs should be valid Kubernetes resource names
	// Allow alphanumeric, hyphens, and periods
	for i, char := range userID {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.') {
			return fmt.Errorf("userID contains invalid character at position %d", i)
		}
		// Cannot start or end with hyphen or period
		if (char == '-' || char == '.') && (i == 0 || i == len(userID)-1) {
			return fmt.Errorf("userID cannot start or end with hyphen or period")
		}
	}
	return nil
}

// ValidateNamespace validates namespace format (already exists in project_service.go as isValidNamespace)
// This is a public version with error messages
func ValidateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if len(namespace) > 63 {
		return fmt.Errorf("namespace exceeds maximum length of 63 characters")
	}

	for i, char := range namespace {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return fmt.Errorf("namespace contains invalid character '%c' at position %d (only lowercase, alphanumeric, and hyphens allowed)", char, i)
		}
		// Cannot start or end with hyphen
		if char == '-' && (i == 0 || i == len(namespace)-1) {
			return fmt.Errorf("namespace cannot start or end with hyphen")
		}
	}

	return nil
}

// --- ArgoCD-aligned Project Validation Functions ---

// IsWildcard checks if a string contains wildcard patterns
// Used for destinations and resource specifications
func IsWildcard(str string) bool {
	if str == "" {
		return false
	}
	return str == "*" || str[0] == '*' || str[len(str)-1] == '*'
}

// MatchNamespace checks if a namespace matches an allowed pattern.
// Patterns can be:
// - Exact match: "staging" matches only "staging"
// - Suffix wildcard: "staging*" matches "staging", "staging-team-a", "staging-env-1"
// - Prefix wildcard: "*-prod" matches "app-prod", "service-prod"
// - Universal wildcard: "*" matches everything
// This function is used for instance filtering based on project destination namespaces.
func MatchNamespace(namespace, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	// Suffix wildcard (e.g., "staging*")
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(namespace, prefix)
	}
	// Prefix wildcard (e.g., "*-prod")
	if pattern[0] == '*' {
		suffix := pattern[1:]
		return strings.HasSuffix(namespace, suffix)
	}
	// Exact match
	return namespace == pattern
}

// MatchNamespaceInList checks if a namespace matches any pattern in the allowed list.
// Returns true if the namespace matches at least one pattern.
// Used for instance filtering based on user's project namespace access.
func MatchNamespaceInList(namespace string, allowedPatterns []string) bool {
	for _, pattern := range allowedPatterns {
		if MatchNamespace(namespace, pattern) {
			return true
		}
	}
	return false
}

// IsDenyRule checks if a string is a deny pattern
// Deny patterns start with "!" and take precedence over allow rules
func IsDenyRule(pattern string) bool {
	return len(pattern) > 0 && pattern[0] == '!'
}

// ValidateProjectName validates a project name follows DNS-1123 subdomain rules
// Rules: lowercase alphanumeric, hyphens, max 253 chars, starts/ends with alphanumeric
func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("project name cannot exceed 253 characters")
	}
	// DNS-1123 subdomain validation pattern
	pattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !pattern.MatchString(name) {
		return fmt.Errorf("project name must be lowercase alphanumeric with hyphens, cannot start or end with hyphen")
	}
	return nil
}

// Validate ensures destination has a valid namespace
func (d *Destination) Validate() error {
	if d.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	// Validate namespace pattern if not a wildcard
	if !IsWildcard(d.Namespace) {
		// DNS-1123 label validation (simplified for wildcards)
		pattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
		if !pattern.MatchString(d.Namespace) {
			return fmt.Errorf("destination namespace must be a valid DNS-1123 label or wildcard pattern")
		}
	}
	return nil
}

// ValidateResourceSpec ensures group and kind are valid
// Both can be wildcards (*) for broad matching
func (r *ResourceSpec) Validate() error {
	// At least one of group or kind must be non-empty
	// Empty group means core API group, which is valid
	if r.Kind == "" {
		return fmt.Errorf("resource spec must specify kind")
	}
	return nil
}

// ValidateProjectRole validates a project role definition
func (r *ProjectRole) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("role name cannot be empty")
	}
	// Validate role name pattern
	pattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !pattern.MatchString(r.Name) {
		return fmt.Errorf("role name must be lowercase alphanumeric with hyphens")
	}
	if len(r.Policies) == 0 {
		return fmt.Errorf("role must have at least one policy")
	}
	return nil
}

// ValidateProjectSpec validates the entire ProjectSpec for ArgoCD-aligned structure
func ValidateProjectSpec(spec ProjectSpec) error {
	// Validate destinations
	if len(spec.Destinations) == 0 {
		return fmt.Errorf("project must specify at least one destination")
	}
	for i, dest := range spec.Destinations {
		if err := dest.Validate(); err != nil {
			return fmt.Errorf("destinations[%d]: %w", i, err)
		}
	}

	// Validate cluster resource whitelists
	for i, res := range spec.ClusterResourceWhitelist {
		if err := res.Validate(); err != nil {
			return fmt.Errorf("clusterResourceWhitelist[%d]: %w", i, err)
		}
	}

	// Validate cluster resource blacklists
	for i, res := range spec.ClusterResourceBlacklist {
		if err := res.Validate(); err != nil {
			return fmt.Errorf("clusterResourceBlacklist[%d]: %w", i, err)
		}
	}

	// Validate namespace resource whitelists
	for i, res := range spec.NamespaceResourceWhitelist {
		if err := res.Validate(); err != nil {
			return fmt.Errorf("namespaceResourceWhitelist[%d]: %w", i, err)
		}
	}

	// Validate namespace resource blacklists
	for i, res := range spec.NamespaceResourceBlacklist {
		if err := res.Validate(); err != nil {
			return fmt.Errorf("namespaceResourceBlacklist[%d]: %w", i, err)
		}
	}

	// Validate roles
	roleNames := make(map[string]bool)
	for i, role := range spec.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("roles[%d]: %w", i, err)
		}
		if roleNames[role.Name] {
			return fmt.Errorf("roles[%d]: duplicate role name '%s'", i, role.Name)
		}
		roleNames[role.Name] = true
	}

	return nil
}

// SanitizeInput removes potentially dangerous characters from string inputs.
// Reserved for future use in input sanitization for security best practices.
// Use this function when accepting user-provided strings that will be:
//   - Displayed in logs or UI (prevent log injection/XSS)
//   - Used in system commands (prevent command injection)
//   - Stored in databases (defense in depth against injection attacks)
func SanitizeInput(input string) string {
	return sanitize.RemoveControlChars(input)
}

// GenerateProjectID generates a deterministic project ID from display name
// The ID is lowercase, URL-safe, and unique for each display name
func GenerateProjectID(displayName string) string {
	// Normalize: lowercase and trim
	normalized := strings.ToLower(strings.TrimSpace(displayName))

	// Create a hash for uniqueness
	hashStr := utilhash.Truncate(utilhash.SHA256String(normalized), 8)

	// Convert to URL-safe slug
	slug := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1 // Remove other characters
	}, normalized)

	// Remove consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Truncate slug if too long (leaving room for hash)
	maxSlugLen := 50
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
		slug = strings.Trim(slug, "-")
	}

	// Combine slug and hash for uniqueness
	if slug == "" {
		return hashStr
	}
	return fmt.Sprintf("%s-%s", slug, hashStr)
}

// --- ArgoCD-compatible Glob Matching and Validation ---
// Reference: https://github.com/argoproj/argo-cd/blob/master/util/glob/glob.go

// MaxGlobSegments limits the maximum number of wildcard segments to prevent DoS
// via excessive pattern complexity. 100 segments is well above any realistic use case.
const MaxGlobSegments = 100

// MatchGlob performs simple glob pattern matching with support for * wildcard
// The * wildcard matches zero or more characters
// Case-sensitive matching (ArgoCD default behavior)
// Returns false if pattern exceeds MaxGlobSegments wildcards to prevent DoS
func MatchGlob(pattern, text string) bool {
	// Empty pattern only matches empty text
	if pattern == "" {
		return text == ""
	}

	// Simple "*" matches everything
	if pattern == "*" {
		return true
	}

	// No wildcards - exact match required
	if !strings.Contains(pattern, "*") {
		return pattern == text
	}

	// Handle patterns with wildcards
	// Split by * and match each segment
	segments := strings.Split(pattern, "*")

	// SECURITY: Limit maximum number of segments to prevent DoS
	// ArgoCD realistic use cases: 5-10 wildcards maximum
	if len(segments) > MaxGlobSegments {
		return false
	}

	// Current position in text
	pos := 0

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		// Find the segment in text starting from current position
		idx := strings.Index(text[pos:], segment)
		if idx == -1 {
			return false
		}

		// First segment must match at the beginning if pattern doesn't start with *
		if i == 0 && pattern[0] != '*' && idx != 0 {
			return false
		}

		// Move position past this segment
		pos += idx + len(segment)
	}

	// Last segment must match at the end if pattern doesn't end with *
	if pattern[len(pattern)-1] != '*' {
		lastSegment := segments[len(segments)-1]
		if lastSegment != "" && !strings.HasSuffix(text, lastSegment) {
			return false
		}
	}

	return true
}

// ValidateDestinationAgainstAllowed checks if a destination is allowed by the project's allowed destinations
// Matches on namespace with wildcard support. Optionally matches on Name if both sides specify it.
func ValidateDestinationAgainstAllowed(dest Destination, allowedDests []Destination) bool {
	if len(allowedDests) == 0 {
		return false
	}

	for _, allowed := range allowedDests {
		// Match namespace with wildcard support
		if !MatchGlob(allowed.Namespace, dest.Namespace) {
			continue
		}

		// If both sides specify a name, match on name too
		if allowed.Name != "" && dest.Name != "" {
			if !MatchGlob(allowed.Name, dest.Name) {
				continue
			}
		}

		return true
	}

	return false
}
