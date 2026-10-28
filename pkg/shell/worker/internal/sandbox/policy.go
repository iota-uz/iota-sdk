package sandbox

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/ewhauser/gbash/policy"
)

// runPolicy shares one command budget across the whole run. gbash starts a
// fresh budget for every nested bash/sh and awk system() execution; counting
// here keeps child executions inside the budget of the run.
type runPolicy struct {
	*policy.Static
	maxCommands int64
	commands    atomic.Int64
	exhausted   atomic.Bool
}

func (p *runPolicy) AllowCommand(ctx context.Context, name string, argv []string) error {
	if err := p.spend(); err != nil {
		return err
	}
	return p.Static.AllowCommand(ctx, name, argv)
}

func (p *runPolicy) AllowBuiltin(ctx context.Context, name string, argv []string) error {
	if err := p.spend(); err != nil {
		return err
	}
	return p.Static.AllowBuiltin(ctx, name, argv)
}

func (p *runPolicy) spend() error {
	if p.commands.Add(1) <= p.maxCommands {
		return nil
	}
	p.exhausted.Store(true)
	return &policy.DeniedError{Subject: "command", Reason: fmt.Sprintf("run budget of %d commands exhausted", p.maxCommands)}
}
