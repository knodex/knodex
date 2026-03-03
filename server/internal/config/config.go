package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server         Server
	Kubernetes     Kubernetes
	Redis          Redis
	Log            Log
	Auth           Auth
	RateLimit      RateLimit
	PolicyCache    PolicyCache
	CasbinRoles    CasbinRoles
	Views          Views
	License        License
	Compliance     Compliance
	Organization   string // Organization identity for multi-tenant deployments. Default: "default".
	SwaggerEnabled bool   // Enable Swagger UI at /swagger/. Default: false. Env: SWAGGER_UI_ENABLED.
}

// Compliance holds enterprise compliance feature configuration
type Compliance struct {
	// HistoryRetentionDays is how long violation history records are retained in Redis.
	// Default: 90 days.
	HistoryRetentionDays int
}

// License holds enterprise license configuration
type License struct {
	// Path is the file path to the license JWT file.
	// Default: /etc/knodex/license.jwt
	Path string

	// Text is the raw license JWT token string.
	// Takes precedence over Path when both are set.
	// Set via KNODEX_LICENSE_TEXT environment variable.
	Text string
}

// Views holds enterprise views configuration
type Views struct {
	// ConfigPath is the path to the views YAML configuration file.
	// If empty, uses VIEWS_CONFIG_PATH environment variable or default.
	// Default: /etc/knodex/views.yaml
	ConfigPath string
}

// CasbinRoles holds Casbin user-role persistence configuration
type CasbinRoles struct {
	// TTL is the time-to-live for persisted user-role assignments in Redis.
	// Roles expire after this duration and are refreshed on next OIDC login.
	// Default: 24h. Format: Go duration string (e.g., "24h", "12h30m").
	TTL time.Duration

	// AdminUsers is a list of user IDs that should be assigned the role:serveradmin
	// role at server startup. This ensures these users have admin access even
	// when bypassing the login flow (e.g., E2E tests injecting JWTs directly).
	// Format: comma-separated user IDs, e.g., "user-global-admin,user-backup-admin"
	// Default: empty (no pre-assigned admin users)
	AdminUsers []string
}

// PolicyCache holds RBAC policy caching configuration
type PolicyCache struct {
	// Enabled determines if authorization caching is enabled
	// Default: true
	Enabled bool

	// TTLSeconds is the cache entry time-to-live in seconds
	// Default: 300 (5 minutes)
	TTLSeconds int

	// SyncIntervalMinutes is the interval between background policy syncs
	// Default: 10 minutes
	SyncIntervalMinutes int

	// WatchEnabled determines if Project CRD watching is enabled
	// Default: true
	WatchEnabled bool
}

// Log holds structured logging configuration
type Log struct {
	Level     string // debug, info, warn, error
	Format    string // json, text
	PodName   string // Kubernetes pod name (from HOSTNAME or POD_NAME)
	Namespace string // Kubernetes namespace (from POD_NAMESPACE)
}

// Server holds HTTP server configuration
type Server struct {
	Address            string
	Port               int
	CORSAllowedOrigins []string // Allowed origins for CORS. Empty = deny all cross-origin requests.
}

// Kubernetes holds Kubernetes client configuration
type Kubernetes struct {
	InCluster  bool
	Kubeconfig string
}

// Redis holds Redis connection configuration
type Redis struct {
	Address               string
	Password              string
	DB                    int
	Username              string // Redis 6+ ACL username
	TLSEnabled            bool   // Enable TLS for Redis connections
	TLSInsecureSkipVerify bool   // Skip TLS certificate verification (dev only)
}

// RateLimit holds rate limiting configuration
type RateLimit struct {
	// UserRequestsPerMinute is the number of requests allowed per minute per authenticated user
	// Default: 100. Set higher for E2E tests (e.g., 10000)
	UserRequestsPerMinute int
	// UserBurstSize is the burst size for per-user rate limiting
	// Default: 100. Set higher for E2E tests (e.g., 10000)
	UserBurstSize int
	// TrustedProxies is a list of trusted proxy IP addresses or CIDR ranges.
	// When set, rate limiters use X-Forwarded-For to identify the real client IP.
	// Format: comma-separated, e.g., "10.0.0.0/8,172.16.0.0/12"
	TrustedProxies []string
}

