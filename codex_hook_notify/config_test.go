package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestMatchingURLs(t *testing.T) {
	config := Config{
		Routes: []Route{
			{
				Events:                []string{"Stop"},
				FeishuCustomRobotURLs: []string{"https://example.com/stop"},
			},
			{
				Events:                []string{"PermissionRequest", "Stop"},
				FeishuCustomRobotURLs: []string{"https://example.com/common"},
			},
		},
	}

	require.Equal(t, []string{
		"https://example.com/stop",
		"https://example.com/common",
	}, config.MatchingURLs("Stop"))
	require.Equal(t, []string{
		"https://example.com/common",
	}, config.MatchingURLs("PermissionRequest"))
	require.Empty(t, config.MatchingURLs("PreToolUse"))
}

func TestMatchingURLsSkipsEmptyRoutes(t *testing.T) {
	config := Config{
		Routes: []Route{
			{
				Events:                []string{"Stop"},
				FeishuCustomRobotURLs: []string{"", "https://example.com/stop"},
			},
			{
				Events:                nil,
				FeishuCustomRobotURLs: []string{"https://example.com/ignored"},
			},
		},
	}

	require.Equal(t, []string{"https://example.com/stop"}, config.MatchingURLs("Stop"))
}

func TestLoadConfigReadsMachineName(t *testing.T) {
	config, err := LoadConfig([]byte(`{
		"machine_name": "home-nas",
		"routes": [
			{"events": ["Stop"], "feishu_custom_robot_urls": ["https://example.com/stop"]}
		]
	}`))

	require.NoError(t, err)
	require.Equal(t, "home-nas", config.MachineName)
	require.Equal(t, []string{"https://example.com/stop"}, config.MatchingURLs("Stop"))
}

func TestResolveMachineNameUsesConfigBeforeOS(t *testing.T) {
	name := ResolveMachineName(Config{MachineName: "  custom-box  "}, func() (string, error) {
		return "os-host", nil
	})

	require.Equal(t, "custom-box", name)
}

func TestResolveMachineNameFallsBackToOS(t *testing.T) {
	name := ResolveMachineName(Config{}, func() (string, error) {
		return "os-host", nil
	})

	require.Equal(t, "os-host", name)
}

func TestResolveMachineNameFallsBackToUnknown(t *testing.T) {
	name := ResolveMachineName(Config{}, func() (string, error) {
		return "", assert.AnError
	})

	require.Equal(t, "unknown", name)
}
