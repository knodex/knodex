package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// SecretName is the name of the Kubernetes secret containing the auto-generated admin password
	SecretName = "knodex-initial-admin-password"
	// SecretKey is the key within the secret that contains the password
	SecretKey = "password"
)

// EnsureAdminPasswordSecret creates the admin password secret if it doesn't exist.
// This function is idempotent - it will not overwrite an existing secret.
// Returns nil if the secret already exists or was created successfully.
func EnsureAdminPasswordSecret(ctx context.Context, k8sClient kubernetes.Interface, namespace, password string) error {
	if k8sClient == nil {
		return fmt.Errorf("kubernetes client is nil")
	}

	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Check if secret already exists
	_, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err == nil {
		// Secret exists - do not overwrite
		slog.Info("admin password secret already exists, skipping creation",
			"secret", SecretName,
			"namespace", namespace,
		)
		return nil
	}

	// If error is not "NotFound", something else went wrong
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check if secret exists: %w", err)
	}

	// Create secret with auto-generated password
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "knodex",
				"app.kubernetes.io/component":  "auth",
				"app.kubernetes.io/managed-by": "knodex-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			SecretKey: []byte(password),
		},
	}

	_, err = k8sClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create admin password secret: %w", err)
	}

	slog.Info("created admin password secret",
		"secret", SecretName,
		"namespace", namespace,
		"action_required", fmt.Sprintf("Retrieve password with: kubectl get secret %s -n %s -o jsonpath='{.data.password}' | base64 -d", SecretName, namespace),
	)

	return nil
}

// GetOrCreateAdminPassword retrieves the admin password from the Kubernetes secret if it exists,
// or generates and stores a new one if it doesn't exist.
// This ensures password consistency across pod restarts.
func GetOrCreateAdminPassword(ctx context.Context, k8sClient kubernetes.Interface, namespace string) (password string, wasGenerated bool, err error) {
	if k8sClient == nil {
		return "", false, fmt.Errorf("kubernetes client is nil")
	}

	if namespace == "" {
		return "", false, fmt.Errorf("namespace cannot be empty")
	}

	// Try to get existing secret
	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err == nil {
		// Secret exists - read password from it
		passwordBytes, ok := secret.Data[SecretKey]
		if !ok || len(passwordBytes) == 0 {
			return "", false, fmt.Errorf("secret exists but password key is missing or empty")
		}
		slog.Info("using existing admin password from secret",
			"secret", SecretName,
			"namespace", namespace,
		)
		return string(passwordBytes), false, nil
	}

	// If error is not "NotFound", something else went wrong
	if !errors.IsNotFound(err) {
		return "", false, fmt.Errorf("failed to check if secret exists: %w", err)
	}

	// Secret doesn't exist - generate new password
	password, err = generateSecurePassword()
	if err != nil {
		return "", false, fmt.Errorf("failed to generate password: %w", err)
	}

	// Create secret with generated password
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "knodex",
				"app.kubernetes.io/component":  "auth",
				"app.kubernetes.io/managed-by": "knodex-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			SecretKey: []byte(password),
		},
	}

	_, err = k8sClient.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		return "", false, fmt.Errorf("failed to create admin password secret: %w", err)
	}

	slog.Info("created new admin password secret",
		"secret", SecretName,
		"namespace", namespace,
		"retrieval_command", fmt.Sprintf("kubectl get secret %s -n %s -o jsonpath='{.data.password}' | base64 -d", SecretName, namespace),
	)

	return password, true, nil
}

// generateSecurePassword generates a cryptographically secure random password.
// The password is 24 characters long, URL-safe base64 encoded, and guaranteed
// to pass the 3-of-4 character class complexity requirement (upper, lower, digit, special).
//
// Base64 URL encoding uses A-Z, a-z, 0-9, -, _ which covers all 4 classes, but
// ~0.7% of random 24-char base64 strings lack digits or special characters.
// This function regenerates until complexity is met (typically first attempt).
func generateSecurePassword() (string, error) {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		// Generate 18 random bytes (18 bytes * 4/3 = 24 base64 characters)
		randomBytes := make([]byte, 18)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}

		// Encode to URL-safe base64 (no padding) for a clean password
		password := base64.RawURLEncoding.EncodeToString(randomBytes)

		if meetsComplexityRequirements(password) {
			return password, nil
		}
	}
	return "", fmt.Errorf("failed to generate password meeting complexity requirements after %d attempts", maxAttempts)
}

// meetsComplexityRequirements checks if a password has at least 3 of 4 character
// classes: uppercase, lowercase, digit, special (punctuation/symbol).
func meetsComplexityRequirements(password string) bool {
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case 'A' <= ch && ch <= 'Z':
			hasUpper = true
		case 'a' <= ch && ch <= 'z':
			hasLower = true
		case '0' <= ch && ch <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	classCount := 0
	if hasUpper {
		classCount++
	}
	if hasLower {
		classCount++
	}
	if hasDigit {
		classCount++
	}
	if hasSpecial {
		classCount++
	}
	return classCount >= 3
}
