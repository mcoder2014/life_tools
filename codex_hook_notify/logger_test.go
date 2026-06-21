package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultLogDirByOS(t *testing.T) {
	require.Equal(t, "/var/log/codex_hook_notify", defaultLogDir("linux", "/home/cq"))
	require.Equal(t, filepath.Join("/Users/cq", "Library", "Logs", "codex_hook_notify"), defaultLogDir("darwin", "/Users/cq"))
	require.Equal(t, filepath.Join("/home/cq", ".codex_hook_notify", "logs"), defaultLogDir("freebsd", "/home/cq"))
}

func TestDefaultLogDirFallbackWithoutHome(t *testing.T) {
	require.Equal(t, filepath.Join(".", ".codex_hook_notify", "logs"), defaultLogDir("darwin", ""))
}
