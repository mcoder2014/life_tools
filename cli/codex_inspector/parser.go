package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type rolloutLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type responseItemHeader struct {
	Type string `json:"type"`
	Role string `json:"role"`
}

func ParseRolloutFile(path string, includeEvents bool) (SessionDetail, error) {
	file, err := os.Open(path)
	if err != nil {
		return SessionDetail{}, err
	}
	defer file.Close()

	detail := SessionDetail{
		Summary: SessionSummary{
			Path:  path,
			Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		},
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 8*1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		detail.Summary.EventCount++

		if includeEvents {
			detail.RawLines = append(detail.RawLines, RawLine{Line: lineNo, Text: truncateText(sanitizedRawJSON(line), 60000)})
		}

		var record rolloutLine
		if err := json.Unmarshal(line, &record); err != nil {
			warning := fmt.Sprintf("line %d invalid JSON: %v", lineNo, err)
			detail.Warnings = append(detail.Warnings, warning)
			if includeEvents {
				detail.Events = append(detail.Events, DisplayEvent{Line: lineNo, Kind: "system_event", Label: "Bad JSON", Text: warning})
			}
			continue
		}
		if !includeEvents {
			updateSummaryFast(&detail.Summary, record)
			continue
		}
		event := parseDisplayEvent(record, lineNo)
		updateSummary(&detail.Summary, record, event)
		detail.Events = append(detail.Events, event)
	}
	if err := scanner.Err(); err != nil {
		detail.Warnings = append(detail.Warnings, fmt.Sprintf("scan %s: %v", path, err))
	}
	if detail.Summary.ID == "" {
		detail.Summary.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if detail.Summary.Title == "" {
		detail.Summary.Title = detail.Summary.ID
	}
	return detail, nil
}

func updateSummaryFast(summary *SessionSummary, record rolloutLine) {
	if summary.StartedAt == "" && record.Timestamp != "" {
		summary.StartedAt = record.Timestamp
	}
	if record.Timestamp != "" {
		summary.UpdatedAt = record.Timestamp
	}

	if record.Type == "session_meta" || record.Type == "turn_context" {
		payload := decodePayload(record.Payload)
		if record.Type == "session_meta" {
			if id := stringField(payload, "id"); id != "" {
				summary.ID = id
			}
			if provider := stringField(payload, "model_provider"); provider != "" {
				summary.ModelProvider = sanitizeText(provider)
			}
		}
		if cwd := stringField(payload, "cwd"); cwd != "" && summary.Cwd == "" {
			summary.Cwd = sanitizeText(cwd)
		}
		if model := stringField(payload, "model"); model != "" && summary.Model == "" {
			summary.Model = sanitizeText(model)
		}
		summary.SystemEvents++
		return
	}

	if updateTokenStats(summary, record) {
		summary.SystemEvents++
		return
	}

	if record.Type != "response_item" {
		summary.SystemEvents++
		return
	}

	var header responseItemHeader
	if err := json.Unmarshal(record.Payload, &header); err != nil {
		summary.SystemEvents++
		return
	}
	switch header.Type {
	case "message":
		if header.Role == "user" {
			summary.UserMessages++
		}
		if header.Role == "assistant" {
			summary.AssistantMessages++
		}
		if summary.Preview == "" {
			payload := decodePayload(record.Payload)
			summary.Preview = truncateText(extractContentText(payload["content"]), 220)
		}
	case "function_call", "custom_tool_call", "tool_search_call", "web_search_call":
		summary.ToolCalls++
	case "function_call_output", "custom_tool_call_output", "tool_search_output":
		summary.ToolResults++
	default:
		summary.SystemEvents++
	}
}

func parseDisplayEvent(record rolloutLine, lineNo int) DisplayEvent {
	event := DisplayEvent{
		Line:      lineNo,
		Timestamp: record.Timestamp,
		Kind:      "system_event",
		Label:     record.Type,
	}

	payload := decodePayload(record.Payload)
	switch record.Type {
	case "session_meta":
		event.Label = "Session metadata"
		event.Text = joinNonEmpty([]string{
			"id: " + stringField(payload, "id"),
			"cwd: " + stringField(payload, "cwd"),
			"model: " + stringField(payload, "model"),
			"provider: " + stringField(payload, "model_provider"),
		}, "\n")
	case "turn_context":
		event.Label = "Turn context"
		event.Text = joinNonEmpty([]string{
			"cwd: " + stringField(payload, "cwd"),
			"model: " + stringField(payload, "model"),
			"approval: " + stringField(payload, "approval_policy"),
		}, "\n")
	case "event_msg":
		event.Label = "System event"
		if stringField(payload, "type") == "token_count" {
			event.Label = "Token usage"
			event.Text = tokenEventText(payload)
		} else {
			event.Text = eventMessageText(payload)
		}
	case "compacted":
		event.Label = "Compacted context"
		event.Text = stringField(payload, "message")
	case "response_item":
		return responseItemEvent(payload, record.Timestamp, lineNo)
	default:
		event.Label = "Unknown event: " + record.Type
		event.Text = truncateText(sanitizeText(string(record.Payload)), 1200)
	}
	event.Text = truncateText(sanitizeText(event.Text), 3000)
	return event
}

func responseItemEvent(payload map[string]any, timestamp string, lineNo int) DisplayEvent {
	payloadType := stringField(payload, "type")
	event := DisplayEvent{
		Line:      lineNo,
		Timestamp: timestamp,
		Kind:      "system_event",
		Label:     payloadType,
	}

	switch payloadType {
	case "message":
		role := stringField(payload, "role")
		event.Kind = "message"
		event.Role = role
		if role == "user" {
			event.Label = "User"
		} else if role == "assistant" {
			event.Label = "Codex"
		} else {
			event.Label = "Message"
		}
		event.Text = truncateText(extractContentText(payload["content"]), 8000)
	case "function_call", "custom_tool_call", "tool_search_call", "web_search_call":
		event.Kind = "tool_call"
		event.Label = firstNonEmpty(stringField(payload, "name"), payloadType)
		event.Text = truncateText(toolCallText(payload), 4000)
	case "function_call_output", "custom_tool_call_output", "tool_search_output":
		event.Kind = "tool_result"
		event.Label = firstNonEmpty(stringField(payload, "call_id"), payloadType)
		event.Text = truncateText(toolOutputText(payload), 6000)
	case "reasoning":
		event.Kind = "reasoning"
		event.Label = "Reasoning"
		event.Text = "encrypted or summarized reasoning item"
	default:
		event.Label = "Response item: " + payloadType
		event.Text = truncateText(fmt.Sprint(sanitizeJSONValue(payload)), 2000)
	}
	event.Text = sanitizeText(event.Text)
	return event
}

func updateSummary(summary *SessionSummary, record rolloutLine, event DisplayEvent) {
	if summary.StartedAt == "" && record.Timestamp != "" {
		summary.StartedAt = record.Timestamp
	}
	if record.Timestamp != "" {
		summary.UpdatedAt = record.Timestamp
	}

	payload := decodePayload(record.Payload)
	if record.Type == "session_meta" {
		if id := stringField(payload, "id"); id != "" {
			summary.ID = id
		}
		if cwd := stringField(payload, "cwd"); cwd != "" {
			summary.Cwd = sanitizeText(cwd)
		}
		if model := stringField(payload, "model"); model != "" {
			summary.Model = sanitizeText(model)
		}
		if provider := stringField(payload, "model_provider"); provider != "" {
			summary.ModelProvider = sanitizeText(provider)
		}
	}
	if record.Type == "turn_context" {
		if cwd := stringField(payload, "cwd"); cwd != "" && summary.Cwd == "" {
			summary.Cwd = sanitizeText(cwd)
		}
		if model := stringField(payload, "model"); model != "" && summary.Model == "" {
			summary.Model = sanitizeText(model)
		}
	}
	updateTokenStats(summary, record)

	switch event.Kind {
	case "message":
		if event.Role == "user" {
			summary.UserMessages++
		}
		if event.Role == "assistant" {
			summary.AssistantMessages++
		}
		if summary.Preview == "" && event.Text != "" {
			summary.Preview = truncateText(event.Text, 220)
		}
	case "tool_call":
		summary.ToolCalls++
	case "tool_result":
		summary.ToolResults++
	default:
		summary.SystemEvents++
	}
}

func updateTokenStats(summary *SessionSummary, record rolloutLine) bool {
	if record.Type != "event_msg" {
		return false
	}
	payload := decodePayload(record.Payload)
	if stringField(payload, "type") != "token_count" {
		return false
	}
	applyTokenPayload(summary, payload)
	return true
}

func applyTokenPayload(summary *SessionSummary, payload map[string]any) {
	info := mapField(payload, "info")
	rateLimits := mapField(payload, "rate_limits")
	summary.TokenStats.Total = tokenUsageFromMap(mapField(info, "total_token_usage"))
	summary.TokenStats.Last = tokenUsageFromMap(mapField(info, "last_token_usage"))
	summary.TokenStats.ModelContextWindow = int64Field(info, "model_context_window")
	summary.TokenStats.PrimaryRateLimit = rateLimitFromMap(mapField(rateLimits, "primary"))
	summary.TokenStats.SecondaryRateLimit = rateLimitFromMap(mapField(rateLimits, "secondary"))
	summary.TokenStats.TokenEvents++
}

func tokenUsageFromMap(values map[string]any) TokenUsage {
	return TokenUsage{
		InputTokens:           int64Field(values, "input_tokens"),
		CachedInputTokens:     int64Field(values, "cached_input_tokens"),
		OutputTokens:          int64Field(values, "output_tokens"),
		ReasoningOutputTokens: int64Field(values, "reasoning_output_tokens"),
		TotalTokens:           int64Field(values, "total_tokens"),
	}
}

func rateLimitFromMap(values map[string]any) RateLimitUsage {
	return RateLimitUsage{
		UsedPercent:   floatField(values, "used_percent"),
		WindowMinutes: int64Field(values, "window_minutes"),
	}
}

func decodePayload(raw json.RawMessage) map[string]any {
	var payload map[string]any
	if len(raw) == 0 {
		return map[string]any{}
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func stringField(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return sanitizeText(text)
	}
	return sanitizeText(fmt.Sprint(value))
}

func extractContentText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return sanitizeText(fmt.Sprint(value))
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		text := stringField(obj, "text")
		if text == "" {
			text = stringField(obj, "input")
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func toolCallText(payload map[string]any) string {
	parts := []string{}
	for _, key := range []string{"name", "status", "execution", "arguments", "input"} {
		value := stringField(payload, key)
		if value != "" {
			parts = append(parts, key+": "+value)
		}
	}
	if len(parts) == 0 {
		return truncateText(fmt.Sprint(sanitizeJSONValue(payload)), 1200)
	}
	return strings.Join(parts, "\n")
}

func toolOutputText(payload map[string]any) string {
	for _, key := range []string{"output", "tools", "status"} {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok && text != "" {
			return text
		}
		content, err := json.Marshal(sanitizeJSONValue(value))
		if err == nil {
			return string(content)
		}
	}
	return truncateText(fmt.Sprint(sanitizeJSONValue(payload)), 1200)
}

func eventMessageText(payload map[string]any) string {
	for _, key := range []string{"message", "type", "turn_id", "collaboration_mode_kind"} {
		value := stringField(payload, key)
		if value != "" {
			return value
		}
	}
	return truncateText(fmt.Sprint(sanitizeJSONValue(payload)), 1200)
}

func tokenEventText(payload map[string]any) string {
	info := mapField(payload, "info")
	total := tokenUsageFromMap(mapField(info, "total_token_usage"))
	last := tokenUsageFromMap(mapField(info, "last_token_usage"))
	primary := rateLimitFromMap(mapField(mapField(payload, "rate_limits"), "primary"))
	secondary := rateLimitFromMap(mapField(mapField(payload, "rate_limits"), "secondary"))
	parts := []string{
		fmt.Sprintf("total: %s tokens (%s input, %s cached input, %s output, %s reasoning)",
			formatInt64(total.TotalTokens), formatInt64(total.InputTokens), formatInt64(total.CachedInputTokens), formatInt64(total.OutputTokens), formatInt64(total.ReasoningOutputTokens)),
		fmt.Sprintf("last: %s tokens (%s input, %s cached input, %s output, %s reasoning)",
			formatInt64(last.TotalTokens), formatInt64(last.InputTokens), formatInt64(last.CachedInputTokens), formatInt64(last.OutputTokens), formatInt64(last.ReasoningOutputTokens)),
	}
	if window := int64Field(info, "model_context_window"); window > 0 {
		parts = append(parts, "context window: "+formatInt64(window))
	}
	if primary.UsedPercent > 0 || primary.WindowMinutes > 0 {
		parts = append(parts, fmt.Sprintf("primary rate limit: %.1f%% / %d min", primary.UsedPercent, primary.WindowMinutes))
	}
	if secondary.UsedPercent > 0 || secondary.WindowMinutes > 0 {
		parts = append(parts, fmt.Sprintf("secondary rate limit: %.1f%% / %d min", secondary.UsedPercent, secondary.WindowMinutes))
	}
	return strings.Join(parts, "\n")
}

func mapField(payload map[string]any, key string) map[string]any {
	value, ok := payload[key]
	if !ok || value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func int64Field(payload map[string]any, key string) int64 {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		n, _ := typed.Int64()
		return n
	default:
		return 0
	}
}

func floatField(payload map[string]any, key string) float64 {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		n, _ := typed.Float64()
		return n
	default:
		return 0
	}
}

func formatInt64(value int64) string {
	return fmt.Sprintf("%d", value)
}

func joinNonEmpty(values []string, sep string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasSuffix(value, ":") || strings.HasSuffix(value, ": ") {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, sep)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
