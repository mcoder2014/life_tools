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

	msgs := BuildMessages(event)
	require.Len(t, msgs, 1)
	msg := msgs[0]

	require.Contains(t, msg, "Reason: Stop")
	require.Contains(t, msg, "Session: session-123")
	require.Contains(t, msg, "CWD: /tmp/project")
	require.Contains(t, msg, "Model: gpt-test")
	require.Contains(t, msg, "Summary:")
	require.NotContains(t, msg, "(truncated)")
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

	msgs := BuildMessages(event)
	require.Len(t, msgs, 1)
	msg := msgs[0]

	require.Contains(t, msg, "Reason: PermissionRequest")
	require.Contains(t, msg, "Session: session-456")
	require.Contains(t, msg, "Tool: functions.exec_command")
	require.Contains(t, msg, "需要执行 git push")
	require.Contains(t, msg, "git push origin")
}

func TestBuildMessagesSplitsLongStopSummary(t *testing.T) {
	event := HookEvent{
		HookEventName:        "Stop",
		SessionID:            "session-789",
		TurnID:               "turn-789",
		LastAssistantMessage: strings.Repeat("long output line\n", 600),
	}

	msgs := BuildMessages(event)

	require.Greater(t, len(msgs), 1)
	require.Contains(t, msgs[0], "Codex Hook Reminder [1/")
	require.Contains(t, msgs[len(msgs)-1], "Codex Hook Reminder [")
	require.NotContains(t, strings.Join(msgs, "\n"), "(truncated)")
}

func TestBuildMessagesKeepsNonStopEventsShort(t *testing.T) {
	event := HookEvent{
		HookEventName:        "PostToolUse",
		SessionID:            "session-short",
		LastAssistantMessage: strings.Repeat("long output line\\n", 600),
	}

	msgs := BuildMessages(event)

	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0], "...(truncated)")
}

func TestBuildMessagesWithMachineIncludesMachineForStop(t *testing.T) {
	event := HookEvent{
		HookEventName:        "Stop",
		SessionID:            "session-machine",
		LastAssistantMessage: "summary",
	}

	msgs := BuildMessagesWithMachine(event, "home-nas")

	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0], "Reason: Stop")
	require.Contains(t, msgs[0], "Machine: home-nas")
	require.Contains(t, msgs[0], "Session: session-machine")
}

func TestBuildMessagesWithMachineIncludesMachineForPermissionRequest(t *testing.T) {
	event := HookEvent{
		HookEventName: "PermissionRequest",
		SessionID:     "session-machine",
	}

	msgs := BuildMessagesWithMachine(event, "home-nas")

	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0], "Reason: PermissionRequest")
	require.Contains(t, msgs[0], "Machine: home-nas")
}

func TestBuildMessagesWithMachineIncludesMachineOnEveryPart(t *testing.T) {
	event := HookEvent{
		HookEventName:        "Stop",
		SessionID:            "session-parts-machine",
		LastAssistantMessage: strings.Repeat("long output line\n", 600),
	}

	msgs := BuildMessagesWithMachine(event, "home-nas")

	require.Greater(t, len(msgs), 1)
	for _, msg := range msgs {
		require.Contains(t, msg, "Machine: home-nas")
	}
}

func TestSafeLogFileName(t *testing.T) {
	name := LogFileName("2026-06-21", "abc/../def 123")

	require.Equal(t, "2026-06-21_abc____def_123.log", name)
}
