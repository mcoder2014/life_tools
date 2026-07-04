package life_codex

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type StateStore struct {
	mu           sync.Mutex
	path         string
	machines     map[string]Machine
	agentTokens  map[string]string
	enrollTokens map[string]int64
	sessions     map[string]Session
	commands     map[string][]AgentCommand
	audit        *AuditLogger
	subscribers  map[chan struct{}]struct{}
	config       ServerConfig
}

type persistedState struct {
	Machines    map[string]Machine `json:"machines"`
	AgentTokens map[string]string  `json:"agent_tokens"`
	Sessions    map[string]Session `json:"sessions"`
}

func NewStateStore(config ServerConfig) (*StateStore, error) {
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, err
	}
	store := &StateStore{
		path:         filepath.Join(config.DataDir, "state.json"),
		machines:     map[string]Machine{},
		agentTokens:  map[string]string{},
		enrollTokens: map[string]int64{},
		sessions:     map[string]Session{},
		commands:     map[string][]AgentCommand{},
		audit:        NewAuditLogger(config.AuditDir),
		subscribers:  map[chan struct{}]struct{}{},
		config:       config,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *StateStore) CreateEnrollToken() (string, error) {
	token, err := randomID("enroll")
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.enrollTokens[token] = time.Now().Add(15 * time.Minute).Unix()
	s.mu.Unlock()
	s.notify()
	_ = s.audit.Log("enroll_token_created", "", "", "", map[string]any{"token_suffix": suffix(token)})
	return token, nil
}

func (s *StateStore) Enroll(req EnrollRequest) (EnrollResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.enrollTokens[req.Token]
	if !ok || expires < nowUnix() {
		return EnrollResponse{}, fmt.Errorf("invalid enrollment token")
	}
	delete(s.enrollTokens, req.Token)
	machineID, err := randomID("machine")
	if err != nil {
		return EnrollResponse{}, err
	}
	agentToken, err := randomID("agent")
	if err != nil {
		return EnrollResponse{}, err
	}
	if req.MaxActiveSessions <= 0 {
		req.MaxActiveSessions = 1
	}
	s.machines[machineID] = Machine{
		ID:                machineID,
		Name:              req.Name,
		Status:            "online",
		AllowedRoots:      req.AllowedRoots,
		CodexPath:         req.CodexPath,
		MaxActiveSessions: req.MaxActiveSessions,
		LastSeenUnix:      nowUnix(),
	}
	s.agentTokens[machineID] = agentToken
	if err := s.saveLocked(); err != nil {
		return EnrollResponse{}, err
	}
	_ = s.audit.Log("machine_enrolled", machineID, "", "", s.machines[machineID])
	s.notifyLocked()
	return EnrollResponse{MachineID: machineID, Token: agentToken}, nil
}

func (s *StateStore) Poll(req AgentPollRequest) ([]AgentCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validAgentLocked(req.MachineID, req.Token) {
		return nil, fmt.Errorf("invalid agent token")
	}
	machine := s.machines[req.MachineID]
	machine.Status = "online"
	machine.LastSeenUnix = nowUnix()
	machine.Name = first(machine.Name, req.Name)
	machine.AllowedRoots = req.AllowedRoots
	machine.CodexPath = req.CodexPath
	if req.MaxActiveSessions > 0 {
		machine.MaxActiveSessions = req.MaxActiveSessions
	}
	s.machines[req.MachineID] = machine
	commands := s.commands[req.MachineID]
	s.commands[req.MachineID] = nil
	_ = s.saveLocked()
	return commands, nil
}

func (s *StateStore) Report(report AgentReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validAgentLocked(report.MachineID, report.Token) {
		return fmt.Errorf("invalid agent token")
	}
	if report.ThreadID != "" && report.SessionID != "" {
		session := s.sessions[report.SessionID]
		session.ThreadID = report.ThreadID
		session.UpdatedUnix = nowUnix()
		if session.Status == SessionCreating {
			session.Status = SessionIdle
		}
		s.sessions[session.ID] = session
	}
	if report.Event != nil {
		s.appendEventLocked(*report.Event)
		_ = s.audit.Log("agent_event", report.MachineID, report.SessionID, report.CommandID, report.Event)
	}
	if report.CommandID != "" && (report.Status == "completed" || report.Status == "failed") {
		s.finishCommandLocked(report)
	}
	_ = s.saveLocked()
	s.notifyLocked()
	return nil
}

