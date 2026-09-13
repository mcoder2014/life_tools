package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigJSONAndLegacyYAML(t *testing.T) {
	for name, content := range map[string]string{
		"config.json":        `{"cloudflare":{"api_token":"test-token","zone":"test-zone","proxy":"127.0.0.1:1080","debug":true},"ddns_config":[{"domain":"pi.example.org","ip_version":"ipv4"},{"domain":"pi6.example.org","ip_version":"ipv6"}]}`,
		"client_config.yaml": "cloudflare:\n  api_token: test-token\n  zone: test-zone\n  proxy: 127.0.0.1:1080\n  debug: true\nddns_config:\n  - domain: pi.example.org\n    ip_version: ipv4\n  - domain: pi6.example.org\n    ip_version: ipv6\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			require.NoError(t, os.WriteFile(path, []byte(content), 0600))
			config, err := LoadConfig(path)
			require.NoError(t, err)
			require.Equal(t, "test-token", config.Cloudflare.APIToken)
			require.Equal(t, "test-zone", config.Cloudflare.Zone)
			require.Equal(t, "127.0.0.1:1080", config.Cloudflare.Proxy)
			require.True(t, config.Cloudflare.Debug)
			require.Equal(t, []DomainConfig{{Domain: "pi.example.org", IPVersion: "ipv4"}, {Domain: "pi6.example.org", IPVersion: "ipv6"}}, config.DDNSConfig)
		})
	}
}

func TestRejectInvalidConfig(t *testing.T) {
	for name, content := range map[string]string{
		"empty":           `{}`,
		"missing token":   `{"cloudflare":{"zone":"test-zone"},"ddns_config":[{"domain":"a.example.org","ip_version":"ipv4"}]}`,
		"missing zone":    `{"cloudflare":{"api_token":"test-token"},"ddns_config":[{"domain":"a.example.org","ip_version":"ipv4"}]}`,
		"missing domains": `{"cloudflare":{"api_token":"test-token","zone":"test-zone"}}`,
		"unknown version": `{"cloudflare":{"api_token":"test-token","zone":"test-zone"},"ddns_config":[{"domain":"a.example.org","ip_version":"ipv5"}]}`,
		"null domain":     `{"cloudflare":{"api_token":"test-token","zone":"test-zone"},"ddns_config":[null]}`,
		"invalid json":    `{"cloudflare":`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(content), 0600))
			_, err := LoadConfig(path)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "test-token")
		})
	}
}
