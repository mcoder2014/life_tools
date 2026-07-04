package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxMemoryReadBytes = 512 * 1024

func (s *Store) LoadMemory(query string, limit int) MemoryResponse {
	files, warnings := s.memoryFiles()
	out := make([]MemoryFile, 0, len(files))
	for _, path := range files {
		item, err := s.memoryFileSummary(path, query)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if query != "" && item.Matches == 0 {
			continue
		}
		out = append(out, item)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return MemoryResponse{Files: out, Warnings: warnings}
}

func (s *Store) LoadMemoryDetail(relPath string) (MemoryDetail, bool) {
	path, ok := s.safeMemoryPath(relPath)
	if !ok {
		return MemoryDetail{Warnings: []string{"invalid memory path"}}, false
	}
	item, err := s.memoryFileSummary(path, "")
	if err != nil {
		return MemoryDetail{Warnings: []string{err.Error()}}, false
	}
	content, truncated, err := readTextFileLimit(path, maxMemoryReadBytes)
	if err != nil {
		return MemoryDetail{File: item, Warnings: []string{err.Error()}}, false
	}
	content = sanitizeText(content)
	detail := MemoryDetail{File: item, Content: content}
	if truncated {
		detail.Warnings = append(detail.Warnings, fmt.Sprintf("content truncated at %d bytes", maxMemoryReadBytes))
	}
	return detail, true
}

func (s *Store) memoryFiles() ([]string, []string) {
	root := filepath.Join(s.CodexHome, "memories")
	allowed := map[string]bool{
		"MEMORY.md":         true,
		"memory_summary.md": true,
		"raw_memories.md":   true,
	}
	var files []string
	var warnings []string
	for name := range allowed {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}
	rolloutDir := filepath.Join(root, "rollout_summaries")
	err := filepath.WalkDir(rolloutDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("walk %s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("walk memory rollouts: %v", err))
	}
	sort.Slice(files, func(i, j int) bool {
		left, leftErr := os.Stat(files[i])
		right, rightErr := os.Stat(files[j])
		if leftErr == nil && rightErr == nil && !left.ModTime().Equal(right.ModTime()) {
			return left.ModTime().After(right.ModTime())
		}
		return files[i] < files[j]
	})
	return files, warnings
}

func (s *Store) memoryFileSummary(path string, query string) (MemoryFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return MemoryFile{}, fmt.Errorf("stat memory file %s: %w", path, err)
	}
	content, truncated, err := readTextFileLimit(path, 96*1024)
	if err != nil {
		return MemoryFile{}, fmt.Errorf("read memory file %s: %w", path, err)
	}
	content = sanitizeText(content)
	rel, _ := filepath.Rel(filepath.Join(s.CodexHome, "memories"), path)
	rel = filepath.ToSlash(rel)
	item := MemoryFile{
		Path:       rel,
		Kind:       memoryKind(rel),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		Title:      memoryTitle(rel, content),
		Preview:    memoryPreview(content, query),
		Matches:    countMatches(content, query),
	}
	if truncated {
		item.Preview = truncateText(item.Preview+"\n(content preview truncated)", 1200)
	}
	return item, nil
}

func (s *Store) safeMemoryPath(relPath string) (string, bool) {
	relPath = filepath.Clean(strings.TrimSpace(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return "", false
	}
	path := filepath.Join(s.CodexHome, "memories", relPath)
	root := filepath.Join(s.CodexHome, "memories")
	if !strings.HasPrefix(path, root+string(filepath.Separator)) && path != root {
		return "", false
	}
	if !strings.HasSuffix(path, ".md") {
		return "", false
	}
	return path, true
}

func readTextFileLimit(path string, maxBytes int64) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	content := make([]byte, maxBytes+1)
	n, err := file.Read(content)
	if err != nil && n == 0 {
		return "", false, err
	}
	truncated := int64(n) > maxBytes
	if truncated {
		n = int(maxBytes)
	}
	return string(content[:n]), truncated, nil
}

func memoryKind(rel string) string {
	switch {
	case rel == "MEMORY.md":
		return "index"
	case rel == "memory_summary.md":
		return "summary"
	case strings.HasPrefix(rel, "rollout_summaries/"):
		return "rollout"
	default:
		return "note"
	}
}

func memoryTitle(rel string, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		if strings.HasPrefix(line, "thread_id:") {
			continue
		}
		if line != "" {
			break
		}
	}
	return strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
}

func memoryPreview(content string, query string) string {
	lines := strings.Split(content, "\n")
	if query != "" {
		for i, line := range lines {
			if containsFold(line, query) {
				start := i - 2
				if start < 0 {
					start = 0
				}
				end := i + 3
				if end > len(lines) {
					end = len(lines)
				}
				return truncateText(strings.Join(lines[start:end], "\n"), 1000)
			}
		}
	}
	var picked []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		picked = append(picked, line)
		if len(picked) >= 8 {
			break
		}
	}
	return truncateText(strings.Join(picked, "\n"), 1000)
}

func countMatches(content string, query string) int {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}
	return strings.Count(strings.ToLower(content), query)
}
