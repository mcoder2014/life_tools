package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Store struct {
	CodexHome    string
	CachePath    string
	mu           sync.Mutex
	cond         *sync.Cond
	loading      bool
	sessionCache *sessionCache
	cacheManager *summaryCacheManager
}

type sessionCache struct {
	loadedAt time.Time
	sessions []SessionSummary
	byID     map[string]SessionSummary
	warnings []string
}

type sessionIndexLine struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

func NewStore(codexHome string) *Store {
	return NewStoreWithCache(codexHome, defaultCachePath())
}

func NewStoreWithCache(codexHome string, cachePath string) *Store {
	return NewStoreWithCacheWorkers(codexHome, cachePath, 0)
}

func NewStoreWithCacheWorkers(codexHome string, cachePath string, cacheWorkers int) *Store {
	if codexHome == "" {
		codexHome = defaultCodexHome()
	}
	store := &Store{CodexHome: filepath.Clean(codexHome), CachePath: cachePath}
	store.cond = sync.NewCond(&store.mu)
	store.cacheManager = newSummaryCacheManager(cachePath, cacheWorkers)
	store.cacheManager.StartAutoBuild(store)
	return store
}

func (s *Store) Sources() []SourceStatus {
	items := []struct {
		name  string
		rel   string
		notes []string
	}{
		{name: "session index", rel: "session_index.jsonl"},
		{name: "sessions", rel: "sessions"},
		{name: "memory index", rel: filepath.Join("memories", "MEMORY.md")},
		{name: "memory summary", rel: filepath.Join("memories", "memory_summary.md")},
		{name: "memory rollouts", rel: filepath.Join("memories", "rollout_summaries")},
		{name: "thread sqlite", rel: "state_5.sqlite", notes: []string{"schema only in diagnostics"}},
		{name: "goal sqlite", rel: "goals_1.sqlite", notes: []string{"schema only in diagnostics"}},
		{name: "memory sqlite", rel: "memories_1.sqlite", notes: []string{"schema only in diagnostics"}},
		{name: "auth files", rel: "auth.json", notes: []string{"excluded by design; content is never read"}},
	}

	statuses := make([]SourceStatus, 0, len(items))
	for _, item := range items {
		statuses = append(statuses, sourceStatus(item.name, filepath.Join(s.CodexHome, item.rel), item.notes))
	}
	if s.CachePath != "" {
		statuses = append(statuses, sourceStatus("summary cache", s.CachePath, []string{"tool-owned sqlite cache; no auth data"}))
	}
	return statuses
}

func (s *Store) Close() {
	if s.cacheManager != nil {
		s.cacheManager.Close()
	}
}

func sourceStatus(name string, path string, notes []string) SourceStatus {
	status := SourceStatus{Name: name, Path: path, Notes: append([]string(nil), notes...)}
	info, err := os.Stat(path)
	if err != nil {
		status.Exists = false
		return status
	}
	status.Exists = true
	status.Size = info.Size()
	status.ModifiedAt = info.ModTime().Format("2006-01-02T15:04:05Z07:00")
	if info.IsDir() {
		status.Size = 0
	}
	return status
}

func (s *Store) LoadSessions(filter QueryFilter) ([]SessionSummary, []string) {
	allSessions, warnings := s.loadAllSessions()
	candidates := make([]SessionSummary, 0, len(allSessions))
	for _, summary := range allSessions {
		if sessionMatchesRange(summary, filter) {
			candidates = append(candidates, summary)
		}
	}
	sortSessions(candidates)
	if filter.Query != "" {
		var enrichWarnings []string
		candidates, enrichWarnings = s.enrichSummaries(candidates)
		warnings = append(warnings, enrichWarnings...)
		candidates = filterByQuery(candidates, filter.Query)
		sortSessions(candidates)
	}
	if filter.Limit > 0 && len(candidates) > filter.Limit {
		candidates = candidates[:filter.Limit]
	}
	sessions, enrichWarnings := s.enrichSummaries(candidates)
	warnings = append(warnings, enrichWarnings...)
	return sessions, append([]string(nil), warnings...)
}

func (s *Store) LoadSessionIndex() ([]SessionSummary, []string) {
	return s.loadAllSessions()
}

