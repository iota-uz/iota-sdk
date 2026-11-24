package graphs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
)

// maxStateLimit bounds a page of the memory's keys.
const maxStateLimit = 100

func (c *Controller) memory() (engine.Memory, error) {
	if c.cfg.Engine.Memory == nil {
		return nil, rpcError(ErrUnavailable, "the memory of automations is not available", nil)
	}
	return c.cfg.Engine.Memory, nil
}

func stateEntryOf(e state.Entry) StateEntry {
	out := StateEntry{Key: e.Key, Value: e.Value, UpdatedAt: e.UpdatedAt.UTC().Format(time.RFC3339)}
	if e.ExpiresAt != nil {
		s := e.ExpiresAt.UTC().Format(time.RFC3339)
		out.ExpiresAt = &s
	}
	if e.UpdatedByRun != nil {
		s := e.UpdatedByRun.String()
		out.UpdatedByRun = &s
	}
	return out
}

// stateErr is a refusal of the memory (a key, a value, a lifetime or a number of keys over the limits)
// as an invalid input with the engine's message; any other error as mapErr maps it.
func stateErr(err error) error {
	if errors.Is(err, engine.ErrInvalidInput) || errors.Is(err, engine.ErrLimitExceeded) {
		msg := err.Error()
		for _, prefix := range []string{engine.ErrInvalidInput.Error() + ": ", engine.ErrLimitExceeded.Error() + ": "} {
			msg = strings.TrimPrefix(msg, prefix)
		}
		return rpcError(ErrInvalidInput, msg, nil)
	}
	if errors.Is(err, engine.ErrNotFound) {
		return solid.NewRPCError(solid.CodeNotFound, "the key is not found", map[string]string{"code": solid.CodeNotFound})
	}
	return mapErr(err)
}

func (c *Controller) stateList(ctx context.Context, req StateListRequest) (StateListResponse, error) {
	a, err := actor(ctx)
	if err != nil {
		return StateListResponse{}, err
	}
	mem, err := c.memory()
	if err != nil {
		return StateListResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return StateListResponse{}, err
	}
	limit := req.Limit
	if limit <= 0 || limit > maxStateLimit {
		limit = maxStateLimit
	}
	page, err := mem.List(ctx, a, id, state.Filter{Search: req.Search, After: req.Cursor, Limit: limit})
	if err != nil {
		return StateListResponse{}, stateErr(err)
	}
	out := StateListResponse{Items: make([]StateEntry, 0, len(page.Items)), Total: page.Total}
	for _, e := range page.Items {
		out.Items = append(out.Items, stateEntryOf(e))
	}
	if page.Next != "" {
		out.NextCursor = &page.Next
	}
	return out, nil
}

func (c *Controller) stateSet(ctx context.Context, req StateSetRequest) (StateSetResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return StateSetResponse{}, err
	}
	mem, err := c.memory()
	if err != nil {
		return StateSetResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return StateSetResponse{}, err
	}
	var ttl time.Duration
	if req.TTL != "" {
		ms, ok := graph.DurationMs(req.TTL)
		if !ok {
			return StateSetResponse{}, rpcError(ErrInvalidInput, "the lifetime is a duration like 20m, 2h or 7d", nil)
		}
		ttl = time.Duration(ms) * time.Millisecond
	}
	e, err := mem.Set(ctx, a, id, req.Key, req.Value, ttl)
	if err != nil {
		return StateSetResponse{}, stateErr(err)
	}
	return StateSetResponse{Entry: stateEntryOf(e)}, nil
}

func (c *Controller) stateDelete(ctx context.Context, req StateKeyRequest) (OKResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return OKResponse{}, err
	}
	mem, err := c.memory()
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := mem.Delete(ctx, a, id, req.Key); err != nil {
		return OKResponse{}, stateErr(err)
	}
	return OKResponse{OK: true}, nil
}
