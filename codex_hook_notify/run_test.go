package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunDoesNotBlockWhenSenderFails(t *testing.T) {
	config := Config{
		Routes: []Route{
			{
				Events:                []string{"Stop"},
				FeishuCustomRobotURLs: []string{"https://example.com/webhook"},
			},
		},
	}
	var stderr bytes.Buffer
	sender := SenderFunc(func(ctx context.Context, url string, text string) error {
		require.Equal(t, "https://example.com/webhook", url)
		require.Contains(t, text, "Reason: Stop")
		return errors.New("network down")
	})
	logger := LoggerFunc(func(sessionID string, message string) error {
		require.Equal(t, "session-123", sessionID)
		require.Contains(t, message, "network down")
		return nil
	})

	err := Run(context.Background(), RunOption{
		Config: config,
		Input:  bytes.NewBufferString(`{"hook_event_name":"Stop","session_id":"session-123","cwd":"/tmp/project"}`),
		Stderr: &stderr,
		Sender: sender,
		Logger: logger,
	})

	require.NoError(t, err)
	require.Contains(t, stderr.String(), "codex_hook_notify: send feishu message failed")
}

func TestRunSkipsUnmatchedEventSilently(t *testing.T) {
	called := false
	sender := SenderFunc(func(ctx context.Context, url string, text string) error {
		called = true
		return nil
	})
	logged := false
	var stderr bytes.Buffer

	err := Run(context.Background(), RunOption{
		Config: Config{
			Routes: []Route{{Events: []string{"Stop"}, FeishuCustomRobotURLs: []string{"https://example.com/webhook"}}},
		},
		Input:  bytes.NewBufferString(`{"hook_event_name":"PreToolUse","session_id":"session-123"}`),
		Stderr: &stderr,
		Sender: sender,
		Logger: LoggerFunc(func(sessionID string, message string) error {
			logged = true
			return nil
		}),
	})

	require.NoError(t, err)
	require.False(t, called)
	require.False(t, logged)
	require.Empty(t, stderr.String())
}

func TestRunLogsSuccessfulSend(t *testing.T) {
	config := Config{
		Routes: []Route{
			{
				Events:                []string{"Stop"},
				FeishuCustomRobotURLs: []string{"https://example.com/webhook"},
			},
		},
	}
	var logs []string

	err := Run(context.Background(), RunOption{
		Config: config,
		Input:  bytes.NewBufferString(`{"hook_event_name":"Stop","session_id":"session-success","cwd":"/tmp/project"}`),
		Sender: SenderFunc(func(ctx context.Context, url string, text string) error { return nil }),
		Logger: LoggerFunc(func(sessionID string, message string) error {
			require.Equal(t, "session-success", sessionID)
			logs = append(logs, message)
			return nil
		}),
	})

	require.NoError(t, err)
	require.Contains(t, logs, "event Stop sent to 1 feishu webhook(s), 1 message(s)")
}

func TestRunSendsEveryMessagePart(t *testing.T) {
	config := Config{
		Routes: []Route{
			{
				Events:                []string{"Stop"},
				FeishuCustomRobotURLs: []string{"https://example.com/webhook"},
			},
		},
	}
	var sent []string
	var logs []string

	input := `{"hook_event_name":"Stop","session_id":"session-parts","last_assistant_message":` + strconv.Quote(strings.Repeat("long output line\n", 600)) + `}`
	err := Run(context.Background(), RunOption{
		Config: config,
		Input:  bytes.NewBufferString(input),
		Logger: LoggerFunc(func(sessionID string, message string) error {
			logs = append(logs, message)
			return nil
		}),
		Sender: SenderFunc(func(ctx context.Context, url string, text string) error {
			require.Equal(t, "https://example.com/webhook", url)
			sent = append(sent, text)
			return nil
		}),
	})

	require.NoError(t, err)
	require.Greater(t, len(sent), 1)
	require.Contains(t, sent[0], "Codex Hook Reminder [1/")
	require.Contains(t, logs, fmt.Sprintf("event Stop sent to 1 feishu webhook(s), %d message(s)", len(sent)))
}
