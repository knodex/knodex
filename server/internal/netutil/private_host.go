// Package netutil provides network-related utility functions for SSRF protection.
package netutil

import (
	"fmt"
	"net"
)

// privateIPNets defines private and reserved IP ranges that must be blocked for SSRF protection.
var privateIPNets = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",       // Current network (RFC 1122)
		"127.0.0.0/8",     // Loopback
		"10.0.0.0/8",      // RFC 1918
		"100.64.0.0/10",   // Shared Address Space / CGNAT (RFC 6598)
		"172.16.0.0/12",   // RFC 1918
		"192.0.0.0/24",    // IETF Protocol Assignments (RFC 6890)
		"192.0.2.0/24",    // TEST-NET-1 (RFC 5737)
		"192.168.0.0/16",  // RFC 1918
		"169.254.0.0/16",  // Link-local
		"198.18.0.0/15",   // Benchmark testing (RFC 2544)
		"198.51.100.0/24", // TEST-NET-2 (RFC 5737)
		"203.0.113.0/24",  // TEST-NET-3 (RFC 5737)
		"224.0.0.0/4",     // Multicast (RFC 5771)
		"240.0.0.0/4",     // Reserved for future use (RFC 1112)
		"::1/128",         // IPv6 loopback
		"fe80::/10",       // IPv6 link-local
		"fc00::/7",        // IPv6 unique local
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("netutil: invalid CIDR %q: %v", cidr, err))
		}
		nets = append(nets, n)
	}
	return nets
}()

// IsPrivateHost checks whether a hostname resolves to or is a private/loopback IP address.
// If DNS resolution fails, the host is rejected (fail-closed for safety).
//
// NOTE: This check alone is vulnerable to DNS rebinding (TOCTOU). An attacker could make
// a hostname resolve to a public IP during this check, then switch DNS to a private IP
// before the actual connection. Callers making outbound HTTP requests MUST use
// NewSSRFSafeDialer() in their http.Transport to pin resolved IPs at connect time.
func IsPrivateHost(hostname string) bool {
	// Check if the hostname is itself an IP address
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip)
	}

	// Resolve hostname and check all returned IPs
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		// If we can't resolve, reject to be safe
		return true
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && isPrivateIP(ip) {
			return true
		}
	}
	return false
}

// isPrivateIP checks if an IP falls within any private/reserved range
func isPrivateIP(ip net.IP) bool {
	for _, n := range privateIPNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
