package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChosoMeister/tg2rss/internal/api/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFirewall(t *testing.T) {
	tests := []struct {
		name        string
		allowedIPs  string
		trustProxy  bool
		expectError bool
	}{
		{
			name:        "empty allowed IPs",
			allowedIPs:  "",
			trustProxy:  false,
			expectError: false,
		},
		{
			name:        "single IPv4",
			allowedIPs:  "192.168.1.1",
			trustProxy:  false,
			expectError: false,
		},
		{
			name:        "single IPv6",
			allowedIPs:  "2001:db8::1",
			trustProxy:  false,
			expectError: false,
		},
		{
			name:        "IPv4 CIDR",
			allowedIPs:  "10.0.0.0/24",
			trustProxy:  false,
			expectError: false,
		},
		{
			name:        "IPv6 CIDR",
			allowedIPs:  "2001:db8::/32",
			trustProxy:  false,
			expectError: false,
		},
		{
			name:        "multiple IPs mixed",
			allowedIPs:  "192.168.1.1,10.0.0.0/24,2001:db8::1",
			trustProxy:  true,
			expectError: false,
		},
		{
			name:        "invalid IP",
			allowedIPs:  "999.999.999.999",
			trustProxy:  false,
			expectError: true,
		},
		{
			name:        "invalid CIDR",
			allowedIPs:  "192.168.1.1/99",
			trustProxy:  false,
			expectError: true,
		},
		{
			name:        "mixed with spaces",
			allowedIPs:  "192.168.1.1 , 10.0.0.0/24 ,  2001:db8::1  ",
			trustProxy:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firewall, err := rest.NewFirewall(tt.allowedIPs, tt.trustProxy)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, firewall)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, firewall)
			}
		})
	}
}

