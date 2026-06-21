package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Logger interface {
	Log(sessionID string, message string) error
}

type LoggerFunc func(sessionID string, message string) error

func (f LoggerFunc) Log(sessionID string, message string) error {
	return f(sessionID, message)
}

type FileLogger struct {
	Now func() time.Time
}

func (l FileLogger) Log(sessionID string, message string) error {
	now := time.Now()
	if l.Now != nil {
		now = l.Now()
	}

	logDir := DefaultLogDir()
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(logDir, LogFileName(now.Format("2006-01-02"), sessionID))
	line := fmt.Sprintf("%s %s\n", now.Format(time.RFC3339), message)
	return appendFile(path, line)
}

func DefaultLogDir() string {
	home, _ := os.UserHomeDir()
	return defaultLogDir(runtime.GOOS, home)
}

func defaultLogDir(goos string, home string) string {
	if goos == "linux" {
		return "/var/log/codex_hook_notify"
	}
	if home == "" {
		return filepath.Join(".", ".codex_hook_notify", "logs")
	}
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Logs", "codex_hook_notify")
	}
	return filepath.Join(home, ".codex_hook_notify", "logs")
}

func LogFileName(date string, sessionID string) string {
	if sessionID == "" {
		sessionID = "unknown"
	}
	return date + "_" + safeFilePart(sessionID) + ".log"
}

func safeFilePart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if isSafeFileRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func isSafeFileRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '-' || r == '_'
}

func appendFile(path string, line string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line)
	return err
}
