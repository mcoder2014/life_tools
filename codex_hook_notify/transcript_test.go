package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStopSummaryReadsVisibleMessagesFromTranscript(t *testing.T) {
	codexHome := t.TempDir()
	sessionID := "session-123"
	turnID := "turn-123"
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "21")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))
	sessionFile := filepath.Join(sessionDir, "rollout-2026-06-21T12-00-00-"+sessionID+".jsonl")
	require.NoError(t, os.WriteFile(sessionFile, []byte(`{"type":"session_meta","payload":{"id":"session-123"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"visible commentary"}],"phase":"commentary","metadata":{"turn_id":"turn-123"}}}
{"type":"response_item","payload":{"type":"function_call_output","output":"secret tool output","metadata":{"turn_id":"turn-123"}}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"visible final"}],"phase":"final_answer","metadata":{"turn_id":"turn-123"}}}
`), 0644))

	summary := stopSummary(HookEvent{
		HookEventName:        "Stop",
		SessionID:            sessionID,
		TurnID:               turnID,
		LastAssistantMessage: "fallback summary",
	}, codexHome)

	require.Equal(t, "visible commentary\n\nvisible final", summary)
	require.NotContains(t, summary, "secret tool output")
	require.NotContains(t, summary, "fallback summary")
}

func TestStopSummaryFindsTranscriptBySessionMeta(t *testing.T) {
	codexHome := t.TempDir()
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "21")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))
	sessionFile := filepath.Join(sessionDir, "rollout-without-session-id.jsonl")
	require.NoError(t, os.WriteFile(sessionFile, []byte(`{"type":"session_meta","payload":{"id":"session-meta"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"meta matched final"}],"phase":"final_answer","metadata":{"turn_id":"turn-meta"}}}
`), 0644))

	summary := stopSummary(HookEvent{
		HookEventName:        "Stop",
		SessionID:            "session-meta",
		TurnID:               "turn-meta",
		LastAssistantMessage: "fallback summary",
	}, codexHome)

	require.Equal(t, "meta matched final", summary)
}

func TestStopSummaryFallsBackWhenTranscriptIsMissing(t *testing.T) {
	summary := stopSummary(HookEvent{
		HookEventName:        "Stop",
		SessionID:            "missing-session",
		TurnID:               "missing-turn",
		LastAssistantMessage: "fallback summary",
	}, t.TempDir())

	require.Equal(t, "fallback summary", summary)
}