// Auth holds authentication configuration
type Auth struct {
	// LocalAdmin configuration
	AdminUsername          string
	AdminPassword          string
	AdminPasswordGenerated bool // True if password was auto-generated

	// OIDC configuration
	OIDCEnabled bool

	// OIDCGroupMappings defines how OIDC groups map to projects/roles
	// Loaded from OIDC_GROUP_MAPPINGS environment variable (JSON format)
	// or from GroupMappingsFile (YAML format)
	OIDCGroupMappings []OIDCGroupMapping

	// GroupsClaim is the OIDC token claim name that contains user groups
	// Default: "groups". Common alternatives: "roles", "memberOf"
	GroupsClaim string

	// GroupMappingsFile is the path to a YAML file containing group mappings
	// If specified, mappings are loaded from this file instead of environment variable
	GroupMappingsFile string

	// DefaultRole is the Casbin role assigned to OIDC users whose groups
	// don't match any configured mapping. Follows ArgoCD's policy.default pattern.
	// Valid values: "role:serveradmin" or "" (disabled/deny-by-default).
	// Default: "" (deny-by-default — unmapped users get zero permissions)
	DefaultRole string

	// AllowedRedirectOrigins is a list of allowed origins for OIDC callback redirects.
	// Only relative paths or URLs with origins in this list are accepted.
	// Prevents open redirect attacks (CWE-601).
	// Format: comma-separated list of origins, e.g., "http://localhost:3000,https://knodex.example.com"
	// Default: empty (only relative paths allowed)
	AllowedRedirectOrigins []string
}

// OIDCGroupMapping represents a mapping from an OIDC group to a project/role
// or global admin status. Configured via Helm values, environment variables,
// or YAML configuration file. Supports wildcard patterns in group names
// (e.g., "dev-*", "*-admin") which are evaluated at runtime.
type OIDCGroupMapping struct {
	// Group is the OIDC group name to match.
	// Supports wildcards using filepath.Match patterns: *, ?, [abc], [a-z]
	// Examples: "dev-team", "dev-*", "*-admin", "team-[abc]"
	Group string `json:"group" yaml:"group"`

	// Project is the target project ID (mutually exclusive with GlobalAdmin)
	Project string `json:"project,omitempty" yaml:"project,omitempty"`

	// Role is the role to assign within the project
	// Must be: platform-admin, developer, or viewer
	Role string `json:"role,omitempty" yaml:"role,omitempty"`

	// GlobalAdmin grants Global Admin privileges (mutually exclusive with Project)
	GlobalAdmin bool `json:"globalAdmin,omitempty" yaml:"globalAdmin,omitempty"`
}

