package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	cacheStatusDisabled    = "disabled"
	cacheStatusMissing     = "missing"
	cacheStatusHealthy     = "healthy"
	cacheStatusCorrupt     = "corrupt"
	cacheStatusRebuilding  = "rebuilding"
	cacheStatusUnavailable = "unavailable"
)

type summaryCacheManager struct {
	mu         sync.Mutex
	path       string
	workers    int
	status     string
	reason     string
	backupPath string
	disk       *summaryDiskCache
	job        CacheJobState
	jobSeq     int
}

type cacheBuildTask struct {
	Path    string
	Meta    fileMeta
	Summary SessionSummary
}

type cacheBuildResult struct {
	Path    string
	Meta    fileMeta
	Summary SessionSummary
	Error   string
}

func newSummaryCacheManager(path string, workers int) *summaryCacheManager {
	manager := &summaryCacheManager{path: path, workers: workers}
	manager.refreshLocked()
	return manager
}

func (m *summaryCacheManager) refreshLocked() {
	if m.path == "" {
		m.status = cacheStatusDisabled
		m.reason = "cache disabled by -no-cache"
		return
	}
	if _, err := os.Stat(m.path); err != nil {
		if os.IsNotExist(err) {
			m.status = cacheStatusMissing
			m.reason = "cache file does not exist"
			return
		}
		m.status = cacheStatusUnavailable
		m.reason = err.Error()
		return
	}
	cache, err := openExistingSummaryDiskCache(m.path)
	if err != nil {
		if isSQLiteCorruptError(err) {
			m.status = cacheStatusCorrupt
			m.reason = err.Error()
			return
		}
		m.status = cacheStatusUnavailable
		m.reason = err.Error()
		return
	}
	m.disk = cache
	m.status = cacheStatusHealthy
	m.reason = ""
}

func (m *summaryCacheManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.disk != nil {
		_ = m.disk.Close()
		m.disk = nil
	}
}

func (m *summaryCacheManager) DiskCache() *summaryDiskCache {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.disk
}

func (m *summaryCacheManager) Status() CacheStatus {
	if m == nil {
		return CacheStatus{Status: cacheStatusDisabled, Reason: "cache manager unavailable", GeneratedAt: time.Now().Format(time.RFC3339)}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *summaryCacheManager) statusLocked() CacheStatus {
	status := m.status
	if m.job.Running {
		status = cacheStatusRebuilding
	}
	canBuild := !m.job.Running && (m.status == cacheStatusMissing || m.status == cacheStatusHealthy)
	canRebuild := !m.job.Running && m.status == cacheStatusCorrupt
	return CacheStatus{
		Status:      status,
		Path:        m.path,
		Workers:     m.workers,
		Reason:      m.reason,
		BackupPath:  m.backupPath,
		CanBuild:    canBuild,
		CanRebuild:  canRebuild,
		AutoFill:    m.workers > 0 && m.status == cacheStatusHealthy,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Job:         m.job,
	}
}

func (m *summaryCacheManager) StartAutoBuild(store *Store) CacheStatus {
	if m == nil || m.workers <= 0 {
		return m.Status()
	}
	m.mu.Lock()
	if m.status != cacheStatusHealthy || m.job.Running {
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	}
	m.mu.Unlock()
	return m.StartBuild(store, false)
}

func (m *summaryCacheManager) StartBuild(store *Store, rebuild bool) CacheStatus {
	if m == nil {
		return CacheStatus{Status: cacheStatusDisabled, Reason: "cache manager unavailable", GeneratedAt: time.Now().Format(time.RFC3339)}
	}
	m.mu.Lock()
	if m.job.Running {
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	}
	if rebuild {
		if m.status != cacheStatusCorrupt {
			status := m.statusLocked()
			m.mu.Unlock()
			return status
		}
		if err := m.backupCorruptAndCreateLocked(); err != nil {
			m.status = cacheStatusUnavailable
			m.reason = err.Error()
			status := m.statusLocked()
			m.mu.Unlock()
			return status
		}
	} else {
		switch m.status {
		case cacheStatusMissing:
			if err := m.createLocked(); err != nil {
				m.status = cacheStatusUnavailable
				m.reason = err.Error()
				status := m.statusLocked()
				m.mu.Unlock()
				return status
			}
		case cacheStatusHealthy:
		default:
			status := m.statusLocked()
			m.mu.Unlock()
			return status
		}
	}

	m.jobSeq++
	seq := m.jobSeq
	now := time.Now().Format(time.RFC3339)
	m.job = CacheJobState{Running: true, StartedAt: now}
	m.status = cacheStatusRebuilding
	m.reason = ""
	workers := m.workers
	if workers <= 0 {
		workers = 1
	}
	status := m.statusLocked()
	m.mu.Unlock()

	go store.runCacheBuild(seq, workers)
	return status
}

func (m *summaryCacheManager) createLocked() error {
	cache, err := openSummaryDiskCache(m.path)
	if err != nil {
		return err
	}
	m.disk = cache
	m.status = cacheStatusHealthy
	m.reason = ""
	return nil
}

func (m *summaryCacheManager) backupCorruptAndCreateLocked() error {
	if m.disk != nil {
		_ = m.disk.Close()
		m.disk = nil
	}
	backupPath := fmt.Sprintf("%s.corrupt.%s.bak", m.path, time.Now().Format("20060102T150405"))
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}
	if err := os.Rename(m.path, backupPath); err != nil {
		return err
	}
	m.backupPath = backupPath
	return m.createLocked()
}

func (m *summaryCacheManager) setJobTotal(seq int, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq == m.jobSeq && m.job.Running {
		m.job.Total = total
	}
}

func (m *summaryCacheManager) addJobProgress(seq int, cached int, skipped int, failed int, lastError string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq != m.jobSeq || !m.job.Running {
		return
	}
	m.job.Done++
	m.job.Cached += cached
	m.job.Skipped += skipped
	m.job.Failed += failed
	if lastError != "" {
		m.job.LastError = lastError
	}
}

func (m *summaryCacheManager) finishJob(seq int, lastError string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq != m.jobSeq || !m.job.Running {
		return
	}
	m.job.Running = false
	m.job.FinishedAt = time.Now().Format(time.RFC3339)
	if lastError != "" {
		m.job.LastError = lastError
	}
	if m.disk == nil {
		m.status = cacheStatusUnavailable
		if m.reason == "" {
			m.reason = "cache unavailable after build"
		}
		return
	}
	m.status = cacheStatusHealthy
	m.reason = ""
}

func (m *summaryCacheManager) ReportRuntimeError(err error) {
	if m == nil || err == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.disk != nil {
		_ = m.disk.Close()
		m.disk = nil
	}
	if isSQLiteCorruptError(err) {
		m.status = cacheStatusCorrupt
	} else {
		m.status = cacheStatusUnavailable
	}
	m.reason = err.Error()
}