func (s *Store) loadAllSessions() ([]SessionSummary, []string) {
	s.mu.Lock()
	for {
		if s.sessionCache != nil && time.Since(s.sessionCache.loadedAt) < 5*time.Minute {
			sessions := append([]SessionSummary(nil), s.sessionCache.sessions...)
			warnings := append([]string(nil), s.sessionCache.warnings...)
			s.mu.Unlock()
			return sessions, warnings
		}
		if !s.loading {
			s.loading = true
			break
		}
		s.cond.Wait()
	}
	s.mu.Unlock()

	sessions, byID, warnings := s.buildSessionCache()
	if warning := s.cacheWarning(); warning != "" {
		warnings = append(warnings, warning)
	}

	s.mu.Lock()
	s.sessionCache = &sessionCache{
		loadedAt: time.Now(),
		sessions: append([]SessionSummary(nil), sessions...),
		byID:     copySessionMap(byID),
		warnings: append([]string(nil), warnings...),
	}
	s.loading = false
	s.cond.Broadcast()
	s.mu.Unlock()
	return sessions, warnings
}

func (s *Store) buildSessionCache() ([]SessionSummary, map[string]SessionSummary, []string) {
	index, warnings := s.readSessionIndex()
	files, fileWarnings := s.rolloutFiles()
	warnings = append(warnings, fileWarnings...)

	byID := make(map[string]SessionSummary)
	for _, path := range files {
		summary := summaryFromPath(path)
		if summary.ID == "" {
			summary.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		summary.Path = path
		if item, ok := index[summary.ID]; ok {
			mergeIndex(&summary, item)
		}
		byID[summary.ID] = summary
	}

	for id, item := range index {
		if _, ok := byID[id]; ok {
			continue
		}
		summary := SessionSummary{ID: id}
		mergeIndex(&summary, item)
		byID[id] = summary
	}

	sessions := make([]SessionSummary, 0, len(byID))
	for _, summary := range byID {
		sessions = append(sessions, summary)
	}
	sortSessions(sessions)
	return sessions, byID, warnings
}

func (s *Store) enrichSummaries(sessions []SessionSummary) ([]SessionSummary, []string) {
	out := make([]SessionSummary, 0, len(sessions))
	var warnings []string
	now := time.Now()
	for _, summary := range sessions {
		enriched := summary
		meta, metaOK := statFile(summary.Path)
		historicalFile := metaOK && cacheableFile(meta, now)
		if cached, ok := s.cachedSummary(summary.ID); ok && cached.Enriched && historicalFile && cacheableSummary(cached, now) {
			enriched = cached
		} else if summary.Path != "" {
			if cache := s.diskCache(); historicalFile && cache != nil {
				cached, ok, err := cache.Lookup(summary.Path, meta)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("cache lookup %s: %v", summary.Path, err))
					s.reportCacheError(err)
				}
				if ok && cacheableSummary(cached, now) {
					detail := SessionDetail{Summary: cached}
					mergeCachedSummary(&detail, summary)
					enriched = detail.Summary
					enriched.Enriched = true
					s.updateCachedSummary(enriched)
					out = append(out, enriched)
					continue
				}
			}
			detail, err := ParseRolloutFile(summary.Path, false)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("parse %s: %v", summary.Path, err))
			} else {
				mergeCachedSummary(&detail, summary)
				enriched = detail.Summary
				enriched.Enriched = true
				s.updateCachedSummary(enriched)
				if cache := s.diskCache(); historicalFile && cacheableSummary(enriched, now) && cache != nil {
					if err := cache.Upsert(summary.Path, meta, enriched); err != nil {
						warnings = append(warnings, fmt.Sprintf("cache write %s: %v", summary.Path, err))
						s.reportCacheError(err)
					}
				}
			}
		}
		out = append(out, enriched)
	}
	return out, warnings
}

func (s *Store) diskCache() *summaryDiskCache {
	if s.cacheManager == nil {
		return nil
	}
	return s.cacheManager.DiskCache()
}

func (s *Store) reportCacheError(err error) {
	if s.cacheManager != nil {
		s.cacheManager.ReportRuntimeError(err)
	}
}