func (s *StateStore) Snapshot() StateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	machines := make([]Machine, 0, len(s.machines))
	for _, machine := range s.machines {
		if nowUnix()-machine.LastSeenUnix > 10 {
			machine.Status = "offline"
		}
		machines = append(machines, machine)
	}
	sort.Slice(machines, func(i, j int) bool { return machines[i].Name < machines[j].Name })
	sessions := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].UpdatedUnix > sessions[j].UpdatedUnix })
	return StateSnapshot{Machines: machines, Sessions: sessions, NowUnix: nowUnix()}
}

func (s *StateStore) CreateSession(machineID string, cwd string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	machine, ok := s.machines[machineID]
	if !ok {
		return Session{}, fmt.Errorf("machine not found")
	}
	if !PathInAllowedRoots(cwd, machine.AllowedRoots) {
		return Session{}, fmt.Errorf("cwd is outside allowed roots")
	}
	id, err := randomID("session")
	if err != nil {
		return Session{}, err
	}
	session := Session{
		ID:          id,
		MachineID:   machineID,
		Title:       "New session",
		CWD:         cwd,
		Status:      SessionCreating,
		CreatedUnix: nowUnix(),
		UpdatedUnix: nowUnix(),
	}
	s.sessions[id] = session
	command := s.baseCommandLocked(machineID, id, CommandCreateSession)
	command.CWD = cwd
	s.commands[machineID] = append(s.commands[machineID], command)
	_ = s.audit.Log("session_create_requested", machineID, id, command.ID, command)
	_ = s.saveLocked()
	s.notifyLocked()
	return session, nil
}

func (s *StateStore) StartTurn(sessionID string, text string, skill *SkillInput, images []ImagePayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	queued := QueuedTurn{ID: mustRandomID("turn"), Text: text, Skill: skill, Images: images, CreatedUnix: nowUnix()}
	if session.ThreadID == "" || session.Active || s.activeCountLocked(session.MachineID) >= s.machineLimitLocked(session.MachineID) {
		session.Queue = append(session.Queue, queued)
		session.Status = SessionQueued
		session.UpdatedUnix = nowUnix()
		s.sessions[sessionID] = session
		_ = s.audit.Log("turn_queued", session.MachineID, sessionID, "", queued)
		_ = s.saveLocked()
		s.notifyLocked()
		return nil
	}
	s.dispatchTurnLocked(session, queued)
	_ = s.saveLocked()
	s.notifyLocked()
	return nil
}

func (s *StateStore) RetryLastTurn(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.Active {
		return fmt.Errorf("session has an active turn")
	}
	if session.Status != SessionFailed {
		return fmt.Errorf("session is not failed")
	}
	if session.ThreadID == "" {
		return fmt.Errorf("session is not ready")
	}
	lastUserIndex := -1
	for i := len(session.Events) - 1; i >= 0; i-- {
		if session.Events[i].Type == EventUser && strings.TrimSpace(session.Events[i].Text) != "" {
			lastUserIndex = i
			break
		}
	}
	if lastUserIndex < 0 {
		return fmt.Errorf("no text turn to retry")
	}
	for _, event := range session.Events[lastUserIndex+1:] {
		if event.Type == EventImage {
			return fmt.Errorf("retry cannot reuse image attachments; paste images again")
		}
	}
	turn := QueuedTurn{ID: mustRandomID("turn"), Text: session.Events[lastUserIndex].Text, CreatedUnix: nowUnix()}
	session.LastError = ""
	if s.activeCountLocked(session.MachineID) >= s.machineLimitLocked(session.MachineID) {
		session.Queue = append(session.Queue, turn)
		session.Status = SessionQueued
		session.UpdatedUnix = nowUnix()
		s.sessions[sessionID] = session
		_ = s.audit.Log("turn_retry_queued", session.MachineID, sessionID, "", turn)
		_ = s.saveLocked()
		s.notifyLocked()
		return nil
	}
	s.dispatchTurnLocked(session, turn)
	_ = s.audit.Log("turn_retry_started", session.MachineID, sessionID, "", turn)
	_ = s.saveLocked()
	s.notifyLocked()
	return nil
}

