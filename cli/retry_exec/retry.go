package main

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mcoder2014/go_utils/command"
	"github.com/mcoder2014/go_utils/common"
	"github.com/mcoder2014/go_utils/log"
)

type Retry struct {
	opt      *ExecOption
	commands []string

	ExitCode int
}

const commandOutputTailBytes = 4096

type commandResult struct {
	ExitCode   int
	Err        error
	StdoutTail string
	StderrTail string
}

func NewRetry(commands []string, opt *ExecOption) *Retry {
	return &Retry{
		opt:      opt,
		commands: commands,
	}
}

func (r *Retry) Try(ctx context.Context) (err error) {
	defer common.Recover(ctx, &err)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	f := commandExecutor(r.commands)

	result := f(ctx)
	r.ExitCode, err = result.ExitCode, result.Err
	log.Ctx(ctx).Infof("exec command finished, commands: %v, exit code: %d", r.commands, r.ExitCode)

	if err == nil && r.ExitCode == 0 {
		return nil
	} else if r.opt.RetryTimes <= 1 {
		log.Ctx(ctx).WithError(err).Errorf("try failed, will not retry")
		r.reportErr(ctx, "retry_exec execute commands=%v failed, exit code=%d err=%+v%s",
			r.commands, r.ExitCode, err, commandOutputTail(result))
		return err
	}

	log.Ctx(ctx).Infof("try failed, sleep %d ms, remain %d times to try", r.opt.RetryInterval, r.opt.RetryTimes)
	time.Sleep(time.Duration(r.opt.RetryInterval) * time.Millisecond)
	r.opt.RetryTimes -= 1
	if (r.opt.RetryInterval << 1) <= r.opt.MaxRetryInterval {
		r.opt.RetryInterval = r.opt.RetryInterval << 1
	}

	return r.Try(ctx)
}

func commandExecutor(commands []string) func(ctx context.Context) commandResult {
	return commandExecutorWithIO(commands, os.Stdin, os.Stdout, os.Stderr)
}

func commandExecutorWithIO(commands []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) func(ctx context.Context) commandResult {
	return func(ctx context.Context) commandResult {
		stdoutTail := newTailWriter(commandOutputTailBytes)
		stderrTail := newTailWriter(commandOutputTailBytes)
		cmd := command.NewExecutor(commands[0], commands[1:]...)
		cmd.Stdout = io.MultiWriter(stdout, stdoutTail)
		cmd.Stderr = io.MultiWriter(stderr, stderrTail)
		cmd.Stdin = stdin

		cmd.Build()
		err := cmd.Exec(ctx)
		if err != nil {
			log.Ctx(ctx).WithError(err).Errorf("exec command failed, commands: %v, failed msg:%s", commands, cmd.ExitMsg())
			return commandResult{
				ExitCode:   cmd.ExitCode(),
				Err:        err,
				StdoutTail: stdoutTail.String(),
				StderrTail: stderrTail.String(),
			}
		}
		if cmd.ExitCode() == 0 {
			log.Ctx(ctx).Infof("exec command success, commands: %v", commands)
		}
		return commandResult{
			ExitCode:   cmd.ExitCode(),
			StdoutTail: stdoutTail.String(),
			StderrTail: stderrTail.String(),
		}
	}
}

func commandOutputTail(result commandResult) string {
	stderrTail := strings.TrimSpace(result.StderrTail)
	if stderrTail != "" {
		return "\n\nstderr tail:\n" + stderrTail
	}

	stdoutTail := strings.TrimSpace(result.StdoutTail)
	if stdoutTail != "" {
		return "\n\nstdout tail:\n" + stdoutTail
	}

	return ""
}