func TestFirewall_IsAllowed_IPv4(t *testing.T) {
	tests := []struct {
		name       string
		allowedIPs string
		remoteAddr string
		expected   bool
	}{
		{
			name:       "exact match IPv4",
			allowedIPs: "192.168.1.100",
			remoteAddr: "192.168.1.100:12345",
			expected:   true,
		},
		{
			name:       "no match IPv4",
			allowedIPs: "192.168.1.100",
			remoteAddr: "192.168.1.101:12345",
			expected:   false,
		},
		{
			name:       "CIDR match IPv4",
			allowedIPs: "10.0.0.0/24",
			remoteAddr: "10.0.0.50:12345",
			expected:   true,
		},
		{
			name:       "CIDR no match IPv4",
			allowedIPs: "10.0.0.0/24",
			remoteAddr: "10.0.1.50:12345",
			expected:   false,
		},
		{
			name:       "multiple IPs - match first",
			allowedIPs: "192.168.1.1,10.0.0.0/24",
			remoteAddr: "192.168.1.1:12345",
			expected:   true,
		},
		{
			name:       "multiple IPs - match second",
			allowedIPs: "192.168.1.1,10.0.0.0/24",
			remoteAddr: "10.0.0.50:12345",
			expected:   true,
		},
		{
			name:       "multiple IPs - no match",
			allowedIPs: "192.168.1.1,10.0.0.0/24",
			remoteAddr: "172.16.0.1:12345",
			expected:   false,
		},
		{
			name:       "empty list allows all",
			allowedIPs: "",
			remoteAddr: "1.2.3.4:12345",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firewall, err := rest.NewFirewall(tt.allowedIPs, false)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			result := firewall.IsAllowed(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirewall_IsAllowed_IPv6(t *testing.T) {
	tests := []struct {
		name       string
		allowedIPs string
		remoteAddr string
		expected   bool
	}{
		{
			name:       "exact match IPv6",
			allowedIPs: "2001:db8::1",
			remoteAddr: "[2001:db8::1]:12345",
			expected:   true,
		},
		{
			name:       "no match IPv6",
			allowedIPs: "2001:db8::1",
			remoteAddr: "[2001:db8::2]:12345",
			expected:   false,
		},
		{
			name:       "CIDR match IPv6",
			allowedIPs: "2001:db8::/32",
			remoteAddr: "[2001:db8::50]:12345",
			expected:   true,
		},
		{
			name:       "CIDR no match IPv6",
			allowedIPs: "2001:db8::/32",
			remoteAddr: "[2001:db9::1]:12345",
			expected:   false,
		},
		{
			name:       "IPv6 loopback",
			allowedIPs: "::1",
			remoteAddr: "[::1]:12345",
			expected:   true,
		},
		{
			name:       "mixed IPv4 and IPv6 - match IPv6",
			allowedIPs: "192.168.1.1,2001:db8::1",
			remoteAddr: "[2001:db8::1]:12345",
			expected:   true,
		},
		{
			name:       "mixed IPv4 and IPv6 - match IPv4",
			allowedIPs: "192.168.1.1,2001:db8::1",
			remoteAddr: "192.168.1.1:12345",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firewall, err := rest.NewFirewall(tt.allowedIPs, false)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			result := firewall.IsAllowed(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirewall_IsAllowed_ProxyHeaders(t *testing.T) {
	tests := []struct {
		name        string
		allowedIPs  string
		trustProxy  bool
		remoteAddr  string
		xRealIP     string
		xForwardFor string
		expected    bool
	}{
		{
			name:        "trust proxy - X-Real-IP allowed",
			allowedIPs:  "203.0.113.1",
			trustProxy:  true,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "203.0.113.1",
			xForwardFor: "",
			expected:    true,
		},
		{
			name:        "trust proxy - X-Real-IP denied",
			allowedIPs:  "203.0.113.1",
			trustProxy:  true,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "203.0.113.2",
			xForwardFor: "",
			expected:    false,
		},
		{
			name:        "trust proxy - X-Forwarded-For allowed",
			allowedIPs:  "203.0.113.1",
			trustProxy:  true,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "",
			xForwardFor: "203.0.113.1, 10.0.0.2",
			expected:    true,
		},
		{
			name:        "trust proxy - X-Forwarded-For denied",
			allowedIPs:  "203.0.113.1",
			trustProxy:  true,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "",
			xForwardFor: "203.0.113.2, 10.0.0.2",
			expected:    false,
		},
		{
			name:        "no trust proxy - ignore X-Real-IP",
			allowedIPs:  "203.0.113.1",
			trustProxy:  false,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "203.0.113.1",
			xForwardFor: "",
			expected:    false,
		},
		{
			name:        "no trust proxy - ignore X-Forwarded-For",
			allowedIPs:  "203.0.113.1",
			trustProxy:  false,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "",
			xForwardFor: "203.0.113.1, 10.0.0.2",
			expected:    false,
		},
		{
			name:        "no trust proxy - use RemoteAddr",
			allowedIPs:  "10.0.0.1",
			trustProxy:  false,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "203.0.113.1",
			xForwardFor: "203.0.113.1",
			expected:    true,
		},
		{
			name:        "trust proxy - X-Real-IP priority over X-Forwarded-For",
			allowedIPs:  "203.0.113.1",
			trustProxy:  true,
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "203.0.113.1",
			xForwardFor: "203.0.113.2",
			expected:    true,
		},
		{
			name:        "trust proxy - IPv6 in X-Real-IP",
			allowedIPs:  "2001:db8::1",
			trustProxy:  true,
			remoteAddr:  "[::1]:12345",
			xRealIP:     "2001:db8::1",
			xForwardFor: "",
			expected:    true,
		},
		{
			name:        "trust proxy - IPv6 in X-Forwarded-For",
			allowedIPs:  "2001:db8::1",
			trustProxy:  true,
			remoteAddr:  "[::1]:12345",
			xRealIP:     "",
			xForwardFor: "2001:db8::1, ::1",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firewall, err := rest.NewFirewall(tt.allowedIPs, tt.trustProxy)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if tt.xForwardFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardFor)
			}

			result := firewall.IsAllowed(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirewall_IsAllowed_InvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		allowedIPs string
		remoteAddr string
		expected   bool
	}{
		{
			name:       "invalid RemoteAddr format",
			allowedIPs: "192.168.1.1",
			remoteAddr: "invalid",
			expected:   false,
		},
		{
			name:       "empty RemoteAddr",
			allowedIPs: "192.168.1.1",
			remoteAddr: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firewall, err := rest.NewFirewall(tt.allowedIPs, false)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			result := firewall.IsAllowed(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirewall_IsAllowed_EdgeCases(t *testing.T) {
	t.Run("allows all when no IPs configured", func(t *testing.T) {
		firewall, err := rest.NewFirewall("", false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:12345"

		result := firewall.IsAllowed(req)
		assert.True(t, result)
	})

	t.Run("handles localhost IPv4", func(t *testing.T) {
		firewall, err := rest.NewFirewall("127.0.0.1", false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		result := firewall.IsAllowed(req)
		assert.True(t, result)
	})

	t.Run("handles localhost IPv6", func(t *testing.T) {
		firewall, err := rest.NewFirewall("::1", false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "[::1]:12345"

		result := firewall.IsAllowed(req)
		assert.True(t, result)
	})

	t.Run("handles private networks", func(t *testing.T) {
		firewall, err := rest.NewFirewall("10.0.0.0/8,172.16.0.0/12,192.168.0.0/16", false)
		require.NoError(t, err)

		privateIPs := []string{
			"10.1.2.3:12345",
			"172.16.0.1:12345",
			"172.31.255.254:12345",
			"192.168.100.50:12345",
		}

		for _, addr := range privateIPs {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = addr

			result := firewall.IsAllowed(req)
			assert.True(t, result, "expected %s to be allowed", addr)
		}

		publicIP := "8.8.8.8:12345"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = publicIP
		result := firewall.IsAllowed(req)
		assert.False(t, result, "expected %s to be denied", publicIP)
	})
}
