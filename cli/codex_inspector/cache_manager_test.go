package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheMissingCanCreateAndFill(t *testing.T) {
	codexHome, rolloutPath := writeCacheTestRollout(t, "2000", "01", "02", "missing-cache-session")
	cachePath := filepath.Join(t.TempDir(), "summary.sqlite")
	store := NewStoreWithCacheWorkers(codexHome, cachePath, 0)
	defer store.Close()

	require.Equal(t, cacheStatusMissing, store.CacheStatus().Status)
	status := store.StartCacheBuild()
	require.Equal(t, cacheStatusRebuilding, status.Status)
	status = waitCacheJob(t, store)
	require.Equal(t, cacheStatusHealthy, status.Status)
	require.Equal(t, 1, status.Job.Cached)

	cache, err := openExistingSummaryDiskCache(cachePath)
	require.NoError(t, err)
	defer cache.Close()
	meta, ok := statFile(rolloutPath)
	require.True(t, ok)
	summary, ok, err := cache.Lookup(rolloutPath, meta)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "missing-cache-session", summary.ID)
}

func TestCacheCorruptBackupAndRebuild(t *testing.T) {
	codexHome, rolloutPath := writeCacheTestRollout(t, "2000", "01", "03", "corrupt-cache-session")
	cachePath := filepath.Join(t.TempDir(), "summary.sqlite")
	require.NoError(t, os.WriteFile(cachePath, []byte("not a sqlite database"), 0600))
	store := NewStoreWithCacheWorkers(codexHome, cachePath, 0)
	defer store.Close()

	require.Equal(t, cacheStatusCorrupt, store.CacheStatus().Status)
	status := store.StartCacheRebuild()
	require.Equal(t, cacheStatusRebuilding, status.Status)
	status = waitCacheJob(t, store)
	require.Equal(t, cacheStatusHealthy, status.Status)
	require.NotEmpty(t, status.BackupPath)
	require.FileExists(t, status.BackupPath)

	cache, err := openExistingSummaryDiskCache(cachePath)
	require.NoError(t, err)
	defer cache.Close()
	meta, ok := statFile(rolloutPath)
	require.True(t, ok)
	summary, ok, err := cache.Lookup(rolloutPath, meta)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "corrupt-cache-session", summary.ID)
}

func TestCacheDisabledAPIStatus(t *testing.T) {
	store := NewStoreWithCacheWorkers(t.TempDir(), "", 2)
	defer store.Close()
	server := httptest.NewServer(NewServer(store))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/cache/status")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var status CacheStatus
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	require.Equal(t, cacheStatusDisabled, status.Status)
	require.Contains(t, status.Reason, "-no-cache")
}

func TestCacheBuildSkipsTodayRollout(t *testing.T) {
	codexHome, oldPath := writeCacheTestRollout(t, "2000", "01", "04", "old-session")
	today := time.Now()
	todayDir := filepath.Join(codexHome, "sessions", today.Format("2006"), today.Format("01"), today.Format("02"))
	require.NoError(t, os.MkdirAll(todayDir, 0755))
	todayPath := filepath.Join(todayDir, "rollout-"+today.Format("2006-01-02T15-04-05")+"-today.jsonl")
	require.NoError(t, os.WriteFile(todayPath, []byte(`{"type":"session_meta","timestamp":"`+today.Format(time.RFC3339)+`","payload":{"id":"today-session"}}
{"type":"response_item","timestamp":"`+today.Format(time.RFC3339)+`","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"today"}]}}
`), 0644))

	cachePath := filepath.Join(t.TempDir(), "summary.sqlite")
	store := NewStoreWithCacheWorkers(codexHome, cachePath, 0)
	defer store.Close()
	store.StartCacheBuild()
	status := waitCacheJob(t, store)
	require.Equal(t, cacheStatusHealthy, status.Status)
	require.Equal(t, 1, status.Job.Cached)

	cache, err := openExistingSummaryDiskCache(cachePath)
	require.NoError(t, err)
	defer cache.Close()
	oldMeta, ok := statFile(oldPath)
	require.True(t, ok)
	_, ok, err = cache.Lookup(oldPath, oldMeta)
	require.NoError(t, err)
	require.True(t, ok)
	todayMeta, ok := statFile(todayPath)
	require.True(t, ok)
	_, ok, err = cache.Lookup(todayPath, todayMeta)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestRepeatedBuildRequestReturnsRunningJob(t *testing.T) {
	codexHome, _ := writeCacheTestRollout(t, "2000", "01", "05", "repeat-session")
	cachePath := filepath.Join(t.TempDir(), "summary.sqlite")
	store := NewStoreWithCacheWorkers(codexHome, cachePath, 0)
	defer store.Close()
	require.Equal(t, cacheStatusMissing, store.CacheStatus().Status)
	require.Equal(t, cacheStatusRebuilding, store.StartCacheBuild().Status)

	store.cacheManager.mu.Lock()
	firstSeq := store.cacheManager.jobSeq
	store.cacheManager.mu.Unlock()

	second := store.StartCacheBuild()
	require.Equal(t, cacheStatusRebuilding, second.Status)
	store.cacheManager.mu.Lock()
	require.Equal(t, firstSeq, store.cacheManager.jobSeq)
	store.cacheManager.mu.Unlock()
	_ = waitCacheJob(t, store)
}

func writeCacheTestRollout(t *testing.T, year string, month string, day string, id string) (string, string) {
	t.Helper()
	codexHome := t.TempDir()
	sessionDir := filepath.Join(codexHome, "sessions", year, month, day)
	require.NoError(t, os.MkdirAll(sessionDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "session_index.jsonl"), nil, 0644))
	path := filepath.Join(sessionDir, "rollout-"+year+"-"+month+"-"+day+"T10-00-00-"+id+".jsonl")
	require.NoError(t, os.WriteFile(path, []byte(`{"type":"session_meta","timestamp":"`+year+`-`+month+`-`+day+`T10:00:00Z","payload":{"id":"`+id+`","cwd":"/tmp/demo","model":"gpt-test"}}
{"type":"response_item","timestamp":"`+year+`-`+month+`-`+day+`T10:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
`), 0644))
	oldTime := time.Date(2000, 1, 2, 10, 0, 0, 0, time.Local)
	require.NoError(t, os.Chtimes(path, oldTime, oldTime))
	return codexHome, path
}

func waitCacheJob(t *testing.T, store *Store) CacheStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := store.CacheStatus()
		if !status.Job.Running {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("cache job did not finish: %+v", store.CacheStatus())
	return CacheStatus{}
}
