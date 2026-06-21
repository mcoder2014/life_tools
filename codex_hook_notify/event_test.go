package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMessageForStop(t *testing.T) {
	event := HookEvent{
		HookEventName:        "Stop",
		SessionID:            "session-123",
		CWD:                  "/tmp/project",
		Model:                "gpt-test",
		LastAssistantMessage: strings.Repeat("完成了一个很长的回答", 80),
	}

	msg := BuildMessage(event)

	require.Contains(t, msg, "Reason: Stop")
	require.Contains(t, msg, "Session: session-123")
	require.Contains(t, msg, "CWD: /tmp/project")
	require.Contains(t, msg, "Model: gpt-test")
	require.LessOrEqual(t, len(msg), 1600)
	require.Contains(t, msg, "Summary:")
}

func TestBuildMessageForPermissionRequest(t *testing.T) {
	raw := json.RawMessage(`{
		"hook_event_name": "PermissionRequest",
		"session_id": "session-456",
		"cwd": "/tmp/project",
		"tool_name": "functions.exec_command",
		"permission_request": {
			"reason": "需要执行 git push",
			"command": "git push origin feat/cq/codex_hook_notify"
		}
	}`)
	event, err := ParseHookEvent(raw)
	require.NoError(t, err)

	msg := BuildMessage(event)

	require.Contains(t, msg, "Reason: PermissionRequest")
	require.Contains(t, msg, "Session: session-456")
	require.Contains(t, msg, "Tool: functions.exec_command")
	require.Contains(t, msg, "需要执行 git push")
	require.Contains(t, msg, "git push origin")
}

func TestSafeLogFileName(t *testing.T) {
	name := LogFileName("2026-06-21", "abc/../def 123")

	require.Equal(t, "2026-06-21_abc____def_123.log", name)
}
