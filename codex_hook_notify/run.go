package main

import (
	"context"
	"fmt"
	"io"
)

type RunOption struct {
	Config Config
	Input  io.Reader
	Stderr io.Writer
	Sender Sender
	Logger Logger
}

func Run(ctx context.Context, opt RunOption) error {
	input, err := io.ReadAll(opt.Input)
	if err != nil {
		report(opt, "", "read hook input failed: %v", err)
		return nil
	}

	event, err := ParseHookEvent(input)
	if err != nil {
		report(opt, "", "parse hook input failed: %v", err)
		return nil
	}

	urls := opt.Config.MatchingURLs(event.HookEventName)
	if len(urls) == 0 {
		return nil
	}

	text := BuildMessage(event)
	sent := 0
	for _, url := range urls {
		if err := opt.Sender.Send(ctx, url, text); err != nil {
			report(opt, event.SessionID, "send feishu message failed: %v", err)
			continue
		}
		sent++
	}
	if sent > 0 && opt.Logger != nil {
		if err := opt.Logger.Log(event.SessionID, fmt.Sprintf("event %s sent to %d feishu webhook(s)", event.HookEventName, sent)); err != nil && opt.Stderr != nil {
			fmt.Fprintf(opt.Stderr, "codex_hook_notify: write log failed: %v\n", err)
		}
	}
	return nil
}

func report(opt RunOption, sessionID string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if opt.Stderr != nil {
		fmt.Fprintf(opt.Stderr, "codex_hook_notify: %s\n", message)
	}
	if opt.Logger != nil {
		if err := opt.Logger.Log(sessionID, message); err != nil && opt.Stderr != nil {
			fmt.Fprintf(opt.Stderr, "codex_hook_notify: write log failed: %v\n", err)
		}
	}
}
