// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package netutil provides network-related utility functions for SSRF protection.
package netutil

import (
	"context"
	"fmt"
	"net"
)

// IsPrivateIP reports whether ip falls within any private or reserved IP range.
// This is a public wrapper around isPrivateIP for use in custom dialers and external packages.
func IsPrivateIP(ip net.IP) bool {
	return isPrivateIP(ip)
}

// Resolver abstracts hostname resolution for testability.
type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// defaultResolver wraps net.DefaultResolver to satisfy the Resolver interface.
type defaultResolver struct{}

func (defaultResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

// NewSSRFSafeDialer returns a DialContext function that resolves hostnames,
// validates ALL resolved IPs against private/reserved ranges, and connects
// to the first valid public IP. This closes the DNS rebinding TOCTOU window
// that IsPrivateHost is vulnerable to: the IP is pinned at connect time.
//
// Behavior:
//   - Direct IP addresses are validated before connection.
//   - Hostnames are resolved via the provided resolver (or net.DefaultResolver if none given).
//   - If ANY resolved IP is private/reserved, the connection is rejected (fail-closed).
//   - DNS resolution failure rejects the connection (fail-closed).
//   - The connection is made to the resolved IP directly, preventing TOCTOU attacks.
func NewSSRFSafeDialer(resolvers ...Resolver) func(ctx context.Context, network, addr string) (net.Conn, error) {
	var resolver Resolver = defaultResolver{}
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("ssrf dialer: invalid address %q: %w", addr, err)
		}

		// Resolve the host to IP addresses
		var ips []net.IP
		if ip := net.ParseIP(host); ip != nil {
			// Host is already an IP address
			ips = []net.IP{ip}
		} else {
			// Resolve hostname
			addrs, lookupErr := resolver.LookupHost(ctx, host)
			if lookupErr != nil {
				// Fail-closed: DNS failure blocks the connection
				return nil, fmt.Errorf("ssrf dialer: DNS resolution failed for %q: %w", host, lookupErr)
			}
			if len(addrs) == 0 {
				return nil, fmt.Errorf("ssrf dialer: no addresses found for %q", host)
			}
			for _, a := range addrs {
				ip := net.ParseIP(a)
				if ip == nil {
					return nil, fmt.Errorf("ssrf dialer: invalid resolved IP %q for host %q", a, host)
				}
				ips = append(ips, ip)
			}
		}

		// Validate ALL resolved IPs — reject if ANY is private (fail-closed)
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return nil, fmt.Errorf("ssrf dialer: connection to private/reserved address %s blocked", ip)
			}
		}

		// Connect to the first resolved IP directly (pinned IP, no TOCTOU)
		pinnedAddr := net.JoinHostPort(ips[0].String(), port)
		return (&net.Dialer{}).DialContext(ctx, network, pinnedAddr)
	}
}
