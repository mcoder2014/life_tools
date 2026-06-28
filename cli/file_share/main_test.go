package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadEntriesFromMultiplePaths(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	writeTestFile(t, first, "first")
	writeTestFile(t, second, "second")

	entries, err := loadEntries("", []string{first, second})

	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "first.txt", entries[0].Name)
	require.Equal(t, "second.txt", entries[1].Name)
}

func TestLoadEntriesRejectsConfigAndPaths(t *testing.T) {
	_, err := loadEntries("file_share.json", []string{"."})

	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestLoadEntriesFromConfig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "share.txt")
	writeTestFile(t, file, "share")
	configPath := filepath.Join(dir, "file_share.json")
	writeTestFile(t, configPath, `{"entries":[{"path":"`+file+`","name":"Shared"}]}`)

	entries, err := loadEntries(configPath, nil)

	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Shared", entries[0].Name)
	require.Equal(t, file, entries[0].Root)
}