func (s *Store) cacheWarning() string {
	if s.cacheManager == nil {
		return ""
	}
	status := s.cacheManager.Status()
	switch status.Status {
	case cacheStatusCorrupt, cacheStatusUnavailable:
		if status.Reason != "" {
			return fmt.Sprintf("summary cache %s: %s", status.Status, status.Reason)
		}
		return fmt.Sprintf("summary cache %s", status.Status)
	default:
		return ""
	}
}

func (s *Store) CacheStatus() CacheStatus {
	if s.cacheManager == nil {
		return CacheStatus{Status: cacheStatusDisabled, Reason: "cache manager unavailable", GeneratedAt: time.Now().Format(time.RFC3339)}
	}
	return s.cacheManager.Status()
}

func (s *Store) StartCacheBuild() CacheStatus {
	if s.cacheManager == nil {
		return s.CacheStatus()
	}
	return s.cacheManager.StartBuild(s, false)
}

func (s *Store) StartCacheRebuild() CacheStatus {
	if s.cacheManager == nil {
		return s.CacheStatus()
	}
	return s.cacheManager.StartBuild(s, true)
}

func (s *Store) runCacheBuild(seq int, workers int) {
	tasks, skipped, failed, lastError := s.cacheBuildTasks()
	if workers < 1 {
		workers = 1
	}
	if workers > 16 {
		workers = 16
	}
	s.cacheManager.setJobTotal(seq, len(tasks)+skipped+failed)
	for i := 0; i < skipped; i++ {
		s.cacheManager.addJobProgress(seq, 0, 1, 0, "")
	}
	for i := 0; i < failed; i++ {
		s.cacheManager.addJobProgress(seq, 0, 0, 1, lastError)
	}
	if len(tasks) == 0 || s.diskCache() == nil {
		s.cacheManager.finishJob(seq, lastError)
		return
	}

	taskCh := make(chan cacheBuildTask)
	resultCh := make(chan cacheBuildResult)
	var wg sync.WaitGroup
	now := time.Now()
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				detail, err := ParseRolloutFile(task.Path, false)
				if err != nil {
					resultCh <- cacheBuildResult{Path: task.Path, Meta: task.Meta, Error: err.Error()}
					continue
				}
				mergeCachedSummary(&detail, task.Summary)
				detail.Summary.Enriched = true
				if !cacheableSummary(detail.Summary, now) {
					resultCh <- cacheBuildResult{Path: task.Path, Meta: task.Meta, Error: "session is not historical"}
					continue
				}
				resultCh <- cacheBuildResult{Path: task.Path, Meta: task.Meta, Summary: detail.Summary}
			}
		}()
	}

	go func() {
		for _, task := range tasks {
			taskCh <- task
		}
		close(taskCh)
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		if result.Error != "" {
			s.cacheManager.addJobProgress(seq, 0, 0, 1, result.Error)
			lastError = result.Error
			continue
		}
		cache := s.diskCache()
		if cache == nil {
			lastError = "cache unavailable"
			s.cacheManager.addJobProgress(seq, 0, 0, 1, lastError)
			continue
		}
		if err := cache.Upsert(result.Path, result.Meta, result.Summary); err != nil {
			lastError = err.Error()
			s.reportCacheError(err)
			s.cacheManager.addJobProgress(seq, 0, 0, 1, lastError)
			continue
		}
		s.updateCachedSummary(result.Summary)
		s.cacheManager.addJobProgress(seq, 1, 0, 0, "")
	}
	s.cacheManager.finishJob(seq, lastError)
}

