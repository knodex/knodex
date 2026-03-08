// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package netutil

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeResolver returns preconfigured addresses for LookupHost calls.
type fakeResolver struct {
	addrs []string
	err   error
}

func (f *fakeResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	return f.addrs, f.err
}

func TestNewSSRFSafeDialer_BlocksPrivateIPs(t *testing.T) {
	dialer := NewSSRFSafeDialer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	privateAddrs := []struct {
		name string
		addr string
	}{
		{"loopback", "127.0.0.1:80"},
		{"rfc1918-10", "10.0.0.1:80"},
		{"rfc1918-172", "172.16.0.1:80"},
		{"rfc1918-192", "192.168.1.1:80"},
		{"link-local", "169.254.169.254:80"},
		{"ipv6-loopback", "[::1]:80"},
		{"current-network", "0.0.0.1:80"},
		{"cgnat", "100.64.0.1:80"},
	}

	for _, tt := range privateAddrs {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := dialer(ctx, "tcp", tt.addr)
			if conn != nil {
				conn.Close()
				t.Fatal("expected connection to be blocked, but got a connection")
			}
			if err == nil {
				t.Fatal("expected error for private IP, got nil")
			}
			if !strings.Contains(err.Error(), "private/reserved") {
				t.Fatalf("expected SSRF block error, got: %v", err)
			}
		})
	}
}

func TestNewSSRFSafeDialer_AllowsPublicIP(t *testing.T) {
	// Use a fake resolver that returns a known public IP so we don't make real
	// network calls. The dialer will attempt to connect to 93.184.216.34:1
	// (a public IP) which will fail with a connection error, NOT an SSRF error.
	resolver := &fakeResolver{addrs: []string{"93.184.216.34"}}
	dialer := NewSSRFSafeDialer(resolver)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", "example.com:1")
	if conn != nil {
		conn.Close()
	}

	// We expect a connection error (refused/timeout), NOT an SSRF error
	if err != nil && isSSRFError(err) {
		t.Fatalf("public IP should NOT be blocked by SSRF dialer, got: %v", err)
	}
}

func TestNewSSRFSafeDialer_BlocksDNSFailure(t *testing.T) {
	dialer := NewSSRFSafeDialer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Unresolvable hostname should be blocked (fail-closed)
	conn, err := dialer(ctx, "tcp", "definitely-not-a-real-host-98765.invalid:80")
	if conn != nil {
		conn.Close()
		t.Fatal("expected connection to be blocked for unresolvable host")
	}
	if err == nil {
		t.Fatal("expected error for unresolvable hostname, got nil")
	}
	if !strings.Contains(err.Error(), "DNS resolution failed") {
		t.Fatalf("expected DNS failure error, got: %v", err)
	}
}

func TestNewSSRFSafeDialer_InvalidAddress(t *testing.T) {
	dialer := NewSSRFSafeDialer()
	ctx := context.Background()

	// Missing port
	conn, err := dialer(ctx, "tcp", "127.0.0.1")
	if conn != nil {
		conn.Close()
	}
	if err == nil {
		t.Fatal("expected error for address without port")
	}
	if !strings.Contains(err.Error(), "invalid address") {
		t.Fatalf("expected invalid address error, got: %v", err)
	}
}

func TestNewSSRFSafeDialer_HostnameResolvesToPrivateIP(t *testing.T) {
	// Simulate a hostname that resolves to a private IP (e.g., internal.corp → 10.0.0.1).
	// The dialer must block this even though the input is a hostname, not a raw IP.
	resolver := &fakeResolver{addrs: []string{"10.0.0.1"}}
	dialer := NewSSRFSafeDialer(resolver)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", "internal.corp:80")
	if conn != nil {
		conn.Close()
		t.Fatal("expected connection to be blocked for hostname resolving to private IP")
	}
	if err == nil {
		t.Fatal("expected SSRF error, got nil")
	}
	if !strings.Contains(err.Error(), "private/reserved") {
		t.Fatalf("expected private/reserved block error, got: %v", err)
	}
}

func TestNewSSRFSafeDialer_MixedPublicPrivateIPs(t *testing.T) {
	// Simulate a hostname that resolves to both a public and a private IP.
	// The dialer MUST reject this (fail-closed: ANY private IP = blocked).
	// This is the core DNS rebinding defense — an attacker can add a public IP
	// alongside a private one to try to slip through.
	resolver := &fakeResolver{addrs: []string{"93.184.216.34", "169.254.169.254"}}
	dialer := NewSSRFSafeDialer(resolver)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", "attacker.example.com:80")
	if conn != nil {
		conn.Close()
		t.Fatal("expected connection to be blocked for mixed public/private IPs")
	}
	if err == nil {
		t.Fatal("expected SSRF error for mixed IPs, got nil")
	}
	if !strings.Contains(err.Error(), "private/reserved") {
		t.Fatalf("expected private/reserved block error, got: %v", err)
	}
}

func TestIsPrivateIP_Exported(t *testing.T) {
	// Verify the exported wrapper works
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"8.8.8.8", false},
		{"169.254.169.254", true},
		{"::1", true},
		{"2607:f8b0::1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %q", tt.ip)
		}
		got := IsPrivateIP(ip)
		if got != tt.private {
			t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func TestNewSSRFSafeDialer_DefaultResolver(t *testing.T) {
	// Verify that calling NewSSRFSafeDialer() with no args uses net.DefaultResolver
	// and still blocks private IPs correctly.
	dialer := NewSSRFSafeDialer()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", "127.0.0.1:80")
	if conn != nil {
		conn.Close()
	}
	if err == nil {
		t.Fatal("expected SSRF error with default resolver")
	}
	if !strings.Contains(err.Error(), "private/reserved") {
		t.Fatalf("expected private/reserved block error, got: %v", err)
	}
}

// isSSRFError checks if an error is from SSRF validation (not a network error)
func isSSRFError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "private/reserved") ||
		strings.Contains(errStr, "ssrf dialer")
}
