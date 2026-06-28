package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Entries []ConfigEntry `json:"entries"`
}

type ConfigEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type ShareEntry struct {
	ID    int
	Name  string
	Root  string
	IsDir bool
}

func LoadConfig(content []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func BuildEntries(configEntries []ConfigEntry) ([]ShareEntry, error) {
	if len(configEntries) == 0 {
		return nil, fmt.Errorf("at least one share path is required")
	}

	entries := make([]ShareEntry, 0, len(configEntries))
	for i, configEntry := range configEntries {
		if configEntry.Path == "" {
			return nil, fmt.Errorf("entry %d path is required", i)
		}

		info, err := os.Stat(configEntry.Path)
		if err != nil {
			return nil, fmt.Errorf("stat share path %q: %w", configEntry.Path, err)
		}

		name := configEntry.Name
		if name == "" {
			name = filepath.Base(configEntry.Path)
		}
		if name == "." || name == string(filepath.Separator) {
			name = configEntry.Path
		}

		entries = append(entries, ShareEntry{
			ID:    i,
			Name:  name,
			Root:  configEntry.Path,
			IsDir: info.IsDir(),
		})
	}
	return entries, nil
}

func EntriesFromPaths(paths []string) []ConfigEntry {
	entries := make([]ConfigEntry, 0, len(paths))
	for _, path := range paths {
		entries = append(entries, ConfigEntry{Path: path})
	}
	return entries
}
