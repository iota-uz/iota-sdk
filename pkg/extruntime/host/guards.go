package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opCheckWrite = serrors.Op("extruntime.host.CheckWrite")

// execKey carries the execution serving a host call, so a data store that
// calls back into the host (WriteGuard.CheckWrite) nests its guard runs on the
// worker of the invocation that issued the write instead of taking a new slot.
type execKey struct{}

// guardRun describes one guard phase to run for a target.
type guardRun struct {
	root    *rootState
	parent  *execution
	subject spi.Subject
	trigger Trigger
	// contract is the guarded command/operation; nil for entity write rules.
	contract *manifest.Resource
	target   string
	phase    manifest.GuardPhase
	input    json.RawMessage
	// join is the transaction the guards run in (operation/atomic phase).
	join *txHandle
	// ext is the caller-owned transaction of a write rule.
	ext pgx.Tx
}

// runGuards runs every active guard of the target in stable ID order,
// independent of which implementation is bound to the contract. Validation
// issues of all guards are aggregated and stamped with the guard ID as source;
// any other failure aborts immediately.
func (h *Host) runGuards(ctx context.Context, g guardRun) error {
	guards, err := h.cfg.Catalog.Guards(ctx, g.target, g.phase)
	if err != nil {
		return err
	}
	if len(guards) == 0 {
		return nil
	}
	if h.cfg.Artifacts == nil {
		return fail("runtime.notConfigured", "guards are active for %s but the host has no ArtifactResolver", g.target)
	}
	phase := "write"
	if g.phase == manifest.GuardInput {
		phase = "input"
	}
	var issues []pool.Issue
	for _, rg := range guards {
		art, secrets, err := h.cfg.Artifacts.ResolveArtifact(ctx, rg.Package, rg.Version)
		if err != nil {
			return fail("runtime.guardArtifact", "guard %s: %v", rg.Guard.ID, err)
		}
		_, err = h.execute(ctx, g.root, execSpec{
			export: &Export{
				Kind:     pool.KindGuard,
				Resource: rg.Guard.ID,
				Binding: Binding{
					Contract: rg.Guard.ID, Package: rg.Package, Version: rg.Version, Export: rg.Guard.Export,
					Artifact: art, Secrets: secrets,
				},
			},
			input:      g.input,
			subject:    g.subject,
			trigger:    g.trigger,
			parent:     g.parent,
			join:       g.join,
			externalTx: g.ext,
		})
		if err == nil {
			continue
		}
		we := asWire(err)
		switch {
		case we.Kind == pool.ErrKindValidation:
			if len(we.Issues) == 0 {
				we.Issues = []pool.Issue{{Code: we.Code, Params: we.Params}}
			}
			for _, is := range we.Issues {
				if is.Source == "" {
					is.Source = rg.Guard.ID
				}
				if is.Phase == "" {
					is.Phase = phase
				}
				issues = append(issues, is)
			}
		case we.Kind == pool.ErrKindDomain && g.contract != nil && slices.Contains(g.contract.Errors, we.Code):
			return err
		case we.Kind == pool.ErrKindDomain:
			return fail("runtime.undeclaredDomainError", "guard %s raised %q which %s does not declare", rg.Guard.ID, we.Code, g.target)
		default:
			return err
		}
	}
	if len(issues) == 0 {
		return nil
	}
	return &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.guardRejected", Message: fmt.Sprintf("%d validation issue(s) from guards of %s", len(issues), g.target), Issues: issues}
}

// CheckWrite implements spi.WriteGuard: it runs the entity's writeRule exports
// in the writing transaction. Called by the data store inside a write, so the
// guard sees uncommitted state and an error aborts the write.
func (h *Host) CheckWrite(ctx context.Context, tx pgx.Tx, sc spi.Scope, change spi.WriteChange) error {
	parent, _ := ctx.Value(execKey{}).(*execution)
	var root *rootState
	trigger := Trigger{Kind: TriggerUI}
	if parent != nil {
		root, trigger = parent.root, parent.trigger()
	} else {
		root = &rootState{txs: newTxRegistry(h.cfg.DB)}
		defer root.txs.rollbackAll()
		var cancel context.CancelFunc
		ctx, cancel = h.withDeadline(ctx)
		defer cancel()
	}
	input, err := json.Marshal(struct {
		Entity   string          `json:"entity"`
		Key      string          `json:"key"`
		Op       string          `json:"op"`
		Existing json.RawMessage `json:"existing"`
		Proposed json.RawMessage `json:"proposed"`
		Changed  []string        `json:"changed"`
	}{change.Entity, change.Key, change.Op, nullIfEmptyJSON(change.Existing), nullIfEmptyJSON(change.Proposed), nonNilStrings(change.Changed)})
	if err != nil {
		return serrors.Wrap(opCheckWrite, err)
	}
	err = h.runGuards(ctx, guardRun{
		root: root, parent: parent, subject: sc.Subject, trigger: trigger,
		target: change.Entity, phase: manifest.GuardWrite, input: input, ext: tx,
	})
	if err != nil {
		var we *pool.WireError
		if errors.As(err, &we) {
			return we
		}
		return serrors.Wrap(opCheckWrite, err)
	}
	return nil
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

var _ spi.WriteGuard = (*Host)(nil)
