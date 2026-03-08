// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"strings"
	"testing"
)

func TestValidateHTTPSRepoURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Valid external URLs (must use resolvable domains)
		{"github HTTPS", "https://github.com/org/repo.git", false, ""},
		{"gitlab HTTPS", "https://gitlab.com/org/repo.git", false, ""},

		// Scheme validation
		{"HTTP rejected", "http://github.com/org/repo.git", true, "must start with 'https://'"},
		{"SSH rejected", "git@github.com:org/repo.git", true, "must start with 'https://'"},

		// SSRF: cloud metadata
		{"SSRF cloud metadata", "https://169.254.169.254/latest/meta-data", true, "private or internal"},

		// SSRF: loopback
		{"SSRF loopback", "https://127.0.0.1/repo.git", true, "private or internal"},

		// SSRF: RFC 1918 ranges
		{"SSRF 10.x", "https://10.0.0.1/repo.git", true, "private or internal"},
		{"SSRF 172.16.x", "https://172.16.0.1/repo.git", true, "private or internal"},
		{"SSRF 192.168.x", "https://192.168.1.1/repo.git", true, "private or internal"},

		// SSRF: CGNAT / Shared Address Space (RFC 6598)
		{"SSRF CGNAT", "https://100.64.0.1/repo.git", true, "private or internal"},
		{"SSRF CGNAT upper", "https://100.127.255.254/repo.git", true, "private or internal"},

		// SSRF: current network
		{"SSRF 0.0.0.0", "https://0.0.0.1/repo.git", true, "private or internal"},

		// SSRF: IPv6
		{"SSRF IPv6 loopback", "https://[::1]/repo.git", true, "private or internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPSRepoURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateHTTPSRepoURL(%q) = nil, want error containing %q", tt.url, tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateHTTPSRepoURL(%q) error = %q, want error containing %q", tt.url, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateHTTPSRepoURL(%q) = %v, want nil", tt.url, err)
				}
			}
		})
	}
}

func TestValidateEnterpriseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Valid (must use resolvable public domains)
		{"valid enterprise URL", "https://github.com", false, ""},
		{"valid with path", "https://gitlab.com/api/v4", false, ""},

		// Scheme enforcement
		{"HTTP rejected", "http://github.example.com", true, "must use HTTPS"},
		{"no scheme", "github.example.com", true, "must be a valid URL"},

		// SSRF: cloud metadata
		{"SSRF cloud metadata", "http://169.254.169.254/", true, "must use HTTPS"},
		{"SSRF cloud metadata HTTPS", "https://169.254.169.254/", true, "private or internal"},

		// SSRF: loopback
		{"SSRF loopback", "https://127.0.0.1/", true, "private or internal"},

		// SSRF: RFC 1918
		{"SSRF 10.x", "https://10.0.0.1/", true, "private or internal"},
		{"SSRF 192.168.x", "https://192.168.1.1/", true, "private or internal"},
		{"SSRF 172.16.x", "https://172.16.0.1/", true, "private or internal"},

		// SSRF: link-local
		{"SSRF link-local", "https://169.254.0.1/", true, "private or internal"},

		// SSRF: CGNAT (RFC 6598)
		{"SSRF CGNAT", "https://100.64.0.1/", true, "private or internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnterpriseURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateEnterpriseURL(%q) = nil, want error containing %q", tt.url, tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateEnterpriseURL(%q) error = %q, want error containing %q", tt.url, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateEnterpriseURL(%q) = %v, want nil", tt.url, err)
				}
			}
		})
	}
}

func TestValidateSSHRepoURL_SSRF(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Valid external SSH URLs
		{"valid git@", "git@github.com:org/repo.git", false, ""},
		{"valid ssh://", "ssh://git@github.com/org/repo.git", false, ""},

		// SSRF: git@ format with private hosts
		{"SSRF git@ loopback", "git@127.0.0.1:org/repo.git", true, "private or internal"},
		{"SSRF git@ RFC1918 10.x", "git@10.0.0.1:org/repo.git", true, "private or internal"},
		{"SSRF git@ RFC1918 192.168.x", "git@192.168.1.1:org/repo.git", true, "private or internal"},
		{"SSRF git@ link-local", "git@169.254.169.254:org/repo.git", true, "private or internal"},

		// SSRF: ssh:// format with private hosts
		{"SSRF ssh:// loopback", "ssh://git@127.0.0.1/org/repo.git", true, "private or internal"},
		{"SSRF ssh:// RFC1918", "ssh://git@10.0.0.1/org/repo.git", true, "private or internal"},
		{"SSRF ssh:// IPv6 loopback", "ssh://git@[::1]/org/repo.git", true, "private or internal"},

		// Invalid format
		{"invalid format", "https://github.com/org/repo.git", true, "must start with"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSSHRepoURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSSHRepoURL(%q) = nil, want error containing %q", tt.url, tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSSHRepoURL(%q) error = %q, want error containing %q", tt.url, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSSHRepoURL(%q) = %v, want nil", tt.url, err)
				}
			}
		})
	}
}
