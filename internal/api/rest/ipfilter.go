package rest

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Firewall implements IP-based access control for HTTP requests.
// It maintains a list of allowed IP addresses and CIDR ranges, and can
// optionally trust reverse proxy headers (X-Real-IP, X-Forwarded-For) to
// determine the client's real IP address.
type Firewall struct {
	allowedNets []*net.IPNet
	trustProxy  bool
}

// NewFirewall creates a new Firewall instance with the specified configuration.
// The allowedIPsStr parameter accepts a comma-separated list of IP addresses
// and/or CIDR ranges (e.g., "10.0.0.0/24,192.168.1.1,2001:db8::/32").
// If allowedIPsStr is empty, all IP addresses are allowed by default.
// When trustProxy is true, the firewall will check X-Real-IP and X-Forwarded-For
// headers to determine the client's IP address, which is necessary when the
// application runs behind a reverse proxy.
// Returns an error if any IP address or CIDR notation is invalid.
func NewFirewall(allowedIPsStr string, trustProxy bool) (*Firewall, error) {
	if allowedIPsStr == "" {
		return &Firewall{
			allowedNets: nil,
			trustProxy:  trustProxy,
		}, nil
	}

	allowedNets, err := parseAllowedIPs(allowedIPsStr)

	if err != nil {
		return nil, err
	}

	return &Firewall{
		allowedNets: allowedNets,
		trustProxy:  trustProxy,
	}, nil
}

// IsAllowed checks if the request originates from an allowed IP address.
// When trustProxy is enabled, it first checks X-Real-IP and X-Forwarded-For headers
// before falling back to RemoteAddr. If no IP restrictions are configured (empty allowlist),
// all requests are allowed. Returns false if the IP cannot be extracted or is not in the allowlist.
func (f *Firewall) IsAllowed(r *http.Request) bool {
	if len(f.allowedNets) == 0 {
		return true
	}

	clientIP, err := extractClientIP(r, f.trustProxy)

	if err != nil {
		return false
	}

	return isIPAllowed(clientIP, f.allowedNets)
}

// extractClientIP extracts the client IP address from the request
func extractClientIP(r *http.Request, trustProxy bool) (string, error) {
	if trustProxy {
		if clientIP := tryExtractFromProxyHeaders(r); clientIP != "" {
			return clientIP, nil
		}
	}

	return extractFromRemoteAddr(r.RemoteAddr)
}

// tryExtractFromProxyHeaders attempts to extract IP from proxy headers
func tryExtractFromProxyHeaders(r *http.Request) string {
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		if ip := net.ParseIP(xRealIP); ip != nil {
			return ip.String()
		}
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")

		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])

			if ip := net.ParseIP(clientIP); ip != nil {
				return ip.String()
			}
		}
	}

	return ""
}

// extractFromRemoteAddr extracts IP from RemoteAddr
func extractFromRemoteAddr(remoteAddr string) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddr)

	if err != nil {
		return "", fmt.Errorf("invalid remote address: %w", err)
	}

	ip := net.ParseIP(host)

	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", host)
	}

	return ip.String(), nil
}

// parseAllowedIPs parses a comma-separated list of IP addresses and CIDR ranges
func parseAllowedIPs(allowedIPsStr string) ([]*net.IPNet, error) {
	parts := strings.Split(allowedIPsStr, ",")
	allowedNets := make([]*net.IPNet, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		ipNet, err := parseIPOrCIDR(part)

		if err != nil {
			return nil, err
		}

		allowedNets = append(allowedNets, ipNet)
	}

	return allowedNets, nil
}

// parseIPOrCIDR parses a single IP or CIDR notation
func parseIPOrCIDR(part string) (*net.IPNet, error) {
	if strings.Contains(part, "/") {
		_, ipNet, err := net.ParseCIDR(part)

		if err != nil {
			return nil, fmt.Errorf("invalid CIDR notation %q: %w", part, err)
		}

		return ipNet, nil
	}

	ip := net.ParseIP(part)

	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %q", part)
	}

	cidr := part + "/32"

	if ip.To4() == nil {
		cidr = part + "/128"
	}

	_, ipNet, err := net.ParseCIDR(cidr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse IP %q as CIDR: %w", part, err)
	}

	return ipNet, nil
}

// isIPAllowed checks if an IP address is in the allowed networks
func isIPAllowed(ipStr string, allowedNets []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)

	if ip == nil {
		return false
	}

	for _, ipNet := range allowedNets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}
