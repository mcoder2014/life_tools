package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseRolloutFileToleratesBadJSONAndRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-2026-07-03T10-00-00-demo.jsonl")
	content := `{"type":"session_meta","timestamp":"2026-07-03T10:00:00Z","payload":{"id":"demo-session","cwd":"/tmp/demo","model_provider":"openai","model":"gpt-test"}}
{"type":"response_item","timestamp":"2026-07-03T10:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"please inspect token=abc123456789"}]}}
{"type":"response_item","timestamp":"2026-07-03T10:00:02Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}
{"type":"event_msg","timestamp":"2026-07-03T10:00:02Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":80,"reasoning_output_tokens":20,"total_tokens":1080},"last_token_usage":{"input_tokens":300,"cached_input_tokens":100,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":330},"model_context_window":200000},"rate_limits":{"primary":{"used_percent":12.5,"window_minutes":300},"secondary":{"used_percent":2.5,"window_minutes":10080}}}}
{"type":"response_item","timestamp":"2026-07-03T10:00:03Z","payload":{"type":"function_call","name":"shell","call_id":"call-1","arguments":"{\"password\":\"secret-value\"}"}}
{"type":"response_item","timestamp":"2026-07-03T10:00:04Z","payload":{"type":"function_call_output","call_id":"call-1","output":"ok cookie=real-cookie-value"}}
not json
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	detail, err := ParseRolloutFile(path, true)
	require.NoError(t, err)
	require.Equal(t, "demo-session", detail.Summary.ID)
	require.Equal(t, "/tmp/demo", detail.Summary.Cwd)
	require.Equal(t, 1, detail.Summary.UserMessages)
	require.Equal(t, 1, detail.Summary.AssistantMessages)
	require.Equal(t, 1, detail.Summary.ToolCalls)
	require.Equal(t, 1, detail.Summary.ToolResults)
	require.Equal(t, int64(1080), detail.Summary.TokenStats.Total.TotalTokens)
	require.Equal(t, int64(400), detail.Summary.TokenStats.Total.CachedInputTokens)
	require.Equal(t, int64(330), detail.Summary.TokenStats.Last.TotalTokens)
	require.Equal(t, int64(200000), detail.Summary.TokenStats.ModelContextWindow)
	require.Equal(t, 12.5, detail.Summary.TokenStats.PrimaryRateLimit.UsedPercent)
	require.NotEmpty(t, detail.Warnings)
	require.Len(t, detail.RawLines, 7)

	combined := ""
	for _, event := range detail.Events {
		combined += event.Text + "\n"
	}
	for _, raw := range detail.RawLines {
		combined += raw.Text + "\n"
	}
	require.NotContains(t, combined, "abc123456789")
	require.NotContains(t, combined, "secret-value")
	require.NotContains(t, combined, "real-cookie-value")
	require.Contains(t, combined, "[REDACTED]")
}

func TestStoreLoadsSessionIndexAndRolloutSummaries(t *testing.T) {
	codexHome := t.TempDir()
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "07", "03")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "session_index.jsonl"), []byte(`{"id":"demo-session","thread_name":"Demo Thread","updated_at":"2026-07-03T11:00:00Z"}`+"\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "rollout-2026-07-03T10-00-00-demo.jsonl"), []byte(`{"type":"session_meta","timestamp":"2026-07-03T10:00:00Z","payload":{"id":"demo-session","cwd":"/tmp/demo"}}
{"type":"response_item","timestamp":"2026-07-03T10:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
`), 0644))

	store := NewStoreWithCache(codexHome, filepath.Join(t.TempDir(), "cache.sqlite"))
	defer store.Close()
	sessions, warnings := store.LoadSessions(parseFilter("2026-07-01", "2026-07-31", "", 0))
	require.Empty(t, warnings)
	require.Len(t, sessions, 1)
	require.Equal(t, "Demo Thread", sessions[0].Title)
	require.Equal(t, "2026-07-03T11:00:00Z", sessions[0].UpdatedAt)

	detail, ok, warnings := store.LoadSessionDetail("demo-session")
	require.True(t, ok)
	require.Empty(t, warnings)
	require.Equal(t, "Demo Thread", detail.Summary.Title)
	require.Len(t, detail.Events, 2)
}

func TestHistoricalSessionSummaryIsCachedInSQLite(t *testing.T) {
	codexHome := t.TempDir()
	sessionDir := filepath.Join(codexHome, "sessions", "2000", "01", "01")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "session_index.jsonl"), nil, 0644))
	path := filepath.Join(sessionDir, "rollout-2000-01-01T10-00-00-demo.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(`{"type":"session_meta","timestamp":"2000-01-01T10:00:00Z","payload":{"id":"cached-session","cwd":"/tmp/demo","model":"gpt-test"}}
{"type":"response_item","timestamp":"2000-01-01T10:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
`), 0644))
	oldTime := time.Date(2000, 1, 1, 10, 0, 0, 0, time.Local)
	require.NoError(t, os.Chtimes(path, oldTime, oldTime))

	cachePath := filepath.Join(t.TempDir(), "summary.sqlite")
	store := NewStoreWithCache(codexHome, cachePath)
	sessions, warnings := store.LoadSessions(parseFilter("2000-01-01", "2000-01-02", "", 0))
	store.Close()

	require.Empty(t, warnings)
	require.Len(t, sessions, 1)
	require.Equal(t, 1, sessions[0].UserMessages)

	cache, err := openSummaryDiskCache(cachePath)
	require.NoError(t, err)
	defer cache.Close()
	meta, ok := statFile(path)
	require.True(t, ok)
	summary, ok, err := cache.Lookup(path, meta)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "cached-session", summary.ID)
	require.Equal(t, 1, summary.UserMessages)
	require.Equal(t, "gpt-test", summary.Model)
}
