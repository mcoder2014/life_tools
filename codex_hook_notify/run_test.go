package main

import (
	"bytes"
	"context"
	"errors"
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
	require.Contains(t, logs, "event Stop sent to 1 feishu webhook(s)")
}
