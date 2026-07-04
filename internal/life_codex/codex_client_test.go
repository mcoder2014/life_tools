package life_codex

import (
	"encoding/json"
	"testing"
)

func TestExtractThreadID(t *testing.T) {
	id, err := extractThreadID(json.RawMessage(`{"thread":{"id":"thread-1"}}`))
	if err != nil {
		t.Fatalf("extract thread id: %v", err)
	}
	if id != "thread-1" {
		t.Fatalf("id = %q", id)
	}
}

func TestCodexNotificationEvent(t *testing.T) {
	event := codexNotificationEvent("session", "machine", rpcMessage{
		Method: "item/completed",
		Params: json.RawMessage(`{"item":{"type":"agentMessage","text":"done"}}`),
	})
	if event == nil || event.Type != EventAssistant || event.Text != "done" {
		t.Fatalf("unexpected event: %+v", event)
	}
}
