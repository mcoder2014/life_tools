package main

import (
	"testing"

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
