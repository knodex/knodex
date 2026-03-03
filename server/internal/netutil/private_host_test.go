package netutil

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		// Loopback (127.0.0.0/8)
		{"loopback 127.0.0.1", "127.0.0.1", true},
		{"loopback 127.0.0.2", "127.0.0.2", true},
		{"loopback 127.255.255.255", "127.255.255.255", true},

		// RFC 1918 - 10.0.0.0/8
		{"rfc1918 10.0.0.0", "10.0.0.0", true},
		{"rfc1918 10.0.0.1", "10.0.0.1", true},
		{"rfc1918 10.255.255.255", "10.255.255.255", true},

		// RFC 1918 - 172.16.0.0/12
		{"rfc1918 172.16.0.1", "172.16.0.1", true},
		{"rfc1918 172.31.255.255", "172.31.255.255", true},
		{"non-private 172.32.0.1", "172.32.0.1", false},

		// RFC 1918 - 192.168.0.0/16
		{"rfc1918 192.168.0.1", "192.168.0.1", true},
		{"rfc1918 192.168.255.255", "192.168.255.255", true},

		// Link-local (169.254.0.0/16) - includes cloud metadata endpoint
		{"link-local 169.254.0.1", "169.254.0.1", true},
		{"cloud-metadata 169.254.169.254", "169.254.169.254", true},

		// IPv6 loopback
		{"ipv6-loopback", "::1", true},

		// IPv6 link-local
		{"ipv6-link-local", "fe80::1", true},

		// IPv6 unique local
		{"ipv6-unique-local", "fc00::1", true},
		{"ipv6-unique-local-fd", "fd00::1", true},

		// Current network (0.0.0.0/8)
		{"current-network 0.0.0.1", "0.0.0.1", true},
		{"current-network 0.255.255.255", "0.255.255.255", true},

		// CGNAT / Shared Address Space (100.64.0.0/10)
		{"cgnat 100.64.0.1", "100.64.0.1", true},
		{"cgnat 100.100.100.100", "100.100.100.100", true},
		{"cgnat 100.127.255.254", "100.127.255.254", true},
		{"non-cgnat 100.128.0.1", "100.128.0.1", false},

		// Benchmark testing (198.18.0.0/15)
		{"benchmark 198.18.0.1", "198.18.0.1", true},
		{"benchmark 198.19.255.254", "198.19.255.254", true},
		{"non-benchmark 198.20.0.1", "198.20.0.1", false},

		// IETF Protocol Assignments (192.0.0.0/24)
		{"ietf-protocol 192.0.0.1", "192.0.0.1", true},

		// TEST-NET ranges (RFC 5737)
		{"test-net-1 192.0.2.1", "192.0.2.1", true},
		{"test-net-2 198.51.100.1", "198.51.100.1", true},
		{"test-net-3 203.0.113.1", "203.0.113.1", true},

		// Multicast (224.0.0.0/4)
		{"multicast 224.0.0.1", "224.0.0.1", true},
		{"multicast 239.255.255.255", "239.255.255.255", true},

		// Reserved (240.0.0.0/4)
		{"reserved 240.0.0.1", "240.0.0.1", true},
		{"reserved 255.255.255.254", "255.255.255.254", true},

		// Public IPs
		{"public 8.8.8.8", "8.8.8.8", false},
		{"public 1.1.1.1", "1.1.1.1", false},
		{"public 93.184.216.34", "93.184.216.34", false},
		{"public ipv6 2607:f8b0", "2607:f8b0::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			got := isPrivateIP(ip)
			if got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestIsPrivateHost_IPAddresses(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		private bool
	}{
		// Direct IP addresses (no DNS resolution needed)
		{"loopback", "127.0.0.1", true},
		{"rfc1918-10", "10.0.0.1", true},
		{"rfc1918-172", "172.16.0.1", true},
		{"rfc1918-192", "192.168.1.1", true},
		{"link-local", "169.254.169.254", true},
		{"ipv6-loopback", "::1", true},
		{"ipv6-link-local", "fe80::1", true},
		{"public-ip", "8.8.8.8", false},
		{"public-ip-cloudflare", "1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPrivateHost(tt.host)
			if got != tt.private {
				t.Errorf("IsPrivateHost(%q) = %v, want %v", tt.host, got, tt.private)
			}
		})
	}
}

func TestIsPrivateHost_Hostnames(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		private bool
	}{
		// Well-known public hostnames
		{"github.com", "github.com", false},
		{"gitlab.com", "gitlab.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPrivateHost(tt.host)
			if got != tt.private {
				t.Errorf("IsPrivateHost(%q) = %v, want %v", tt.host, got, tt.private)
			}
		})
	}
}

func TestIsPrivateHost_UnresolvableHostname(t *testing.T) {
	// Unresolvable hostnames should be treated as private (fail-closed)
	got := IsPrivateHost("definitely-not-a-real-host-12345.invalid")
	if !got {
		t.Error("IsPrivateHost should return true for unresolvable hostnames (fail-closed)")
	}
}
