package life_codex

import (
	"path/filepath"
	"testing"
)

func TestPathInAllowedRoots(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "project", "file.txt")
	outside := filepath.Join(t.TempDir(), "file.txt")
	if !PathInAllowedRoots(inside, []string{root}) {
		t.Fatalf("inside path rejected")
	}
	if PathInAllowedRoots(outside, []string{root}) {
		t.Fatalf("outside path accepted")
	}
	if PathInAllowedRoots(filepath.Join(root, ".."), []string{root}) {
		t.Fatalf("parent traversal accepted")
	}
}
