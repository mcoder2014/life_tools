package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

var sensitiveKeyPattern = regexp.MustCompile(`(?i)(auth|authorization|bearer|cookie|credential|jwt|passwd|password|secret|session[_-]?token|token|api[_-]?key|access[_-]?key)`)
var assignmentSecretPattern = regexp.MustCompile(`(?i)(auth|authorization|cookie|jwt|passwd|password|secret|session[_-]?token|token|api[_-]?key|access[_-]?key)([A-Za-z0-9_.-]{0,30})(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
var jsonStringSecretPattern = regexp.MustCompile(`(?i)(\\?"(?:auth|authorization|cookie|credential|jwt|passwd|password|secret|session[_-]?token|token|api[_-]?key|access[_-]?key)"\\?\s*:\s*)\\?"[^"\\]*(?:\\.[^"\\]*)*\\?"`)
var bearerPattern = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`)

func sanitizeText(value string) string {
	if value == "" {
		return ""
	}
	value = assignmentSecretPattern.ReplaceAllString(value, "$1$2$3[REDACTED]")
	value = jsonStringSecretPattern.ReplaceAllString(value, `${1}"[REDACTED]"`)
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	return value
}

func sanitizeJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if sensitiveKeyPattern.MatchString(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = sanitizeJSONValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeJSONValue(item))
		}
		return out
	case string:
		return sanitizeText(typed)
	default:
		return value
	}
}

func sanitizedRawJSON(line []byte) string {
	var value any
	if err := json.Unmarshal(line, &value); err != nil {
		return sanitizeText(string(line))
	}
	value = sanitizeJSONValue(value)
	content, err := json.Marshal(value)
	if err != nil {
		return sanitizeText(string(line))
	}
	return string(content)
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return strings.TrimSpace(value[:max]) + "..."
}

func containsFold(value string, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}
