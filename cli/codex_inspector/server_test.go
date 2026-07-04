package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionDetailAPIUsesLightweightOptions(t *testing.T) {
	codexHome := t.TempDir()
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "07", "03")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "session_index.jsonl"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "rollout-2026-07-03T10-00-00-demo.jsonl"), []byte(`{"type":"session_meta","timestamp":"2026-07-03T10:00:00Z","payload":{"id":"demo-session","cwd":"/tmp/demo"}}
{"type":"response_item","timestamp":"2026-07-03T10:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello token=abc123456789"}]}}
{"type":"response_item","timestamp":"2026-07-03T10:00:02Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}
`), 0644))

	store := NewStoreWithCache(codexHome, "")
	defer store.Close()
	server := httptest.NewServer(NewServer(store))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/sessions/demo-session?event_limit=1&raw=0")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var detail SessionDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	require.Len(t, detail.Events, 1)
	require.Empty(t, detail.RawLines)
	require.Equal(t, 0, detail.EventOffset)
	require.Equal(t, 1, detail.EventLimit)
	require.Equal(t, 3, detail.EventTotal)
	require.True(t, detail.HasMore)

	rawResp, err := http.Get(server.URL + "/api/sessions/demo-session/raw?line=2")
	require.NoError(t, err)
	defer rawResp.Body.Close()
	require.Equal(t, http.StatusOK, rawResp.StatusCode)

	var raw RawLineResponse
	require.NoError(t, json.NewDecoder(rawResp.Body).Decode(&raw))
	require.Equal(t, 2, raw.Line.Line)
	require.Contains(t, raw.Line.Text, "[REDACTED]")
	require.NotContains(t, raw.Line.Text, "abc123456789")
}
