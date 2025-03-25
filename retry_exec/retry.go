package main

import (
	"context"
	"os"
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

	r.ExitCode, err = f(ctx)
	log.Ctx(ctx).Infof("exec command finished, commands: %v, exit code: %d", r.commands, r.ExitCode)

	if err == nil && r.ExitCode == 0 {
		return nil
	} else if r.opt.RetryTimes <= 1 {
		log.Ctx(ctx).WithError(err).Errorf("try failed, will not retry")
		r.reportErr(ctx, "retry_exec execute commands=%v failed, exit code=%d err=%+v", r.commands, r.ExitCode, err)
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

func commandExecutor(commands []string) func(ctx context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		cmd := command.NewExecutor(commands[0], commands[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		cmd.Build()
		err := cmd.Exec(ctx)
		if err != nil {
			log.Ctx(ctx).WithError(err).Errorf("exec command failed, commands: %v, failed msg:%s", commands, cmd.ExitMsg())
			return cmd.ExitCode(), err
		}
		if cmd.ExitCode() == 0 {
			log.Ctx(ctx).Infof("exec command success, commands: %v", commands)
		}
		return cmd.ExitCode(), nil
	}
}
