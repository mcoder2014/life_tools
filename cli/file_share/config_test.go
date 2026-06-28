package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigParsesEntries(t *testing.T) {
	config, err := LoadConfig([]byte(`{
		"entries": [
			{"path": "docs", "name": "Documents"},
			{"path": "README.MD"}
		]
	}`))

	require.NoError(t, err)
	require.Len(t, config.Entries, 2)
	require.Equal(t, "docs", config.Entries[0].Path)
	require.Equal(t, "Documents", config.Entries[0].Name)
	require.Equal(t, "README.MD", config.Entries[1].Path)
	require.Empty(t, config.Entries[1].Name)
}

func TestBuildEntriesRejectsEmptyPath(t *testing.T) {
	_, err := BuildEntries([]ConfigEntry{{Name: "missing path"}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "path is required")
}

func TestBuildEntriesUsesBasenameWhenNameIsEmpty(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "share.txt")
	writeTestFile(t, file, "hello")

	entries, err := BuildEntries([]ConfigEntry{{Path: file}})

	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "share.txt", entries[0].Name)
	require.Equal(t, file, entries[0].Root)
	require.False(t, entries[0].IsDir)
}

func TestBuildEntriesKeepsDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	writeTestFile(t, first, "first")
	writeTestFile(t, second, "second")

	entries, err := BuildEntries([]ConfigEntry{
		{Path: first, Name: "same"},
		{Path: second, Name: "same"},
	})

	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "same", entries[0].Name)
	require.Equal(t, "same", entries[1].Name)
}

func TestBuildEntriesFailsWhenPathDoesNotExist(t *testing.T) {
	_, err := BuildEntries([]ConfigEntry{{Path: filepath.Join(t.TempDir(), "missing")}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "stat share path")
}
