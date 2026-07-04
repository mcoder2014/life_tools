package life_codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditRedactsImageData(t *testing.T) {
	dir := t.TempDir()
	logger := NewAuditLogger(dir)
	err := logger.Log("image", "machine", "session", "cmd", map[string]any{
		"images": []ImagePayload{{Name: "a.png", DataBase64: "raw-image"}},
	})
	if err != nil {
		t.Fatalf("log audit: %v", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read audit dir: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if strings.Contains(string(content), "raw-image") {
		t.Fatalf("audit contains image data: %s", content)
	}
}

func TestClearAuditBefore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01.jsonl"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := time.Parse("2006-01-02", "2026-01-02")
	removed, err := ClearAuditBefore(dir, before)
	if err != nil {
		t.Fatalf("clear audit: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "bad.jsonl")); err != nil {
		t.Fatalf("bad filename should be preserved: %v", err)
	}
}
