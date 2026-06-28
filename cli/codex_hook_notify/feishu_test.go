package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeishuSenderSendsTextMessage(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := FeishuSender{Client: server.Client()}.Send(context.Background(), server.URL, "hello")

	require.NoError(t, err)
	require.Equal(t, "text", payload["msg_type"])
	content, ok := payload["content"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "hello", content["text"])
}
