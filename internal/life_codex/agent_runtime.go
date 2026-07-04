package life_codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type AgentRuntime struct {
	config AgentConfig
	client *http.Client
	codex  *AppServerClient
}

func EnrollAgent(ctx context.Context, configPath string, config AgentConfig, enrollToken string) (EnrollResponse, error) {
	req := EnrollRequest{
		Token:             enrollToken,
		Name:              config.MachineName,
		AllowedRoots:      config.AllowedRoots,
		CodexPath:         config.CodexPath,
		MaxActiveSessions: config.MaxActiveSessions,
	}
	var resp EnrollResponse
	if err := postJSON(ctx, config.ServerURL, "/agent/enroll", req, &resp); err != nil {
		return EnrollResponse{}, err
	}
	config.MachineID = resp.MachineID
	config.AgentToken = resp.Token
	if err := WriteAgentConfig(configPath, config); err != nil {
		return EnrollResponse{}, err
	}
	return resp, nil
}

func RunAgent(ctx context.Context, config AgentConfig) error {
	if config.MachineID == "" || config.AgentToken == "" {
		return fmt.Errorf("agent is not enrolled")
	}
	codex, err := NewAppServerClient(ctx, config.CodexPath)
	if err != nil {
		return err
	}
	defer codex.Close()
	runtime := &AgentRuntime{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
		codex:  codex,
	}
	limit := config.MaxActiveSessions
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		commands, err := runtime.poll(ctx)
		if err != nil {
			time.Sleep(time.Duration(config.PollSeconds) * time.Second)
			continue
		}
		for _, command := range commands {
			cmd := command
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				runtime.execute(ctx, cmd)
			}()
		}
		time.Sleep(time.Duration(config.PollSeconds) * time.Second)
	}
}

func (a *AgentRuntime) poll(ctx context.Context) ([]AgentCommand, error) {
	req := AgentPollRequest{
		MachineID:         a.config.MachineID,
		Token:             a.config.AgentToken,
		Name:              a.config.MachineName,
		AllowedRoots:      a.config.AllowedRoots,
		CodexPath:         a.config.CodexPath,
		MaxActiveSessions: a.config.MaxActiveSessions,
	}
	var resp struct {
		Commands []AgentCommand `json:"commands"`
	}
	if err := a.post(ctx, "/agent/poll", req, &resp); err != nil {
		return nil, err
	}
	return resp.Commands, nil
}

func (a *AgentRuntime) execute(parent context.Context, command AgentCommand) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()
	var err error
	switch command.Type {
	case CommandCreateSession:
		var threadID string
		threadID, err = a.codex.StartThread(ctx, command.CWD, command.ApprovalPolicy, command.ApprovalsReviewer)
		if err == nil {
			a.report(ctx, AgentReport{CommandID: command.ID, SessionID: command.SessionID, ThreadID: threadID, Status: "completed"})
			return
		}
	case CommandStartTurn:
		err = a.executeTurn(ctx, command)
	case CommandForkSession:
		err = a.executeFork(ctx, command)
	default:
		err = fmt.Errorf("unknown command type: %s", command.Type)
	}
	if err != nil {
		a.report(ctx, AgentReport{CommandID: command.ID, SessionID: command.SessionID, Status: "failed", Error: err.Error()})
	}
}

func (a *AgentRuntime) executeFork(ctx context.Context, command AgentCommand) error {
	threadID, err := a.codex.ForkThread(ctx, command.ThreadID, command.CWD, command.ApprovalPolicy, command.ApprovalsReviewer)
	if err != nil {
		return err
	}
	a.report(ctx, AgentReport{SessionID: command.SessionID, ThreadID: threadID})
	command.ThreadID = threadID
	return a.executeTurn(ctx, command)
}

func (a *AgentRuntime) executeTurn(ctx context.Context, command AgentCommand) error {
	images, err := SaveImagePayloads(a.config.AttachmentDir, command.SessionID, command.ID, command.Images)
	if err != nil {
		return err
	}
	if len(images) > 0 {
		a.report(ctx, AgentReport{
			SessionID: command.SessionID,
			CommandID: command.ID,
			Event: &Event{
				ID:          mustRandomID("event"),
				SessionID:   command.SessionID,
				MachineID:   a.config.MachineID,
				Type:        EventImage,
				Text:        fmt.Sprintf("%d image attachment(s) saved on agent", len(images)),
				Payload:     mustMarshalRaw(ImageMetadata(images)),
				CreatedUnix: nowUnix(),
			},
		})
	}
	err = a.codex.StartTurn(ctx, command.SessionID, a.config.MachineID, command.ThreadID, command.CWD, command.Text, command.Skill, images, command.ApprovalPolicy, command.ApprovalsReviewer, func(event Event) {
		a.report(ctx, AgentReport{CommandID: command.ID, SessionID: command.SessionID, Event: &event})
	})
	if err != nil {
		return err
	}
	a.report(ctx, AgentReport{CommandID: command.ID, SessionID: command.SessionID, Status: "completed"})
	return nil
}

func (a *AgentRuntime) report(ctx context.Context, report AgentReport) {
	report.MachineID = a.config.MachineID
	report.Token = a.config.AgentToken
	_ = a.post(ctx, "/agent/report", report, nil)
}

func (a *AgentRuntime) post(ctx context.Context, path string, request any, response any) error {
	return postJSONWithClient(ctx, a.client, a.config.ServerURL, path, request, response)
}

func postJSON(ctx context.Context, baseURL string, path string, request any, response any) error {
	return postJSONWithClient(ctx, &http.Client{Timeout: 60 * time.Second}, baseURL, path, request, response)
}

func postJSONWithClient(ctx context.Context, client *http.Client, baseURL string, path string, request any, response any) error {
	content, err := json.Marshal(request)
	if err != nil {
		return err
	}
	url := strings.TrimRight(baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body.Error == "" {
			body.Error = resp.Status
		}
		return errors.New(body.Error)
	}
	if response == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(response)
}

func mustMarshalRaw(value any) json.RawMessage {
	content, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return content
}
