package life_codex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HTTPServer struct {
	config ServerConfig
	store  *StateStore
	mux    *http.ServeMux
}

func NewHTTPServer(config ServerConfig, store *StateStore) *HTTPServer {
	server := &HTTPServer{config: config, store: store, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *HTTPServer) Handler() http.Handler {
	return s.mux
}

func (s *HTTPServer) routes() {
	s.mux.HandleFunc("/api/state", s.requireAdmin(s.handleState))
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/enrollment-tokens", s.requireAdmin(s.handleEnrollmentTokens))
	s.mux.HandleFunc("/api/sessions", s.requireAdmin(s.handleSessions))
	s.mux.HandleFunc("/api/sessions/", s.requireAdmin(s.handleSessionAction))
	s.mux.HandleFunc("/api/audit/clear", s.requireAdmin(s.handleAuditClear))
	s.mux.HandleFunc("/agent/enroll", s.handleAgentEnroll)
	s.mux.HandleFunc("/agent/poll", s.handleAgentPoll)
	s.mux.HandleFunc("/agent/report", s.handleAgentReport)
	s.mux.HandleFunc("/", s.handleStatic)
}

func (s *HTTPServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.store.Snapshot())
}

func (s *HTTPServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !s.validAdmin(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, cancel := s.store.Subscribe()
	defer cancel()
	writeSSE(w, s.store.Snapshot())
	flusher.Flush()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			writeSSE(w, s.store.Snapshot())
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *HTTPServer) handleEnrollmentTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token, err := s.store.CreateEnrollToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_in_seconds": 900})
}

func (s *HTTPServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		MachineID string `json:"machine_id"`
		CWD       string `json:"cwd"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	session, err := s.store.CreateSession(req.MachineID, req.CWD)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *HTTPServer) handleSessionAction(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	sessionID := parts[0]
	action := parts[1]
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	switch action {
	case "turn":
		var req struct {
			Text   string         `json:"text"`
			Skill  *SkillInput    `json:"skill"`
			Images []ImagePayload `json:"images"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(req.Text) == "" && len(req.Images) == 0 {
			writeError(w, http.StatusBadRequest, "text or image is required")
			return
		}
		if err := ValidateImagePayloads(req.Images, s.config.MaxImagesPerTurn, s.config.MaxImageBytes); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.store.StartTurn(sessionID, req.Text, req.Skill, req.Images); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case "fork":
		var req struct {
			Text string `json:"text"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(req.Text) == "" {
			writeError(w, http.StatusBadRequest, "text is required")
			return
		}
		session, err := s.store.ForkSession(sessionID, req.Text)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, session)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *HTTPServer) handleAuditClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Before  string `json:"before"`
		Confirm bool   `json:"confirm"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Confirm {
		writeError(w, http.StatusBadRequest, "confirm is required")
		return
	}
	before, err := time.Parse("2006-01-02", req.Before)
	if err != nil {
		writeError(w, http.StatusBadRequest, "before must be YYYY-MM-DD")
		return
	}
	removed, err := s.store.ClearAudit(before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (s *HTTPServer) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req EnrollRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.store.Enroll(req)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *HTTPServer) handleAgentPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req AgentPollRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	commands, err := s.store.Poll(req)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": commands})
}

func (s *HTTPServer) handleAgentReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var report AgentReport
	if err := decodeJSON(r, &report); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Report(report); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *HTTPServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		s.serveStaticFile(w, r, "index.html")
		return
	}
	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if strings.HasPrefix(clean, "..") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if s.serveStaticFile(w, r, clean) {
		return
	}
	s.serveStaticFile(w, r, "index.html")
}

func (s *HTTPServer) serveStaticFile(w http.ResponseWriter, r *http.Request, name string) bool {
	if s.config.WebRoot == "" {
		writeError(w, http.StatusNotFound, "web root is not configured")
		return false
	}
	path := filepath.Join(s.config.WebRoot, name)
	if !PathInAllowedRoots(path, []string{s.config.WebRoot}) {
		writeError(w, http.StatusNotFound, "not found")
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		if name == "index.html" {
			writeError(w, http.StatusNotFound, "web build is missing")
		}
		return false
	}
	http.ServeFile(w, r, path)
	return true
}

func (s *HTTPServer) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.validAdmin(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (s *HTTPServer) validAdmin(r *http.Request) bool {
	token := r.Header.Get("X-Life-Codex-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	return token != "" && token == s.config.AdminToken
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeSSE(w http.ResponseWriter, value any) {
	content, err := json.Marshal(value)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: state\ndata: %s\n\n", content)
}
