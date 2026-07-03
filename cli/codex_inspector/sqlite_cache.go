package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const summaryCacheVersion = 1

type summaryDiskCache struct {
	path string
	db   *sql.DB
}

type fileMeta struct {
	Size       int64
	ModifiedAt int64
	ModTime    time.Time
}

func defaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "life_tools", "codex_inspector", "session_summary_cache.sqlite")
}

func openSummaryDiskCache(path string) (*summaryDiskCache, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
		_ = file.Close()
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0600)
	db.SetMaxOpenConns(1)
	cache := &summaryDiskCache{path: path, db: db}
	if err := cache.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return cache, nil
}

func (c *summaryDiskCache) init() error {
	_, err := c.db.Exec(`
PRAGMA busy_timeout = 5000;
CREATE TABLE IF NOT EXISTS session_summaries (
	path TEXT PRIMARY KEY,
	size INTEGER NOT NULL,
	modified_at_ns INTEGER NOT NULL,
	session_id TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	summary_json TEXT NOT NULL,
	cached_at TEXT NOT NULL,
	version INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_summaries_session_id ON session_summaries(session_id);
`)
	return err
}

func (c *summaryDiskCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *summaryDiskCache) Lookup(path string, meta fileMeta) (SessionSummary, bool, error) {
	if c == nil || c.db == nil {
		return SessionSummary{}, false, nil
	}
	var raw string
	err := c.db.QueryRow(
		`SELECT summary_json FROM session_summaries
		 WHERE path = ? AND size = ? AND modified_at_ns = ? AND version = ?`,
		path, meta.Size, meta.ModifiedAt, summaryCacheVersion,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return SessionSummary{}, false, nil
	}
	if err != nil {
		return SessionSummary{}, false, err
	}
	var summary SessionSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return SessionSummary{}, false, err
	}
	summary.Enriched = true
	return summary, true, nil
}

func (c *summaryDiskCache) Upsert(path string, meta fileMeta, summary SessionSummary) error {
	if c == nil || c.db == nil || path == "" {
		return nil
	}
	summary.Enriched = true
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`INSERT INTO session_summaries
		 (path, size, modified_at_ns, session_id, updated_at, summary_json, cached_at, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   size = excluded.size,
		   modified_at_ns = excluded.modified_at_ns,
		   session_id = excluded.session_id,
		   updated_at = excluded.updated_at,
		   summary_json = excluded.summary_json,
		   cached_at = excluded.cached_at,
		   version = excluded.version`,
		path,
		meta.Size,
		meta.ModifiedAt,
		summary.ID,
		firstNonEmpty(summary.UpdatedAt, summary.StartedAt),
		string(raw),
		time.Now().Format(time.RFC3339),
		summaryCacheVersion,
	)
	return err
}

func statFile(path string) (fileMeta, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return fileMeta{}, false
	}
	return fileMeta{Size: info.Size(), ModifiedAt: info.ModTime().UnixNano(), ModTime: info.ModTime()}, true
}

func cacheableFile(meta fileMeta, now time.Time) bool {
	return startOfDay(meta.ModTime).Before(startOfDay(now))
}

func cacheableSummary(summary SessionSummary, now time.Time) bool {
	value := firstNonEmpty(summary.UpdatedAt, summary.StartedAt)
	t, ok := parseTime(value)
	return ok && startOfDay(t).Before(startOfDay(now))
}
