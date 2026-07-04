package life_codex

import (
	"testing"
)

func TestStateStoreQueuesSingleSessionTurns(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t)
	machineID, token := enrollTestMachine(t, store, root, 1)
	session, err := store.CreateSession(machineID, root)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	commands, err := store.Poll(AgentPollRequest{MachineID: machineID, Token: token, AllowedRoots: []string{root}, MaxActiveSessions: 1})
	if err != nil {
		t.Fatalf("poll create: %v", err)
	}
	if len(commands) != 1 || commands[0].Type != CommandCreateSession {
		t.Fatalf("unexpected create commands: %+v", commands)
	}
	if err := store.Report(AgentReport{MachineID: machineID, Token: token, CommandID: commands[0].ID, SessionID: session.ID, ThreadID: "thread-1", Status: "completed"}); err != nil {
		t.Fatalf("report create: %v", err)
	}
	if err := store.StartTurn(session.ID, "first", nil, nil); err != nil {
		t.Fatalf("start first: %v", err)
	}
	if err := store.StartTurn(session.ID, "second", nil, nil); err != nil {
		t.Fatalf("start second: %v", err)
	}
	snapshot := store.Snapshot()
	got := findSession(snapshot.Sessions, session.ID)
	if got == nil || !got.Active || len(got.Queue) != 1 {
		t.Fatalf("turn was not queued: %+v", got)
	}
	commands, err = store.Poll(AgentPollRequest{MachineID: machineID, Token: token, AllowedRoots: []string{root}, MaxActiveSessions: 1})
	if err != nil {
		t.Fatalf("poll turn: %v", err)
	}
	if len(commands) != 1 || commands[0].Text != "first" {
		t.Fatalf("unexpected first command: %+v", commands)
	}
	if err := store.Report(AgentReport{MachineID: machineID, Token: token, CommandID: commands[0].ID, SessionID: session.ID, Status: "completed"}); err != nil {
		t.Fatalf("report first: %v", err)
	}
	commands, err = store.Poll(AgentPollRequest{MachineID: machineID, Token: token, AllowedRoots: []string{root}, MaxActiveSessions: 1})
	if err != nil {
		t.Fatalf("poll second: %v", err)
	}
	if len(commands) != 1 || commands[0].Text != "second" {
		t.Fatalf("unexpected second command: %+v", commands)
	}
}

func TestForkQueuesWhenMachineLimitReached(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t)
	machineID, token := enrollTestMachine(t, store, root, 1)
	session, err := store.CreateSession(machineID, root)
	if err != nil {
		t.Fatal(err)
	}
	commands, _ := store.Poll(AgentPollRequest{MachineID: machineID, Token: token, AllowedRoots: []string{root}, MaxActiveSessions: 1})
	if err := store.Report(AgentReport{MachineID: machineID, Token: token, CommandID: commands[0].ID, SessionID: session.ID, ThreadID: "thread-1", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	if err := store.StartTurn(session.ID, "long", nil, nil); err != nil {
		t.Fatal(err)
	}
	commands, _ = store.Poll(AgentPollRequest{MachineID: machineID, Token: token, AllowedRoots: []string{root}, MaxActiveSessions: 1})
	fork, err := store.ForkSession(session.ID, "quick")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	got := findSession(store.Snapshot().Sessions, fork.ID)
	if got == nil || got.Status != SessionQueued || got.Active {
		t.Fatalf("fork should be queued: %+v", got)
	}
	if err := store.Report(AgentReport{MachineID: machineID, Token: token, CommandID: commands[0].ID, SessionID: session.ID, Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	commands, _ = store.Poll(AgentPollRequest{MachineID: machineID, Token: token, AllowedRoots: []string{root}, MaxActiveSessions: 1})
	if len(commands) != 1 || commands[0].Type != CommandForkSession || commands[0].Text != "quick" {
		t.Fatalf("unexpected fork command: %+v", commands)
	}
}

func newTestStore(t *testing.T) *StateStore {
	t.Helper()
	config := DefaultServerConfig()
	config.AdminToken = "test"
	config.DataDir = t.TempDir()
	config.AuditDir = t.TempDir()
	store, err := NewStateStore(config)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func enrollTestMachine(t *testing.T, store *StateStore, root string, limit int) (string, string) {
	t.Helper()
	enrollToken, err := store.CreateEnrollToken()
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	resp, err := store.Enroll(EnrollRequest{Token: enrollToken, Name: "local", AllowedRoots: []string{root}, CodexPath: "codex", MaxActiveSessions: limit})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	return resp.MachineID, resp.Token
}

func findSession(sessions []Session, id string) *Session {
	for _, session := range sessions {
		if session.ID == id {
			copySession := session
			return &copySession
		}
	}
	return nil
}
