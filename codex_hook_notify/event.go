package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const maxSummaryLen = 900

type HookEvent struct {
	HookEventName        string          `json:"hook_event_name"`
	SessionID            string          `json:"session_id"`
	TurnID               string          `json:"turn_id"`
	CWD                  string          `json:"cwd"`
	Model                string          `json:"model"`
	ToolName             string          `json:"tool_name"`
	LastAssistantMessage string          `json:"last_assistant_message"`
	PermissionRequest    json.RawMessage `json:"permission_request"`
	Raw                  map[string]any  `json:"-"`
}

func ParseHookEvent(content []byte) (HookEvent, error) {
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil {
		return HookEvent{}, err
	}

	var event HookEvent
	if err := json.Unmarshal(content, &event); err != nil {
		return HookEvent{}, err
	}
	event.Raw = raw
	event.HookEventName = firstNonEmpty(event.HookEventName, stringField(raw, "event"))
	event.SessionID = firstNonEmpty(event.SessionID, stringField(raw, "thread_id"))
	event.CWD = firstNonEmpty(event.CWD, stringField(raw, "working_directory"))
	event.ToolName = firstNonEmpty(event.ToolName, stringField(raw, "tool"))
	return event, nil
}

func BuildMessage(event HookEvent) string {
	var buf bytes.Buffer
	writeLine(&buf, "Codex Hook Reminder")
	writeLine(&buf, "")
	writeLine(&buf, "Reason: "+valueOrUnknown(event.HookEventName))
	writeLine(&buf, "Session: "+valueOrUnknown(event.SessionID))
	writeOptionalLine(&buf, "Turn: ", event.TurnID)
	writeOptionalLine(&buf, "CWD: ", event.CWD)
	writeOptionalLine(&buf, "Model: ", event.Model)
	writeOptionalLine(&buf, "Tool: ", event.ToolName)
	writeLine(&buf, "Summary:")
	writeLine(&buf, summary(event))
	return buf.String()
}

func summary(event HookEvent) string {
	if event.HookEventName == "PermissionRequest" {
		return truncate(permissionSummary(event), maxSummaryLen)
	}
	if event.LastAssistantMessage != "" {
		return truncate(event.LastAssistantMessage, maxSummaryLen)
	}
	return "No summary available."
}

func permissionSummary(event HookEvent) string {
	values := flattenJSON(event.PermissionRequest)
	if len(values) == 0 {
		values = event.Raw
	}

	parts := []string{
		stringFromKeys(values, "reason", "message", "description", "justification"),
		stringFromKeys(values, "command", "cmd"),
	}
	var lines []string
	for _, part := range parts {
		if part != "" {
			lines = append(lines, part)
		}
	}
	if len(lines) == 0 {
		return "Permission requested."
	}
	return strings.Join(lines, "\n")
}

func flattenJSON(content json.RawMessage) map[string]any {
	if len(content) == 0 {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal(content, &values); err != nil {
		return nil
	}
	return values
}

func stringFromKeys(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value := stringField(values, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func stringField(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func truncate(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxBytes {
		return value
	}

	var b strings.Builder
	for _, r := range value {
		if b.Len()+len(string(r)) > maxBytes {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "\n...(truncated)"
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func writeOptionalLine(buf *bytes.Buffer, prefix string, value string) {
	if value != "" {
		writeLine(buf, prefix+value)
	}
}

func writeLine(buf *bytes.Buffer, line string) {
	buf.WriteString(line)
	buf.WriteByte('\n')
}
