package life_codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ServerConfig struct {
	Addr                     string `json:"addr"`
	AdminToken               string `json:"admin_token"`
	DataDir                  string `json:"data_dir"`
	AuditDir                 string `json:"audit_dir"`
	WebRoot                  string `json:"web_root"`
	MaxImageBytes            int64  `json:"max_image_bytes"`
	MaxImagesPerTurn         int    `json:"max_images_per_turn"`
	DefaultApprovalPolicy    string `json:"default_approval_policy"`
	DefaultApprovalsReviewer string `json:"default_approvals_reviewer"`
}

type AgentConfig struct {
	ServerURL         string   `json:"server_url"`
	MachineID         string   `json:"machine_id"`
	AgentToken        string   `json:"agent_token"`
	MachineName       string   `json:"machine_name"`
	AllowedRoots      []string `json:"allowed_roots"`
	CodexPath         string   `json:"codex_path"`
	AttachmentDir     string   `json:"attachment_dir"`
	MaxActiveSessions int      `json:"max_active_sessions"`
	PollSeconds       int      `json:"poll_seconds"`
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:                     DefaultServerAddr,
		DataDir:                  "/var/lib/life_tools/life_codex_server",
		AuditDir:                 "/var/log/life_tools/life_codex_server",
		WebRoot:                  "/usr/local/share/life_tools/life_codex",
		MaxImageBytes:            10 << 20,
		MaxImagesPerTurn:         5,
		DefaultApprovalPolicy:    "on-request",
		DefaultApprovalsReviewer: "auto_review",
	}
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		ServerURL:         "http://127.0.0.1:8899",
		CodexPath:         "codex",
		AttachmentDir:     "/var/lib/life_tools/life_codex_agent/attachments",
		MaxActiveSessions: 2,
		PollSeconds:       1,
	}
}

func LoadServerConfig(path string) (ServerConfig, error) {
	config := DefaultServerConfig()
	if strings.TrimSpace(path) == "" {
		return config, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ServerConfig{}, err
	}
	if err := json.Unmarshal(content, &config); err != nil {
		return ServerConfig{}, err
	}
	if config.AdminToken == "" {
		return ServerConfig{}, fmt.Errorf("admin_token is required")
	}
	if config.MaxImageBytes <= 0 {
		config.MaxImageBytes = 10 << 20
	}
	if config.MaxImagesPerTurn <= 0 {
		config.MaxImagesPerTurn = 5
	}
	if config.DefaultApprovalPolicy == "" {
		config.DefaultApprovalPolicy = "on-request"
	}
	if config.DefaultApprovalsReviewer == "" {
		config.DefaultApprovalsReviewer = "auto_review"
	}
	return config, nil
}

func LoadAgentConfig(path string) (AgentConfig, error) {
	config := DefaultAgentConfig()
	if strings.TrimSpace(path) == "" {
		return config, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return AgentConfig{}, err
	}
	if err := json.Unmarshal(content, &config); err != nil {
		return AgentConfig{}, err
	}
	if config.CodexPath == "" {
		config.CodexPath = "codex"
	}
	if config.PollSeconds <= 0 {
		config.PollSeconds = 1
	}
	if config.MaxActiveSessions <= 0 {
		config.MaxActiveSessions = 1
	}
	return config, nil
}

func WriteAgentConfig(path string, config AgentConfig) error {
	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0600)
}

func PathInAllowedRoots(path string, roots []string) bool {
	if path == "" || len(roots) == 0 {
		return false
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		cleanRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if cleanPath == cleanRoot {
			return true
		}
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
			return true
		}
	}
	return false
}