func (s *Store) cacheBuildTasks() ([]cacheBuildTask, int, int, string) {
	index, _ := s.readSessionIndex()
	files, _ := s.rolloutFiles()
	now := time.Now()
	cache := s.diskCache()
	if cache == nil {
		return nil, 0, 1, "cache unavailable"
	}
	tasks := make([]cacheBuildTask, 0, len(files))
	skipped := 0
	failed := 0
	lastError := ""
	for _, path := range files {
		meta, ok := statFile(path)
		if !ok || !cacheableFile(meta, now) {
			continue
		}
		if cached, ok, err := cache.Lookup(path, meta); err != nil {
			lastError = err.Error()
			failed++
			s.reportCacheError(err)
			break
		} else if ok && cacheableSummary(cached, now) {
			skipped++
			continue
		}

		summary := summaryFromPath(path)
		if summary.ID == "" {
			summary.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		summary.Path = path
		if item, ok := index[summary.ID]; ok {
			mergeIndex(&summary, item)
		}
		tasks = append(tasks, cacheBuildTask{Path: path, Meta: meta, Summary: summary})
	}
	return tasks, skipped, failed, lastError
}

func (s *Store) cachedSummary(id string) (SessionSummary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionCache == nil {
		return SessionSummary{}, false
	}
	summary, ok := s.sessionCache.byID[id]
	return summary, ok
}

func (s *Store) updateCachedSummary(summary SessionSummary) {
	if summary.ID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionCache == nil {
		return
	}
	s.sessionCache.byID[summary.ID] = summary
	for i := range s.sessionCache.sessions {
		if s.sessionCache.sessions[i].ID == summary.ID {
			s.sessionCache.sessions[i] = summary
			return
		}
	}
}

func (s *Store) cachedSession(id string) (SessionSummary, bool, []string) {
	_, warnings := s.loadAllSessions()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionCache == nil {
		return SessionSummary{}, false, warnings
	}
	if summary, ok := s.sessionCache.byID[id]; ok {
		return summary, true, warnings
	}
	for _, summary := range s.sessionCache.sessions {
		if summary.ID == id || strings.TrimSuffix(filepath.Base(summary.Path), filepath.Ext(summary.Path)) == id || summary.Title == id {
			return summary, true, warnings
		}
	}
	return SessionSummary{}, false, warnings
}

func copySessionMap(in map[string]SessionSummary) map[string]SessionSummary {
	out := make(map[string]SessionSummary, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeCachedSummary(detail *SessionDetail, cached SessionSummary) {
	if cached.ID != "" {
		detail.Summary.ID = cached.ID
	}
	if cached.Title != "" {
		detail.Summary.Title = cached.Title
	}
	if cached.Path != "" {
		detail.Summary.Path = cached.Path
	}
	if detail.Summary.Cwd == "" {
		detail.Summary.Cwd = cached.Cwd
	}
	if detail.Summary.Model == "" {
		detail.Summary.Model = cached.Model
	}
	if detail.Summary.ModelProvider == "" {
		detail.Summary.ModelProvider = cached.ModelProvider
	}
	if cached.UpdatedAt != "" {
		detail.Summary.UpdatedAt = cached.UpdatedAt
	}
	if detail.Summary.StartedAt == "" {
		detail.Summary.StartedAt = cached.StartedAt
	}
	if detail.Summary.Preview == "" {
		detail.Summary.Preview = cached.Preview
	}
}

func (s *Store) LoadSessionDetail(id string) (SessionDetail, bool, []string) {
	summary, ok, warnings := s.cachedSession(id)
	if ok && summary.Path != "" {
		detail, err := ParseRolloutFile(summary.Path, true)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("parse %s: %v", summary.Path, err))
			return SessionDetail{}, false, warnings
		}
		mergeCachedSummary(&detail, summary)
		return detail, true, warnings
	}

	files, fileWarnings := s.rolloutFiles()
	warnings = append(warnings, fileWarnings...)
	for i := len(files) - 1; i >= 0; i-- {
		path := files[i]
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if id != base && !strings.Contains(base, id) {
			continue
		}
		detail, err := ParseRolloutFile(path, true)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("parse %s: %v", path, err))
			return SessionDetail{}, false, warnings
		}
		return detail, true, warnings
	}
	return SessionDetail{}, false, warnings
}

func (s *Store) readSessionIndex() (map[string]sessionIndexLine, []string) {
	path := filepath.Join(s.CodexHome, "session_index.jsonl")
	file, err := os.Open(path)
	if err != nil {
		return map[string]sessionIndexLine{}, []string{fmt.Sprintf("session index unavailable: %v", err)}
	}
	defer file.Close()

	out := make(map[string]sessionIndexLine)
	var warnings []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item sessionIndexLine
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			warnings = append(warnings, fmt.Sprintf("bad session_index line %d: %v", lineNo, err))
			continue
		}
		if item.ID == "" {
			warnings = append(warnings, fmt.Sprintf("session_index line %d has no id", lineNo))
			continue
		}
		out[item.ID] = item
	}
	if err := scanner.Err(); err != nil {
		warnings = append(warnings, fmt.Sprintf("read session_index: %v", err))
	}
	return out, warnings
}

