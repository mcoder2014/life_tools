package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxPermissionSummaryLen = 900
const maxMessageBytes = 3600

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
	return BuildMessages(event)[0]
}

func BuildMessages(event HookEvent) []string {
	content := summary(event)
	parts := splitText(content, messageContentLimit(event))
	if len(parts) == 0 {
		parts = []string{"No summary available."}
	}

	messages := make([]string, 0, len(parts))
	for i, part := range parts {
		messages = append(messages, formatMessage(event, part, i+1, len(parts)))
	}
	return messages
}

func formatMessage(event HookEvent, content string, part int, total int) string {
	var buf bytes.Buffer
	title := "Codex Hook Reminder"
	if total > 1 {
		title = fmt.Sprintf("%s [%d/%d]", title, part, total)
	}
	writeLine(&buf, title)
	writeLine(&buf, "")
	writeLine(&buf, "Reason: "+valueOrUnknown(event.HookEventName))
	writeLine(&buf, "Session: "+valueOrUnknown(event.SessionID))
	writeOptionalLine(&buf, "Turn: ", event.TurnID)
	writeOptionalLine(&buf, "CWD: ", event.CWD)
	writeOptionalLine(&buf, "Model: ", event.Model)
	writeOptionalLine(&buf, "Tool: ", event.ToolName)
	writeLine(&buf, "Summary:")
	writeLine(&buf, content)
	return buf.String()
}

func summary(event HookEvent) string {
	if event.HookEventName == "PermissionRequest" {
		return truncate(permissionSummary(event), maxPermissionSummaryLen)
	}
	if event.HookEventName == "Stop" {
		return stopSummary(event, codexHome())
	}
	if event.LastAssistantMessage != "" {
		return truncate(event.LastAssistantMessage, maxPermissionSummaryLen)
	}
	return "No summary available."
}

func stopSummary(event HookEvent, codexHome string) string {
	if text := transcriptSummary(event, codexHome); text != "" {
		return text
	}
	if event.LastAssistantMessage != "" {
		return strings.TrimSpace(event.LastAssistantMessage)
	}
	return "No summary available."
}

func codexHome() string {
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex")
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

func messageContentLimit(event HookEvent) int {
	header := formatMessage(event, "", 999, 999)
	limit := maxMessageBytes - len(header)
	if limit < 1000 {
		return 1000
	}
	return limit
}

func splitText(value string, maxBytes int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) <= maxBytes {
		return []string{value}
	}

	var parts []string
	for len(value) > 0 {
		if len(value) <= maxBytes {
			parts = append(parts, value)
			break
		}
		cut := splitIndex(value, maxBytes)
		part := strings.TrimSpace(value[:cut])
		if part != "" {
			parts = append(parts, part)
		}
		value = strings.TrimSpace(value[cut:])
	}
	return parts
}

func splitIndex(value string, maxBytes int) int {
	lastNewline := -1
	end := 0
	for i, r := range value {
		next := i + len(string(r))
		if next > maxBytes {
			break
		}
		end = next
		if r == '\n' {
			lastNewline = next
		}
	}
	if lastNewline > 0 {
		return lastNewline
	}
	if end > 0 {
		return end
	}
	return maxBytes
}