func (s *StateStore) ForkSession(sourceSessionID string, text string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source, ok := s.sessions[sourceSessionID]
	if !ok {
		return Session{}, fmt.Errorf("source session not found")
	}
	if source.ThreadID == "" {
		return Session{}, fmt.Errorf("source session is not ready")
	}
	id, err := randomID("session")
	if err != nil {
		return Session{}, err
	}
	fork := Session{
		ID:           id,
		MachineID:    source.MachineID,
		Title:        "BTW from " + source.Title,
		CWD:          source.CWD,
		Status:       SessionQueued,
		ForkedFromID: source.ID,
		PendingText:  text,
		CreatedUnix:  nowUnix(),
		UpdatedUnix:  nowUnix(),
	}
	s.sessions[id] = fork
	if s.activeCountLocked(source.MachineID) >= s.machineLimitLocked(source.MachineID) {
		_ = s.audit.Log("session_fork_queued", source.MachineID, id, "", fork)
		_ = s.saveLocked()
		s.notifyLocked()
		return fork, nil
	}
	s.dispatchForkLocked(fork)
	_ = s.saveLocked()
	s.notifyLocked()
	return s.sessions[id], nil
}

func (s *StateStore) dispatchForkLocked(session Session) {
	session.Active = true
	session.Status = SessionRunning
	session.UpdatedUnix = nowUnix()
	s.sessions[session.ID] = session
	source := s.sessions[session.ForkedFromID]
	command := s.baseCommandLocked(source.MachineID, session.ID, CommandForkSession)
	command.SourceSessionID = source.ID
	command.ThreadID = source.ThreadID
	command.CWD = source.CWD
	command.Text = session.PendingText
	s.commands[source.MachineID] = append(s.commands[source.MachineID], command)
	_ = s.audit.Log("session_fork_requested", source.MachineID, session.ID, command.ID, command)
	s.appendEventLocked(Event{
		ID:          mustRandomID("event"),
		SessionID:   session.ID,
		MachineID:   session.MachineID,
		Type:        EventUser,
		Text:        session.PendingText,
		CreatedUnix: nowUnix(),
	})
}

func (s *StateStore) Subscribe() (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		close(ch)
		s.mu.Unlock()
	}
}

func (s *StateStore) ClearAudit(before time.Time) (int, error) {
	removed, err := ClearAuditBefore(s.config.AuditDir, before)
	if err == nil {
		_ = s.audit.Log("audit_cleared", "", "", "", map[string]any{"before": before.Format(time.RFC3339), "removed": removed})
	}
	return removed, err
}

func (s *StateStore) dispatchTurnLocked(session Session, turn QueuedTurn) {
	session.Active = true
	session.Status = SessionRunning
	session.UpdatedUnix = nowUnix()
	s.sessions[session.ID] = session
	command := s.baseCommandLocked(session.MachineID, session.ID, CommandStartTurn)
	command.ThreadID = session.ThreadID
	command.Text = turn.Text
	command.Skill = turn.Skill
	command.Images = turn.Images
	s.commands[session.MachineID] = append(s.commands[session.MachineID], command)
	s.appendEventLocked(Event{
		ID:          mustRandomID("event"),
		SessionID:   session.ID,
		MachineID:   session.MachineID,
		Type:        EventUser,
		Text:        turn.Text,
		CreatedUnix: nowUnix(),
	})
	_ = s.audit.Log("turn_started", session.MachineID, session.ID, command.ID, command)
}

func (s *StateStore) finishCommandLocked(report AgentReport) {
	session, ok := s.sessions[report.SessionID]
	if !ok {
		return
	}
	session.Active = false
	session.UpdatedUnix = nowUnix()
	if report.Status == "failed" {
		session.Status = SessionFailed
		session.LastError = report.Error
		event := Event{
			ID:          mustRandomID("event"),
			SessionID:   session.ID,
			MachineID:   session.MachineID,
			Type:        EventError,
			Text:        report.Error,
			CreatedUnix: nowUnix(),
		}
		session.Events = append(session.Events, event)
		if len(session.Events) > 500 {
			session.Events = session.Events[len(session.Events)-500:]
		}
	} else {
		if len(session.Queue) > 0 {
			session.Status = SessionQueued
		} else {
			session.Status = SessionIdle
		}
		session.LastError = ""
	}
	s.sessions[session.ID] = session
	_ = s.audit.Log("command_finished", report.MachineID, report.SessionID, report.CommandID, report)
	s.dispatchQueuedMachineLocked(report.MachineID)
}

