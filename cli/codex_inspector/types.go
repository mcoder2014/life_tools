package main

type SourceStatus struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Exists     bool     `json:"exists"`
	Size       int64    `json:"size"`
	ModifiedAt string   `json:"modifiedAt,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}

type QueryFilter struct {
	From    string
	To      string
	Query   string
	Limit   int
	hasFrom bool
	hasTo   bool
}

type SessionSummary struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Path              string     `json:"path,omitempty"`
	Cwd               string     `json:"cwd,omitempty"`
	Model             string     `json:"model,omitempty"`
	ModelProvider     string     `json:"modelProvider,omitempty"`
	StartedAt         string     `json:"startedAt,omitempty"`
	UpdatedAt         string     `json:"updatedAt,omitempty"`
	EventCount        int        `json:"eventCount"`
	UserMessages      int        `json:"userMessages"`
	AssistantMessages int        `json:"assistantMessages"`
	ToolCalls         int        `json:"toolCalls"`
	ToolResults       int        `json:"toolResults"`
	SystemEvents      int        `json:"systemEvents"`
	TokenStats        TokenStats `json:"tokenStats"`
	Preview           string     `json:"preview,omitempty"`
	Warnings          []string   `json:"warnings,omitempty"`
	Enriched          bool       `json:"-"`
}

type TokenUsage struct {
	InputTokens           int64 `json:"inputTokens"`
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	TotalTokens           int64 `json:"totalTokens"`
}

type RateLimitUsage struct {
	UsedPercent   float64 `json:"usedPercent,omitempty"`
	WindowMinutes int64   `json:"windowMinutes,omitempty"`
}

type TokenStats struct {
	Total              TokenUsage     `json:"total"`
	Last               TokenUsage     `json:"last"`
	TokenEvents        int            `json:"tokenEvents"`
	ModelContextWindow int64          `json:"modelContextWindow,omitempty"`
	PrimaryRateLimit   RateLimitUsage `json:"primaryRateLimit,omitempty"`
	SecondaryRateLimit RateLimitUsage `json:"secondaryRateLimit,omitempty"`
}

type DisplayEvent struct {
	Line      int    `json:"line"`
	Timestamp string `json:"timestamp,omitempty"`
	Kind      string `json:"kind"`
	Role      string `json:"role,omitempty"`
	Label     string `json:"label"`
	Text      string `json:"text,omitempty"`
}

type RawLine struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

type SessionDetail struct {
	Summary  SessionSummary `json:"summary"`
	Events   []DisplayEvent `json:"events"`
	RawLines []RawLine      `json:"rawLines"`
	Warnings []string       `json:"warnings,omitempty"`
}

type CountPoint struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type HeatmapCell struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type OverviewResponse struct {
	CodexHome      string           `json:"codexHome"`
	PrivacyNotice  string           `json:"privacyNotice"`
	Sources        []SourceStatus   `json:"sources"`
	Stats          OverviewStats    `json:"stats"`
	RecentSessions []SessionSummary `json:"recentSessions"`
	Warnings       []string         `json:"warnings,omitempty"`
}

type OverviewStats struct {
	TotalSessions     int           `json:"totalSessions"`
	TotalEvents       int           `json:"totalEvents"`
	ActiveDays        int           `json:"activeDays"`
	UserMessages      int           `json:"userMessages"`
	AssistantMessages int           `json:"assistantMessages"`
	ToolCalls         int           `json:"toolCalls"`
	ToolResults       int           `json:"toolResults"`
	TokenizedSessions int           `json:"tokenizedSessions"`
	InputTokens       int64         `json:"inputTokens"`
	CachedInputTokens int64         `json:"cachedInputTokens"`
	OutputTokens      int64         `json:"outputTokens"`
	ReasoningTokens   int64         `json:"reasoningTokens"`
	TotalTokens       int64         `json:"totalTokens"`
	ByDay             []CountPoint  `json:"byDay"`
	ByWeek            []CountPoint  `json:"byWeek"`
	ByMonth           []CountPoint  `json:"byMonth"`
	Heatmap           []HeatmapCell `json:"heatmap"`
}

type SessionsResponse struct {
	Sessions []SessionSummary `json:"sessions"`
	Warnings []string         `json:"warnings,omitempty"`
}

type MemoryFile struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
	Title      string `json:"title"`
	Preview    string `json:"preview,omitempty"`
	Matches    int    `json:"matches,omitempty"`
}

type MemoryResponse struct {
	Files    []MemoryFile `json:"files"`
	Warnings []string     `json:"warnings,omitempty"`
}

type MemoryDetail struct {
	File     MemoryFile `json:"file"`
	Content  string     `json:"content"`
	Warnings []string   `json:"warnings,omitempty"`
}

type DiagnosticsResponse struct {
	CodexHome string         `json:"codexHome"`
	Sources   []SourceStatus `json:"sources"`
	Schemas   []SQLiteSchema `json:"schemas"`
	Cache     CacheStatus    `json:"cache"`
	Warnings  []string       `json:"warnings,omitempty"`
}

type SQLiteSchema struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Loaded   bool   `json:"loaded"`
	Schema   string `json:"schema,omitempty"`
	Error    string `json:"error,omitempty"`
	ReadMode string `json:"readMode"`
}

type CacheStatus struct {
	Status      string        `json:"status"`
	Path        string        `json:"path,omitempty"`
	Workers     int           `json:"workers"`
	Reason      string        `json:"reason,omitempty"`
	BackupPath  string        `json:"backupPath,omitempty"`
	CanBuild    bool          `json:"canBuild"`
	CanRebuild  bool          `json:"canRebuild"`
	AutoFill    bool          `json:"autoFill"`
	GeneratedAt string        `json:"generatedAt"`
	Job         CacheJobState `json:"job"`
}

type CacheJobState struct {
	Running    bool   `json:"running"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Cached     int    `json:"cached"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	LastError  string `json:"lastError,omitempty"`
}
