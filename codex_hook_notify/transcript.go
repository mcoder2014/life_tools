package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type transcriptLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type sessionMeta struct {
	ID string `json:"id"`
}

type transcriptMessage struct {
	Type     string              `json:"type"`
	Role     string              `json:"role"`
	Phase    string              `json:"phase"`
	Content  []transcriptContent `json:"content"`
	Metadata transcriptMetadata  `json:"metadata"`
}

type transcriptContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type transcriptMetadata struct {
	TurnID string `json:"turn_id"`
}

func transcriptSummary(event HookEvent, codexHome string) string {
	if codexHome == "" || event.SessionID == "" {
		return ""
	}
	path := findTranscript(codexHome, event.SessionID)
	if path == "" {
		return ""
	}
	return readTranscriptSummary(path, event.TurnID)
}

func findTranscript(codexHome string, sessionID string) string {
	sessionsDir := filepath.Join(codexHome, "sessions")
	if match := findTranscriptByName(sessionsDir, sessionID); match != "" {
		return match
	}
	return findTranscriptBySessionMeta(sessionsDir, sessionID)
}

func findTranscriptByName(sessionsDir string, sessionID string) string {
	var match string
	_ = filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if strings.Contains(d.Name(), sessionID) {
			match = path
			return filepath.SkipAll
		}
		return nil
	})
	return match
}

func findTranscriptBySessionMeta(sessionsDir string, sessionID string) string {
	var match string
	_ = filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if transcriptSessionID(path) == sessionID {
			match = path
			return filepath.SkipAll
		}
		return nil
	})
	return match
}

func transcriptSessionID(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		var item transcriptLine
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil || item.Type != "session_meta" {
			continue
		}
		var meta sessionMeta
		if err := json.Unmarshal(item.Payload, &meta); err != nil {
			return ""
		}
		return meta.ID
	}
	return ""
}

func readTranscriptSummary(path string, turnID string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		text := visibleAssistantText(scanner.Bytes(), turnID)
		if text != "" {
			lines = append(lines, text)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func visibleAssistantText(line []byte, turnID string) string {
	var item transcriptLine
	if err := json.Unmarshal(line, &item); err != nil || item.Type != "response_item" {
		return ""
	}

	var msg transcriptMessage
	if err := json.Unmarshal(item.Payload, &msg); err != nil {
		return ""
	}
	if msg.Type != "message" || msg.Role != "assistant" {
		return ""
	}
	if turnID != "" && msg.Metadata.TurnID != turnID {
		return ""
	}
	if msg.Phase != "" && msg.Phase != "commentary" && msg.Phase != "final_answer" {
		return ""
	}

	var parts []string
	for _, content := range msg.Content {
		if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
			parts = append(parts, strings.TrimSpace(content.Text))
		}
	}
	return strings.Join(parts, "\n")
}
