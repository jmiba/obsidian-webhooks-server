package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCookieSecurityFollowsMagicLinkURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected bool
	}{
		{name: "local HTTP", baseURL: "http://localhost:8070", expected: false},
		{name: "production HTTPS", baseURL: "https://webhooks.example.com", expected: true},
		{name: "uppercase HTTPS", baseURL: "HTTPS://webhooks.example.com", expected: true},
		{name: "invalid URL", baseURL: "://invalid", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isHTTPSURL(test.baseURL))
		})
	}
}

func TestCookieSecurityCanBeOverridden(t *testing.T) {
	t.Setenv("MAGIC_LINK_BASE_URL", "https://webhooks.example.com")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("JWT_SECRET", "test-secret")

	assert.False(t, Load().CookieSecure)
}