// Load reads configuration from environment variables with defaults
func Load() (*Config, error) {
	cfg := &Config{
		Server: Server{
			Address:            getEnv("SERVER_ADDRESS", ":8080"),
			Port:               getEnvInt("SERVER_PORT", 8080),
			CORSAllowedOrigins: getEnvStringSlice("CORS_ALLOWED_ORIGINS"),
		},
		Kubernetes: Kubernetes{
			InCluster:  getEnvBool("KUBERNETES_IN_CLUSTER", false),
			Kubeconfig: getEnv("KUBECONFIG", os.Getenv("HOME")+"/.kube/config"),
		},
		Redis: Redis{
			Address:               getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password:              getEnv("REDIS_PASSWORD", ""),
			DB:                    getEnvInt("REDIS_DB", 0),
			Username:              getEnv("REDIS_USERNAME", ""),
			TLSEnabled:            getEnvBool("REDIS_TLS_ENABLED", false),
			TLSInsecureSkipVerify: getEnvBool("REDIS_TLS_INSECURE_SKIP_VERIFY", false),
		},
		Log: Log{
			Level:     getEnv("LOG_LEVEL", "info"),
			Format:    getEnv("LOG_FORMAT", "json"),
			PodName:   getEnv("POD_NAME", getEnv("HOSTNAME", "")),
			Namespace: getEnv("POD_NAMESPACE", ""),
		},
		Auth: Auth{
			AdminUsername:          getEnv("ADMIN_USERNAME", "admin"),
			AdminPassword:          "", // Will be set by main.go from Kubernetes secret
			AdminPasswordGenerated: false,
			OIDCEnabled:            getEnvBool("OIDC_ENABLED", false),
			OIDCGroupMappings:      nil, // Will be loaded below
			GroupsClaim:            getEnv("OIDC_GROUPS_CLAIM", "groups"),
			GroupMappingsFile:      getEnv("OIDC_GROUP_MAPPINGS_FILE", ""),
			DefaultRole:            getEnv("RBAC_DEFAULT_ROLE", ""),
			AllowedRedirectOrigins: getEnvStringSlice("ALLOWED_REDIRECT_ORIGINS"),
		},
		RateLimit: RateLimit{
			UserRequestsPerMinute: getEnvInt("RATE_LIMIT_USER_REQUESTS_PER_MINUTE", 100),
			UserBurstSize:         getEnvInt("RATE_LIMIT_USER_BURST_SIZE", 100),
			TrustedProxies:        getEnvStringSlice("RATE_LIMIT_TRUSTED_PROXIES"),
		},
		PolicyCache: PolicyCache{
			Enabled:             getEnvBool("CACHE_ENABLED", true),
			TTLSeconds:          getEnvInt("CACHE_TTL_SECONDS", 300),
			SyncIntervalMinutes: getEnvInt("POLICY_SYNC_INTERVAL_MINUTES", 10),
			WatchEnabled:        getEnvBool("PROJECT_WATCH_ENABLED", true),
		},
		CasbinRoles: CasbinRoles{
			TTL:        getEnvDuration("CASBIN_ROLE_TTL", 24*time.Hour),
			AdminUsers: getEnvStringSlice("CASBIN_ADMIN_USERS"),
		},
		Views: Views{
			ConfigPath: getEnv("VIEWS_CONFIG_PATH", "/etc/knodex/views.yaml"),
		},
		License: License{
			Path: getEnv("KNODEX_LICENSE_PATH", "/etc/knodex/license.jwt"),
			Text: getEnv("KNODEX_LICENSE_TEXT", ""),
		},
		Compliance: Compliance{
			HistoryRetentionDays: getEnvInt("COMPLIANCE_HISTORY_RETENTION_DAYS", 90),
		},
		Organization:   getEnv("KNODEX_ORGANIZATION", "default"),
		SwaggerEnabled: getEnvBool("SWAGGER_UI_ENABLED", false),
	}

	// Normalize empty/whitespace organization to "default", trim surrounding whitespace
	cfg.Organization = strings.TrimSpace(cfg.Organization)
	if cfg.Organization == "" {
		cfg.Organization = "default"
	}

	// Validate organization value (length, no control characters)
	if err := ValidateOrganization(cfg.Organization); err != nil {
		return nil, fmt.Errorf("invalid organization configuration: %w", err)
	}

	// Load OIDC group mappings from file or environment variable
	groupMappings, err := loadOIDCGroupMappings(cfg.Auth.GroupMappingsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load OIDC group mappings: %w", err)
	}
	cfg.Auth.OIDCGroupMappings = groupMappings

	// Validate group mappings configuration
	if err := ValidateGroupMappings(cfg.Auth.OIDCGroupMappings); err != nil {
		return nil, fmt.Errorf("invalid OIDC group mappings configuration: %w", err)
	}

	// Validate default role configuration
	if err := ValidateDefaultRole(cfg.Auth.DefaultRole); err != nil {
		return nil, fmt.Errorf("invalid RBAC default role configuration: %w", err)
	}

	// Note: Admin password is now managed by bootstrap.GetOrCreateAdminPassword()
	// in main.go to ensure consistency across pod restarts

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(value, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			slog.Warn("invalid duration value for environment variable, using default",
				"key", key,
				"value", value,
				"default", defaultValue.String(),
				"error", err,
			)
			return defaultValue
		}
		return d
	}
	return defaultValue
}

// loadOIDCGroupMappings loads OIDC group-to-project mappings from file or environment variable.
// Priority:
//  1. If filePath is specified, load from YAML file
//  2. Otherwise, load from OIDC_GROUP_MAPPINGS environment variable (JSON format)
//
// Example JSON: [{"group":"engineering","project":"eng-project","role":"developer"}]
// Example YAML:
//
//	mappings:
//	  - group: "engineering"
//	    project: "eng-project"
//	    role: "developer"
func loadOIDCGroupMappings(filePath string) ([]OIDCGroupMapping, error) {
	// If file path is specified, load from YAML file
	if filePath != "" {
		return loadOIDCGroupMappingsFromFile(filePath)
	}

	// Fall back to environment variable (JSON format)
	value := os.Getenv("OIDC_GROUP_MAPPINGS")
	if value == "" {
		// Empty configuration is valid - no mappings defined
		return []OIDCGroupMapping{}, nil
	}

	var mappings []OIDCGroupMapping
	if err := json.Unmarshal([]byte(value), &mappings); err != nil {
		return nil, fmt.Errorf("failed to parse OIDC_GROUP_MAPPINGS JSON: %w", err)
	}

	return mappings, nil
}

// GroupMappings represents the YAML configuration file structure for group mappings.
// This allows for a more readable and maintainable configuration format compared to JSON.
type GroupMappings struct {
	Mappings []OIDCGroupMapping `yaml:"mappings"`
}

