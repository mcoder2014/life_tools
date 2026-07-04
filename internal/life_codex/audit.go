package life_codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AuditLogger struct {
	dir string
}

type AuditEntry struct {
	TimeUnix  int64           `json:"time_unix"`
	Action    string          `json:"action"`
	MachineID string          `json:"machine_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	CommandID string          `json:"command_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func NewAuditLogger(dir string) *AuditLogger {
	return &AuditLogger{dir: dir}
}

func (l *AuditLogger) Log(action string, machineID string, sessionID string, commandID string, payload any) error {
	if l == nil || l.dir == "" {
		return nil
	}
	if err := os.MkdirAll(l.dir, 0700); err != nil {
		return err
	}
	content, err := json.Marshal(sanitizeAuditPayload(payload))
	if err != nil {
		return err
	}
	entry := AuditEntry{
		TimeUnix:  time.Now().Unix(),
		Action:    action,
		MachineID: machineID,
		SessionID: sessionID,
		CommandID: commandID,
		Payload:   content,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := filepath.Join(l.dir, time.Now().Format("2006-01-02")+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(line, '\n'))
	return err
}

func ClearAuditBefore(dir string, before time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		if len(entry.Name()) < len("2006-01-02") {
			continue
		}
		day, err := time.Parse("2006-01-02", entry.Name()[:len("2006-01-02")])
		if err != nil || !day.Before(before) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func sanitizeAuditPayload(payload any) any {
	content, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return payload
	}
	return removeImageData(value)
}

func removeImageData(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.EqualFold(key, "data_base64") {
				if child != nil && child != "" {
					typed[key] = "<redacted>"
				}
				continue
			}
			typed[key] = removeImageData(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = removeImageData(child)
		}
		return typed
	default:
		return value
	}
}
