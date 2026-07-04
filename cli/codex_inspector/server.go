package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

func NewServer(store *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/overview", func(w http.ResponseWriter, r *http.Request) {
		filter := requestFilter(r, 0)
		sessions, warnings := store.LoadSessions(filter)
		heatmapSessions, heatmapWarnings := store.LoadSessionIndex()
		warnings = append(warnings, heatmapWarnings...)
		writeJSON(w, http.StatusOK, BuildOverview(store.CodexHome, sessions, heatmapSessions, store.Sources(), warnings))
	})
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		filter := requestFilter(r, 250)
		sessions, warnings := store.LoadSessions(filter)
		writeJSON(w, http.StatusOK, SessionsResponse{Sessions: sessions, Warnings: warnings})
	})
	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		id, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/sessions/"))
		if err != nil || strings.TrimSpace(id) == "" {
			http.Error(w, "invalid session id", http.StatusBadRequest)
			return
		}
		detail, ok, warnings := store.LoadSessionDetail(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found", "warnings": warnings})
			return
		}
		detail.Warnings = append(detail.Warnings, warnings...)
		writeJSON(w, http.StatusOK, detail)
	})
	mux.HandleFunc("/api/memory", func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := limitFromString(r.URL.Query().Get("limit"), 250)
		writeJSON(w, http.StatusOK, store.LoadMemory(query, limit))
	})
	mux.HandleFunc("/api/memory/file", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		detail, ok := store.LoadMemoryDetail(path)
		if !ok {
			writeJSON(w, http.StatusBadRequest, detail)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	})
	mux.HandleFunc("/api/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.Diagnostics())
	})
	mux.HandleFunc("/api/cache/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, store.CacheStatus())
	})
	mux.HandleFunc("/api/cache/build", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, store.StartCacheBuild())
	})
	mux.HandleFunc("/api/cache/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, store.StartCacheRebuild())
	})

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	static := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			index, err := fs.ReadFile(sub, "index.html")
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(index)
			return
		}
		static.ServeHTTP(w, r)
	})
	return mux
}

func requestFilter(r *http.Request, fallbackLimit int) QueryFilter {
	query := r.URL.Query()
	return parseFilter(
		query.Get("from"),
		query.Get("to"),
		query.Get("q"),
		limitFromString(query.Get("limit"), fallbackLimit),
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}
