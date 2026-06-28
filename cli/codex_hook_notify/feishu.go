package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Sender interface {
	Send(ctx context.Context, url string, text string) error
}

type SenderFunc func(ctx context.Context, url string, text string) error

func (f SenderFunc) Send(ctx context.Context, url string, text string) error {
	return f(ctx, url, text)
}

type FeishuSender struct {
	Client *http.Client
}

func (s FeishuSender) Send(ctx context.Context, url string, text string) error {
	body, err := json.Marshal(map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": text,
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read feishu response failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send feishu message failed, status code: %d, body: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