// loadOIDCGroupMappingsFromFile loads group mappings from a YAML file.
// Supports wildcards in group names (e.g., "dev-*", "*-admin") which will
// be evaluated at runtime by the GroupMapper.
func loadOIDCGroupMappingsFromFile(filePath string) ([]OIDCGroupMapping, error) {
	// Validate file path
	if filePath == "" {
		return []OIDCGroupMapping{}, nil
	}

	// Clean and validate path
	cleanPath := filepath.Clean(filePath)

	// Read file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File not found - return empty mappings with a warning
			// This allows the system to start without a mappings file
			return []OIDCGroupMapping{}, nil
		}
		return nil, fmt.Errorf("failed to read group mappings file %s: %w", cleanPath, err)
	}

	// Parse YAML
	var config GroupMappings
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse group mappings YAML from %s: %w", cleanPath, err)
	}

	return config.Mappings, nil
}

// ValidateDefaultRole validates the RBAC default role configuration.
// The default role must be a known built-in Casbin role or empty string (disabled).
// Valid values: "role:serveradmin" or "" (empty = disabled).
// The only built-in global role is role:serveradmin.
// Cannot import rbac here to avoid circular dependency.
func ValidateDefaultRole(role string) error {
	if role == "" {
		return nil // Empty = disabled (preserves zero-access behavior)
	}

	validRoles := map[string]bool{
		"role:serveradmin": true, // Must match rbac.CasbinRoleServerAdmin
	}
	if !validRoles[role] {
		return fmt.Errorf("RBAC_DEFAULT_ROLE must be 'role:serveradmin' or empty, got %q", role)
	}
	if role == "role:serveradmin" {
		slog.Warn("RBAC_DEFAULT_ROLE is set to 'role:serveradmin' — ALL unmapped OIDC users will receive full server admin access",
			"role", role,
			"recommendation", "set to empty and assign roles via OIDC group mappings instead",
		)
	}
	return nil
}

// MaxOrganizationLength is the maximum length for the Organization value.
// Matches Kubernetes label value limit (63 characters).
const MaxOrganizationLength = 63

// ValidateOrganization validates the organization identity value.
// Rejects values with control characters (log injection prevention) and
// values exceeding the Kubernetes label value limit (63 characters).
// The "default" value is always valid and is not checked here.
func ValidateOrganization(org string) error {
	if len(org) > MaxOrganizationLength {
		return fmt.Errorf("KNODEX_ORGANIZATION must be at most %d characters, got %d", MaxOrganizationLength, len(org))
	}
	for i, r := range org {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("KNODEX_ORGANIZATION contains control character at position %d (byte 0x%02x)", i, r)
		}
	}
	return nil
}

// ValidateGroupMappings validates the OIDC group mappings configuration
// Returns an error if any mapping is invalid, with the mapping index and field for debugging
func ValidateGroupMappings(mappings []OIDCGroupMapping) error {
	// Valid roles for project mappings
	validRoles := map[string]bool{
		"platform-admin": true,
		"developer":      true,
		"viewer":         true,
	}

	for i, m := range mappings {
		// Rule 1: Group name is always required
		if m.Group == "" {
			return fmt.Errorf("groupMappings[%d]: group name is required", i)
		}

		// Check if project/role is set
		hasProject := m.Project != ""
		hasRole := m.Role != ""
		hasGlobal := m.GlobalAdmin

		// Rule 2: Either (project + role) OR globalAdmin, not both
		if hasProject && hasGlobal {
			return fmt.Errorf("groupMappings[%d] (group=%q): cannot set both project and globalAdmin", i, m.Group)
		}
		if hasRole && hasGlobal {
			return fmt.Errorf("groupMappings[%d] (group=%q): cannot set both role and globalAdmin", i, m.Group)
		}

		// Rule 3: Must have either (project + role) OR globalAdmin
		if !hasGlobal && !hasProject && !hasRole {
			return fmt.Errorf("groupMappings[%d] (group=%q): must set either (project + role) or globalAdmin", i, m.Group)
		}

		// Rules for project mappings (non-globalAdmin)
		if !hasGlobal {
			// Rule 4: If project is set, role is required
			if hasProject && !hasRole {
				return fmt.Errorf("groupMappings[%d] (group=%q): role is required when project is set", i, m.Group)
			}

			// Rule 5: If role is set, project is required
			if hasRole && !hasProject {
				return fmt.Errorf("groupMappings[%d] (group=%q): project is required when role is set", i, m.Group)
			}

			// Rule 6: Role must be valid
			if hasRole && !validRoles[m.Role] {
				return fmt.Errorf("groupMappings[%d] (group=%q): invalid role %q, must be one of: platform-admin, developer, viewer", i, m.Group, m.Role)
			}
		}
	}

	return nil
}
