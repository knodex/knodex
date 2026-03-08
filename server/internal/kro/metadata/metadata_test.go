package metadata

import (
	"testing"
)

func TestLabelWithFallback(t *testing.T) {
	labels := map[string]string{
		"kro.run/resource-graph-definition-name": "my-rgd",
		"legacy-key":                             "old-value",
	}

	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"primary key found", []string{"kro.run/resource-graph-definition-name"}, "my-rgd"},
		{"fallback key used", []string{"missing-key", "legacy-key"}, "old-value"},
		{"no keys match", []string{"no-such-key", "also-missing"}, ""},
		{"empty keys", []string{}, ""},
		{"nil labels", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := labels
			if tt.name == "nil labels" {
				input = nil
			}
			got := LabelWithFallback(input, tt.keys...)
			if got != tt.want {
				t.Errorf("LabelWithFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}
