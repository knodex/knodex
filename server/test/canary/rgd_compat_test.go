// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build canary

package canary_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	update  = flag.Bool("update", false, "update golden file baselines")
	baseURL = getEnvOrDefault("CANARY_BASE_URL", "http://localhost:8080")
)

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// referenceRGDs lists the reference RGD names and corresponding files.
// The name matches the metadata.name in each YAML file.
var referenceRGDs = []struct {
	Name    string
	Feature string
}{
	{Name: "ref-basic-types", Feature: "basic-types"},
	{Name: "ref-conditional-resources", Feature: "conditional-resources"},
	{Name: "ref-external-refs", Feature: "external-refs"},
	{Name: "ref-nested-external-refs", Feature: "nested-external-refs"},
	{Name: "ref-advanced-section", Feature: "advanced-section"},
	{Name: "ref-cel-expressions", Feature: "cel-expressions"},
}

func TestCanarySchemaCompat(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	token := generateCanaryJWT()

	// Determine golden file directory
	goldenDir := findGoldenDir(t)

	for _, rgd := range referenceRGDs {
		t.Run(rgd.Feature, func(t *testing.T) {
			// Fetch schema from Knodex API
			resp, err := makeAuthRequest(client, baseURL, "/api/v1/rgds/"+rgd.Name+"/schema", token)
			if err != nil {
				t.Fatalf("failed to fetch schema for %s: %v", rgd.Name, err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d: %s", rgd.Name, resp.StatusCode, string(body))
			}

			// Normalize JSON for deterministic comparison (sorted keys, indented)
			actual, err := normalizeJSON(body)
			if err != nil {
				t.Fatalf("failed to normalize actual JSON: %v", err)
			}

			goldenFile := filepath.Join(goldenDir, rgd.Feature+".json")

			if *update {
				if err := os.WriteFile(goldenFile, actual, 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("updated golden file: %s", goldenFile)
				return
			}

			expected, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("golden file not found: %s (run with -update to create)", goldenFile)
			}

			if string(actual) != string(expected) {
				t.Errorf("schema mismatch for %s\n--- expected ---\n%s\n--- actual ---\n%s",
					rgd.Feature, string(expected), string(actual))
			}
		})
	}
}

func TestCanaryRGDList(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	token := generateCanaryJWT()

	// Verify all reference RGDs appear in the catalog
	resp, err := makeAuthRequest(client, baseURL, "/api/v1/rgds", token)
	if err != nil {
		t.Fatalf("failed to list RGDs: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse RGD list: %v", err)
	}

	found := make(map[string]bool)
	for _, item := range result.Items {
		found[item.Name] = true
	}

	for _, rgd := range referenceRGDs {
		if !found[rgd.Name] {
			t.Errorf("reference RGD %q not found in catalog (KRO may not have processed it yet)", rgd.Name)
		}
	}
}

// normalizeJSON re-encodes JSON with sorted keys and consistent indentation.
func normalizeJSON(data []byte) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	normalized, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	return append(normalized, '\n'), nil
}

// findGoldenDir locates the expected/ directory relative to the reference RGDs.
// In update mode, creates the directory if it doesn't exist.
func findGoldenDir(t *testing.T) string {
	t.Helper()

	// Try relative to repo root (CI environment)
	candidates := []string{
		"deploy/test/reference-rgds/expected",
		"../../deploy/test/reference-rgds/expected",
		"../../../deploy/test/reference-rgds/expected",
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	// Try from REPO_ROOT env var
	if root := os.Getenv("REPO_ROOT"); root != "" {
		dir := filepath.Join(root, "deploy/test/reference-rgds/expected")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		// In update mode, create the directory if the parent exists
		if *update {
			parent := filepath.Join(root, "deploy/test/reference-rgds")
			if _, err := os.Stat(parent); err == nil {
				if err := os.MkdirAll(dir, 0755); err == nil {
					return dir
				}
			}
		}
	}

	// In update mode, try to create via relative path candidates
	if *update {
		for _, dir := range candidates {
			parent := filepath.Dir(dir)
			if _, err := os.Stat(parent); err == nil {
				if err := os.MkdirAll(dir, 0755); err == nil {
					abs, _ := filepath.Abs(dir)
					return abs
				}
			}
		}
	}

	t.Fatalf("cannot find golden file directory deploy/test/reference-rgds/expected/ (run with REPO_ROOT set or from repo root)")
	return ""
}

// generateCanaryJWT creates a JWT for the canary test (admin access).
func generateCanaryJWT() string {
	secret := getEnvOrDefault("E2E_JWT_SECRET", "test-jwt-secret-key-for-qa-testing-only")

	claims := jwt.MapClaims{
		"sub":          "canary@test.local",
		"email":        "canary@test.local",
		"name":         "Canary Test",
		"projects":     []string{},
		"casbin_roles": []string{"role:serveradmin"},
		"iss":          "knodex",
		"aud":          "knodex-api",
		"exp":          time.Now().Add(1 * time.Hour).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(fmt.Sprintf("failed to sign canary JWT: %v", err))
	}
	return tokenString
}

// makeAuthRequest executes an authenticated GET request.
func makeAuthRequest(client *http.Client, base, path, token string) (*http.Response, error) {
	url := strings.TrimRight(base, "/") + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return client.Do(req)
}
