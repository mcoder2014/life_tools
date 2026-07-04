package life_codex

import (
	"encoding/json"
	"time"
)

const (
	DefaultServerAddr       = "127.0.0.1:8899"
	DefaultServerConfigPath = "/etc/life_tools/life_codex_server.json"
	DefaultAgentConfigPath  = "/etc/life_tools/life_codex_agent.json"

	CommandCreateSession = "create_session"
	CommandStartTurn     = "start_turn"
	CommandForkSession   = "fork_session"

	SessionCreating = "creating"
	SessionIdle     = "idle"
	SessionQueued   = "queued"
	SessionRunning  = "running"
	SessionFailed   = "failed"

	EventInfo      = "info"
	EventUser      = "user"
	EventAssistant = "assistant"
	EventTool      = "tool"
	EventError     = "error"
	EventImage     = "image"
)

type Machine struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	AllowedRoots      []string `json:"allowed_roots"`
	CodexPath         string   `json:"codex_path"`
	MaxActiveSessions int      `json:"max_active_sessions"`
	LastSeenUnix      int64    `json:"last_seen_unix"`
	Remark            string   `json:"remark"`
}

type Session struct {
	ID           string       `json:"id"`
	MachineID    string       `json:"machine_id"`
	ThreadID     string       `json:"thread_id"`
	Title        string       `json:"title"`
	CWD          string       `json:"cwd"`
	Status       string       `json:"status"`
	Active       bool         `json:"active"`
	ForkedFromID string       `json:"forked_from_id,omitempty"`
	PendingText  string       `json:"pending_text,omitempty"`
	Queue        []QueuedTurn `json:"queue"`
	Events       []Event      `json:"events"`
	CreatedUnix  int64        `json:"created_unix"`
	UpdatedUnix  int64        `json:"updated_unix"`
	LastError    string       `json:"last_error,omitempty"`
}

type QueuedTurn struct {
	ID          string         `json:"id"`
	Text        string         `json:"text"`
	Skill       *SkillInput    `json:"skill,omitempty"`
	Images      []ImagePayload `json:"images,omitempty"`
	CreatedUnix int64          `json:"created_unix"`
}

type SkillInput struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type ImagePayload struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	DataBase64  string `json:"data_base64,omitempty"`
	LocalPath   string `json:"local_path,omitempty"`
}

type Event struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	MachineID   string          `json:"machine_id"`
	Type        string          `json:"type"`
	Text        string          `json:"text"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	CreatedUnix int64           `json:"created_unix"`
}

type AgentCommand struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	SessionID         string         `json:"session_id"`
	SourceSessionID   string         `json:"source_session_id,omitempty"`
	ThreadID          string         `json:"thread_id,omitempty"`
	CWD               string         `json:"cwd,omitempty"`
	Text              string         `json:"text,omitempty"`
	Skill             *SkillInput    `json:"skill,omitempty"`
	Images            []ImagePayload `json:"images,omitempty"`
	ApprovalPolicy    string         `json:"approval_policy,omitempty"`
	ApprovalsReviewer string         `json:"approvals_reviewer,omitempty"`
	CreatedUnix       int64          `json:"created_unix"`
}

type AgentReport struct {
	MachineID string          `json:"machine_id"`
	Token     string          `json:"token"`
	CommandID string          `json:"command_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	ThreadID  string          `json:"thread_id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Error     string          `json:"error,omitempty"`
	Event     *Event          `json:"event,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type AgentPollRequest struct {
	MachineID         string   `json:"machine_id"`
	Token             string   `json:"token"`
	Name              string   `json:"name"`
	AllowedRoots      []string `json:"allowed_roots"`
	CodexPath         string   `json:"codex_path"`
	MaxActiveSessions int      `json:"max_active_sessions"`
}

type EnrollRequest struct {
	Token             string   `json:"token"`
	Name              string   `json:"name"`
	AllowedRoots      []string `json:"allowed_roots"`
	CodexPath         string   `json:"codex_path"`
	MaxActiveSessions int      `json:"max_active_sessions"`
}

type EnrollResponse struct {
	MachineID string `json:"machine_id"`
	Token     string `json:"token"`
}

type StateSnapshot struct {
	Machines []Machine `json:"machines"`
	Sessions []Session `json:"sessions"`
	NowUnix  int64     `json:"now_unix"`
}

func nowUnix() int64 {
	return time.Now().Unix()
}
