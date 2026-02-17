package rest

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// extractClientIP extracts the client IP address from the request.
// When trustProxy is true, the firewall will check X-Real-IP and
// X-Forwarded-For headers to determine the client's IP address,
// which is necessary when the application runs behind a reverse proxy.
func extractClientIP(r *http.Request, trustProxy bool) (string, error) {
	if trustProxy {
		if clientIP := tryExtractFromProxyHeaders(r); clientIP != "" {
			return clientIP, nil
		}
	}

	return extractFromRemoteAddr(r.RemoteAddr)
}

// mustExtractClientIP behaves exactly like extractClientIP except it
// doesn't return an error, ignoring it instead.
func mustExtractClientIP(r *http.Request, trustProxy bool) string {
	ip, _ := extractClientIP(r, trustProxy)

	return ip
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
		clientIP := strings.TrimSpace(ips[0])

		if ip := net.ParseIP(clientIP); ip != nil {
			return ip.String()
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
