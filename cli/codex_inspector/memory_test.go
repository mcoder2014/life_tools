package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadMemorySearchAndDetail(t *testing.T) {
	codexHome := t.TempDir()
	memDir := filepath.Join(codexHome, "memories")
	require.NoError(t, os.MkdirAll(filepath.Join(memDir, "rollout_summaries"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("# Memory Index\n\ncodex inspector note\napi_key=should-hide\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "memory_summary.md"), []byte("summary only\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "rollout_summaries", "demo.md"), []byte("thread_id: demo\n# Demo Rollout\nkeyword appears here\n"), 0644))

	store := NewStoreWithCache(codexHome, "")
	response := store.LoadMemory("keyword", 10)
	require.Len(t, response.Files, 1)
	require.Equal(t, "rollout_summaries/demo.md", response.Files[0].Path)

	detail, ok := store.LoadMemoryDetail("MEMORY.md")
	require.True(t, ok)
	require.Contains(t, detail.Content, "Memory Index")
	require.NotContains(t, detail.Content, "should-hide")
	require.Contains(t, detail.Content, "[REDACTED]")
}

func TestLoadMemoryRejectsTraversal(t *testing.T) {
	store := NewStoreWithCache(t.TempDir(), "")
	_, ok := store.LoadMemoryDetail("../auth.json")
	require.False(t, ok)
}
