package life_codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type AppServerClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	waiters   map[int]chan rpcMessage
	subs      map[chan rpcMessage]struct{}
	nextID    int
	writeMu   sync.Mutex
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewAppServerClient(ctx context.Context, codexPath string) (*AppServerClient, error) {
	if codexPath == "" {
		codexPath = "codex"
	}
	cmd := exec.CommandContext(ctx, codexPath, "app-server", "--stdio")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	client := &AppServerClient{
		cmd:     cmd,
		stdin:   stdin,
		waiters: map[int]chan rpcMessage{},
		subs:    map[chan rpcMessage]struct{}{},
		nextID:  1,
	}
	go client.readStdout(stdout)
	go client.drainStderr(stderr)
	initCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := client.Request(initCtx, "initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "life_codex_agent", "version": "0.1"},
		"capabilities": map[string]any{},
	}); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *AppServerClient) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		for id, ch := range c.waiters {
			delete(c.waiters, id)
			close(ch)
		}
		for ch := range c.subs {
			delete(c.subs, ch)
			close(ch)
		}
		c.mu.Unlock()
		if c.stdin != nil {
			err = c.stdin.Close()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			_ = c.cmd.Wait()
		}
	})
	return err
}

func (c *AppServerClient) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	content, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	id, ch, err := c.nextRequest()
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  json.RawMessage(content),
	}
	line, err := json.Marshal(req)
	if err != nil {
		c.removeWaiter(id)
		return nil, err
	}
	c.writeMu.Lock()
	_, err = c.stdin.Write(append(line, '\n'))
	c.writeMu.Unlock()
	if err != nil {
		c.removeWaiter(id)
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.removeWaiter(id)
		return nil, ctx.Err()
	case msg, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("codex app-server closed")
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("codex rpc %s failed: %s", method, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func (c *AppServerClient) StartThread(ctx context.Context, cwd string, approvalPolicy string, approvalsReviewer string) (string, error) {
	params := map[string]any{
		"cwd":               cwd,
		"approvalPolicy":    approvalPolicy,
		"approvalsReviewer": approvalsReviewer,
		"sandbox":           "workspace-write",
		"config": map[string]any{
			"features": map[string]any{
				"multi_agent": false,
				"multi_agent_v2": map[string]any{
					"enabled": false,
				},
			},
		},
	}
	result, err := c.Request(ctx, "thread/start", params)
	if err != nil {
		return "", err
	}
	return extractThreadID(result)
}

func (c *AppServerClient) ForkThread(ctx context.Context, sourceThreadID string, cwd string, approvalPolicy string, approvalsReviewer string) (string, error) {
	params := map[string]any{
		"threadId":          sourceThreadID,
		"cwd":               cwd,
		"approvalPolicy":    approvalPolicy,
		"approvalsReviewer": approvalsReviewer,
		"sandbox":           "workspace-write",
	}
	result, err := c.Request(ctx, "thread/fork", params)
	if err == nil {
		return extractThreadID(result)
	}
	return c.StartThread(ctx, cwd, approvalPolicy, approvalsReviewer)
}

func (c *AppServerClient) StartTurn(ctx context.Context, sessionID string, machineID string, threadID string, cwd string, text string, skill *SkillInput, images []ImagePayload, approvalPolicy string, approvalsReviewer string, onEvent func(Event)) error {
	sub := c.subscribe()
	defer c.unsubscribe(sub)
	input := buildTurnInput(text, skill, images)
	result, err := c.Request(ctx, "turn/start", map[string]any{
		"threadId":          threadID,
		"input":             input,
		"cwd":               cwd,
		"approvalPolicy":    approvalPolicy,
		"approvalsReviewer": approvalsReviewer,
		"sandbox":           "workspace-write",
	})
	if err != nil {
		return err
	}
	turnID := extractTurnID(result)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-sub:
			if !ok {
				return fmt.Errorf("codex app-server closed")
			}
			if event := codexNotificationEvent(sessionID, machineID, msg); event != nil && onEvent != nil {
				onEvent(*event)
			}
			if msg.Method == "turn/completed" && notificationThreadID(msg.Params) == threadID {
				if turnID == "" || notificationTurnID(msg.Params) == "" || notificationTurnID(msg.Params) == turnID {
					if status := notificationTurnStatus(msg.Params); status != "" && status != "completed" {
						if message := notificationTurnError(msg.Params); message != "" {
							return fmt.Errorf("turn %s: %s", status, message)
						}
						return fmt.Errorf("turn %s", status)
					}
					return nil
				}
			}
			if msg.Method == "turn/aborted" && notificationThreadID(msg.Params) == threadID {
				return fmt.Errorf("turn aborted")
			}
		}
	}
}

func (c *AppServerClient) nextRequest() (int, chan rpcMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, nil, fmt.Errorf("codex app-server closed")
	}
	id := c.nextID
	c.nextID++
	ch := make(chan rpcMessage, 1)
	c.waiters[id] = ch
	return id, ch, nil
}

func (c *AppServerClient) removeWaiter(id int) {
	c.mu.Lock()
	delete(c.waiters, id)
	c.mu.Unlock()
}

func (c *AppServerClient) subscribe() chan rpcMessage {
	ch := make(chan rpcMessage, 64)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch
}

func (c *AppServerClient) unsubscribe(ch chan rpcMessage) {
	c.mu.Lock()
	if _, ok := c.subs[ch]; ok {
		delete(c.subs, ch)
		close(ch)
	}
	c.mu.Unlock()
}