func (s *Store) rolloutFiles() ([]string, []string) {
	root := filepath.Join(s.CodexHome, "sessions")
	var files []string
	var warnings []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("walk %s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), "rollout-") && strings.HasSuffix(d.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("walk sessions: %v", err))
	}
	sort.Strings(files)
	return files, warnings
}

func mergeIndex(summary *SessionSummary, item sessionIndexLine) {
	if summary.ID == "" {
		summary.ID = item.ID
	}
	if item.ThreadName != "" {
		summary.Title = sanitizeText(item.ThreadName)
	}
	if summary.Title == "" {
		summary.Title = summary.ID
	}
	if item.UpdatedAt != "" {
		summary.UpdatedAt = item.UpdatedAt
	}
}

func summaryFromPath(path string) SessionSummary {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	summary := SessionSummary{
		ID:        base,
		Title:     base,
		Path:      path,
		StartedAt: timeFromRolloutPath(path),
		UpdatedAt: timeFromRolloutPath(path),
	}
	file, err := os.Open(path)
	if err != nil {
		return summary
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 512*1024)
	for lineNo := 0; scanner.Scan() && lineNo < 12; lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record rolloutLine
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if summary.StartedAt == "" && record.Timestamp != "" {
			summary.StartedAt = record.Timestamp
		}
		if record.Timestamp != "" {
			summary.UpdatedAt = record.Timestamp
		}
		if record.Type != "session_meta" && record.Type != "turn_context" {
			continue
		}
		payload := decodePayload(record.Payload)
		if id := stringField(payload, "id"); id != "" {
			summary.ID = id
		}
		if cwd := stringField(payload, "cwd"); cwd != "" && summary.Cwd == "" {
			summary.Cwd = sanitizeText(cwd)
		}
		if model := stringField(payload, "model"); model != "" && summary.Model == "" {
			summary.Model = sanitizeText(model)
		}
		if provider := stringField(payload, "model_provider"); provider != "" && summary.ModelProvider == "" {
			summary.ModelProvider = sanitizeText(provider)
		}
		if summary.ID != base && summary.Cwd != "" {
			break
		}
	}
	if summary.Title == base {
		summary.Title = summary.ID
	}
	return summary
}

func timeFromRolloutPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.TrimPrefix(name, "rollout-")
	if len(name) < len("2006-01-02T15-04-05") {
		return ""
	}
	candidate := name[:len("2006-01-02T15-04-05")]
	date := candidate[:10]
	clock := strings.ReplaceAll(candidate[11:], "-", ":")
	return date + "T" + clock
}

func sessionMatchesRange(summary SessionSummary, filter QueryFilter) bool {
	timeValue := summary.UpdatedAt
	if timeValue == "" {
		timeValue = summary.StartedAt
	}
	return inRange(timeValue, filter)
}

func sessionMatches(summary SessionSummary, filter QueryFilter) bool {
	if !sessionMatchesRange(summary, filter) {
		return false
	}
	query := strings.TrimSpace(filter.Query)
	if query == "" {
		return true
	}
	fields := []string{summary.ID, summary.Title, summary.Cwd, summary.Model, summary.ModelProvider, summary.Preview}
	return containsFold(strings.Join(fields, "\n"), query)
}

func filterByQuery(sessions []SessionSummary, query string) []SessionSummary {
	filter := QueryFilter{Query: strings.TrimSpace(query)}
	out := make([]SessionSummary, 0, len(sessions))
	for _, summary := range sessions {
		if sessionMatches(summary, filter) {
			out = append(out, summary)
		}
	}
	return out
}

func sortSessions(sessions []SessionSummary) {
	sort.Slice(sessions, func(i, j int) bool {
		left := sessions[i].UpdatedAt
		if left == "" {
			left = sessions[i].StartedAt
		}
		right := sessions[j].UpdatedAt
		if right == "" {
			right = sessions[j].StartedAt
		}
		leftTime, leftOK := parseTime(left)
		rightTime, rightOK := parseTime(right)
		if leftOK && rightOK {
			return leftTime.After(rightTime)
		}
		if left != right {
			return left > right
		}
		return sessions[i].ID > sessions[j].ID
	})
}

func limitFromString(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 1000 {
		return 1000
	}
	return n
}
