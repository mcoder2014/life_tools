package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (s *Store) Diagnostics() DiagnosticsResponse {
	response := DiagnosticsResponse{
		CodexHome: s.CodexHome,
		Sources:   s.Sources(),
		Cache:     s.CacheStatus(),
	}
	dbs := []struct {
		name string
		rel  string
	}{
		{name: "state_5.sqlite", rel: "state_5.sqlite"},
		{name: "goals_1.sqlite", rel: "goals_1.sqlite"},
		{name: "memories_1.sqlite", rel: "memories_1.sqlite"},
	}
	for _, db := range dbs {
		response.Schemas = append(response.Schemas, loadSQLiteSchema(db.name, filepath.Join(s.CodexHome, db.rel)))
	}
	return response
}

func loadSQLiteSchema(name string, path string) SQLiteSchema {
	schema := SQLiteSchema{Name: name, Path: path, ReadMode: "sqlite3 file URI mode=ro"}
	if _, err := os.Stat(path); err != nil {
		schema.Exists = false
		schema.Error = err.Error()
		return schema
	}
	schema.Exists = true
	if _, err := exec.LookPath("sqlite3"); err != nil {
		schema.Error = "sqlite3 command not found"
		return schema
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	uri := fmt.Sprintf("file:%s?mode=ro", path)
	output, err := exec.CommandContext(ctx, "sqlite3", uri, ".schema").CombinedOutput()
	if err != nil {
		schema.Error = sanitizeText(string(output))
		if schema.Error == "" {
			schema.Error = err.Error()
		}
		return schema
	}
	schema.Loaded = true
	schema.Schema = truncateText(sanitizeText(string(output)), 30000)
	return schema
}