func (c *AppServerClient) readStdout(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.Method != "" && len(msg.ID) > 0 {
			c.replyToServerRequest(msg)
			continue
		}
		if msg.Method != "" {
			c.publish(msg)
			continue
		}
		if len(msg.ID) > 0 {
			var id int
			if err := json.Unmarshal(msg.ID, &id); err != nil {
				continue
			}
			c.mu.Lock()
			ch := c.waiters[id]
			delete(c.waiters, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- msg
				close(ch)
			}
		}
	}
	_ = c.Close()
}

func (c *AppServerClient) drainStderr(stderr io.Reader) {
	_, _ = io.Copy(io.Discard, stderr)
}

func (c *AppServerClient) publish(msg rpcMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ch := range c.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (c *AppServerClient) replyToServerRequest(msg rpcMessage) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result":  map[string]any{"action": "decline"},
	}
	line, err := json.Marshal(resp)
	if err != nil {
		return
	}
	c.writeMu.Lock()
	_, _ = c.stdin.Write(append(line, '\n'))
	c.writeMu.Unlock()
}

func buildTurnInput(text string, skill *SkillInput, images []ImagePayload) []map[string]any {
	if skill != nil && skill.Name != "" {
		if skill.Path != "" {
			text = fmt.Sprintf("[$%s](%s)\n%s", skill.Name, skill.Path, text)
		} else {
			text = fmt.Sprintf("[$%s]\n%s", skill.Name, text)
		}
	}
	input := []map[string]any{{"type": "text", "text": text, "text_elements": []any{}}}
	for _, image := range images {
		if image.LocalPath == "" {
			continue
		}
		input = append(input, map[string]any{"type": "localImage", "path": image.LocalPath})
	}
	return input
}

func extractThreadID(result json.RawMessage) (string, error) {
	var decoded struct {
		Thread struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionId"`
		} `json:"thread"`
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return "", err
	}
	if decoded.Thread.ID != "" {
		return decoded.Thread.ID, nil
	}
	if decoded.Thread.SessionID != "" {
		return decoded.Thread.SessionID, nil
	}
	if decoded.ID != "" {
		return decoded.ID, nil
	}
	return "", fmt.Errorf("codex response did not include a thread id")
}

func extractTurnID(result json.RawMessage) string {
	var decoded struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
		ID string `json:"id"`
	}
	_ = json.Unmarshal(result, &decoded)
	if decoded.Turn.ID != "" {
		return decoded.Turn.ID
	}
	return decoded.ID
}

func codexNotificationEvent(sessionID string, machineID string, msg rpcMessage) *Event {
	switch msg.Method {
	case "item/agentMessage/delta":
		text := notificationString(msg.Params, "delta")
		if text == "" {
			text = notificationString(msg.Params, "text")
		}
		if text == "" {
			return nil
		}
		return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventAssistant, Text: text, Payload: msg.Params, CreatedUnix: nowUnix()}
	case "item/completed":
		item := notificationObject(msg.Params, "item")
		itemType, _ := item["type"].(string)
		switch itemType {
		case "agentMessage":
			text, _ := item["text"].(string)
			if text == "" {
				return nil
			}
			return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventAssistant, Text: text, Payload: msg.Params, CreatedUnix: nowUnix()}
		case "commandExecution", "mcpToolCall", "dynamicToolCall":
			content, _ := json.Marshal(item)
			return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventTool, Text: itemType + " completed", Payload: content, CreatedUnix: nowUnix()}
		}
	case "item/started":
		item := notificationObject(msg.Params, "item")
		itemType, _ := item["type"].(string)
		if itemType == "commandExecution" || itemType == "mcpToolCall" || itemType == "dynamicToolCall" {
			content, _ := json.Marshal(item)
			return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventTool, Text: itemType + " started", Payload: content, CreatedUnix: nowUnix()}
		}
	case "turn/completed":
		return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventInfo, Text: "turn completed", Payload: msg.Params, CreatedUnix: nowUnix()}
	case "turn/aborted":
		return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventError, Text: "turn aborted", Payload: msg.Params, CreatedUnix: nowUnix()}
	case "warning":
		message := notificationString(msg.Params, "message")
		if message == "" {
			message = "warning"
		}
		return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventInfo, Text: message, Payload: msg.Params, CreatedUnix: nowUnix()}
	}
	if strings.HasPrefix(msg.Method, "mcp/") || strings.HasPrefix(msg.Method, "serverRequest/") {
		return &Event{ID: mustRandomID("event"), SessionID: sessionID, MachineID: machineID, Type: EventTool, Text: msg.Method, Payload: msg.Params, CreatedUnix: nowUnix()}
	}
	return nil
}

func notificationThreadID(params json.RawMessage) string {
	value := notificationString(params, "threadId")
	if value != "" {
		return value
	}
	return notificationString(params, "thread_id")
}

func notificationTurnID(params json.RawMessage) string {
	item := notificationObject(params, "turn")
	if id, _ := item["id"].(string); id != "" {
		return id
	}
	return notificationString(params, "turnId")
}

func notificationTurnStatus(params json.RawMessage) string {
	item := notificationObject(params, "turn")
	if status, _ := item["status"].(string); status != "" {
		return status
	}
	return notificationString(params, "status")
}

func notificationTurnError(params json.RawMessage) string {
	item := notificationObject(params, "turn")
	errValue, _ := item["error"].(map[string]any)
	if message, _ := errValue["message"].(string); message != "" {
		return message
	}
	return notificationString(params, "error")
}

func notificationString(params json.RawMessage, key string) string {
	var decoded map[string]any
	if err := json.Unmarshal(params, &decoded); err != nil {
		return ""
	}
	value, _ := decoded[key].(string)
	return value
}

func notificationObject(params json.RawMessage, key string) map[string]any {
	var decoded map[string]any
	if err := json.Unmarshal(params, &decoded); err != nil {
		return nil
	}
	value, _ := decoded[key].(map[string]any)
	return value
}