func (s *StateStore) dispatchQueuedMachineLocked(machineID string) {
	for s.activeCountLocked(machineID) < s.machineLimitLocked(machineID) {
		var selected *Session
		for _, session := range s.sessions {
			if session.MachineID != machineID || session.Active || session.Status != SessionQueued {
				continue
			}
			if session.ForkedFromID != "" && session.ThreadID == "" && session.PendingText != "" {
				copySession := session
				selected = &copySession
				break
			}
			if session.ThreadID != "" && len(session.Queue) > 0 {
				copySession := session
				selected = &copySession
				break
			}
		}
		if selected == nil {
			return
		}
		if selected.ForkedFromID != "" && selected.ThreadID == "" && selected.PendingText != "" {
			s.dispatchForkLocked(*selected)
			continue
		}
		next := selected.Queue[0]
		selected.Queue = selected.Queue[1:]
		s.sessions[selected.ID] = *selected
		s.dispatchTurnLocked(*selected, next)
	}
}

func (s *StateStore) appendEventLocked(event Event) {
	session := s.sessions[event.SessionID]
	if event.ID == "" {
		event.ID = mustRandomID("event")
	}
	if event.CreatedUnix == 0 {
		event.CreatedUnix = nowUnix()
	}
	session.Events = append(session.Events, event)
	if len(session.Events) > 500 {
		session.Events = session.Events[len(session.Events)-500:]
	}
	if session.Title == "New session" && event.Type == EventUser && event.Text != "" {
		session.Title = trimTitle(event.Text)
	}
	session.UpdatedUnix = nowUnix()
	s.sessions[session.ID] = session
}

func (s *StateStore) baseCommandLocked(machineID string, sessionID string, typ string) AgentCommand {
	return AgentCommand{
		ID:                mustRandomID("cmd"),
		Type:              typ,
		SessionID:         sessionID,
		ApprovalPolicy:    s.config.DefaultApprovalPolicy,
		ApprovalsReviewer: s.config.DefaultApprovalsReviewer,
		CreatedUnix:       nowUnix(),
	}
}

func (s *StateStore) activeCountLocked(machineID string) int {
	count := 0
	for _, session := range s.sessions {
		if session.MachineID == machineID && session.Active {
			count++
		}
	}
	return count
}

func (s *StateStore) machineLimitLocked(machineID string) int {
	limit := s.machines[machineID].MaxActiveSessions
	if limit <= 0 {
		return 1
	}
	return limit
}

func (s *StateStore) validAgentLocked(machineID string, token string) bool {
	return machineID != "" && token != "" && s.agentTokens[machineID] == token
}

func (s *StateStore) load() error {
	content, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var data persistedState
	if err := json.Unmarshal(content, &data); err != nil {
		return err
	}
	if data.Machines != nil {
		s.machines = data.Machines
	}
	if data.AgentTokens != nil {
		s.agentTokens = data.AgentTokens
	}
	if data.Sessions != nil {
		s.sessions = data.Sessions
	}
	return nil
}

func (s *StateStore) saveLocked() error {
	content, err := json.MarshalIndent(persistedState{
		Machines:    s.machines,
		AgentTokens: s.agentTokens,
		Sessions:    s.sessions,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(content, '\n'), 0600)
}

func (s *StateStore) notify() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifyLocked()
}

func (s *StateStore) notifyLocked() {
	for ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func randomID(prefix string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf[:]), nil
}

func mustRandomID(prefix string) string {
	id, err := randomID(prefix)
	if err != nil {
		panic(err)
	}
	return id
}

func trimTitle(value string) string {
	runes := []rune(value)
	if len(runes) <= 64 {
		return value
	}
	return string(runes[:64])
}

func suffix(value string) string {
	if len(value) <= 6 {
		return value
	}
	return value[len(value)-6:]
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
