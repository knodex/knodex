// Package env provides typed environment variable accessors with defaults.
//
// It consolidates the private getEnv* functions from config/config.go into
// reusable, tested utilities, following the ArgoCD util/env package pattern.
//
// Usage:
//
//	port := env.GetInt("PORT", 8080)
//	debug := env.GetBool("DEBUG", false)
//	addr := env.GetString("ADDR", "localhost")
package env

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// GetString returns the value of the environment variable named by key,
// or defaultValue if the variable is not set.
func GetString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetInt returns the environment variable as an integer, or defaultValue
// if not set or not a valid integer. Logs a warning for invalid values.
func GetInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err != nil {
			slog.Warn("invalid integer value for environment variable, using default",
				"key", key,
				"value", value,
				"default", defaultValue,
				"error", err,
			)
			return defaultValue
		}
		return i
	}
	return defaultValue
}

// GetBool returns the environment variable as a boolean, or defaultValue
// if not set or not a valid boolean. Logs a warning for invalid values.
func GetBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err != nil {
			slog.Warn("invalid boolean value for environment variable, using default",
				"key", key,
				"value", value,
				"default", defaultValue,
				"error", err,
			)
			return defaultValue
		}
		return b
	}
	return defaultValue
}

// GetStringSlice returns the environment variable split by commas.
// Empty entries are skipped. Returns nil if the variable is not set.
func GetStringSlice(key string) []string {
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

// GetDuration returns the environment variable parsed as a time.Duration,
// or defaultValue if not set or invalid. Logs a warning for invalid values.
func GetDuration(key string, defaultValue time.Duration) time.Duration {
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
