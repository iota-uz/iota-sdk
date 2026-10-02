package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

type environment struct {
	mu         sync.Mutex
	spec       string
	descriptor Descriptor
	started    bool
	attempted  bool
	stopped    bool
}
type Coordinator struct {
	mu           sync.Mutex
	adapter      Adapter
	environments map[string]*environment
	ids          map[string]*environment
}

func NewCoordinator(adapter Adapter) *Coordinator {
	return &Coordinator{adapter: adapter, environments: map[string]*environment{}, ids: map[string]*environment{}}
}
func (c *Coordinator) Start(ctx context.Context, spec Spec) (Descriptor, error) {
	if c.adapter == nil || spec.RunID == "" || spec.Slot == "" || spec.SchemaFingerprint == "" || spec.BaselineFingerprint == "" || !slices.Contains([]string{"shared", "worker", "attempt"}, spec.Isolation) {
		return Descriptor{}, failure("invalid_spec", "invalid environment specification")
	}
	spec.RequiredCapabilities = slices.Clone(spec.RequiredCapabilities)
	slices.Sort(spec.RequiredCapabilities)
	spec.RequiredCapabilities = slices.Compact(spec.RequiredCapabilities)
	encoded, _ := json.Marshal(spec)
	keyBytes, _ := json.Marshal([]string{spec.RunID, spec.Slot})
	key := string(keyBytes)
	c.mu.Lock()
	env := c.environments[key]
	if env == nil {
		env = &environment{spec: string(encoded), descriptor: Descriptor{EnvironmentID: uuid.NewString()}}
		c.environments[key] = env
		c.ids[env.descriptor.EnvironmentID] = env
	}
	c.mu.Unlock()
	env.mu.Lock()
	defer env.mu.Unlock()
	if env.spec != string(encoded) || env.stopped {
		return Descriptor{}, failure("resource_conflict", "slot has a different specification or has been stopped")
	}
	if env.started {
		return copyDescriptor(env.descriptor), nil
	}
	if env.attempted {
		return Descriptor{}, failure("resource_conflict", "startup failed; stop the environment and use a new slot")
	}
	if err := ctx.Err(); err != nil {
		return Descriptor{}, failure("timeout", err.Error())
	}
	id := env.descriptor.EnvironmentID
	env.attempted = true
	d, err := c.adapter.Start(ctx, spec, id)
	d.EnvironmentID = id
	env.descriptor = d
	if err == nil {
		err = c.adapter.Ready(ctx, d)
	}
	if err == nil {
		u, parseErr := url.Parse(d.BaseURL)
		if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || d.BuildRevision == "" || d.ArtifactDirectory == "" {
			err = failure("startup_failed", "invalid ready descriptor")
		}
	}
	if err == nil && (d.SchemaFingerprint != spec.SchemaFingerprint || d.BaselineFingerprint != spec.BaselineFingerprint) {
		err = failure("baseline_mismatch", "ready fingerprints differ from requested fingerprints")
	}
	if err == nil {
		for _, capability := range spec.RequiredCapabilities {
			if !slices.Contains(d.Capabilities, capability) {
				err = failure("missing_capability", capability)
				break
			}
		}
	}
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if cleanupErr := c.adapter.Stop(cleanupCtx, d); cleanupErr != nil {
			return Descriptor{}, failure("cleanup_failed", cleanupErr.Error())
		}
		env.stopped = true
		if ctx.Err() != nil {
			return Descriptor{}, failure("timeout", ctx.Err().Error())
		}
		if _, ok := err.(*Error); ok {
			return Descriptor{}, err
		}
		return Descriptor{}, failure("startup_failed", err.Error())
	}
	env.started = true
	return copyDescriptor(d), nil
}

func (c *Coordinator) StopAll(ctx context.Context) error {
	c.mu.Lock()
	ids := make([]string, 0, len(c.ids))
	for id := range c.ids {
		ids = append(ids, id)
	}
	c.mu.Unlock()
	var failures []error
	for _, id := range ids {
		if err := c.Stop(ctx, id); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
func copyDescriptor(d Descriptor) Descriptor { d.Capabilities = slices.Clone(d.Capabilities); return d }
func (c *Coordinator) Stop(ctx context.Context, id string) error {
	c.mu.Lock()
	env := c.ids[id]
	c.mu.Unlock()
	if env == nil {
		return failure("resource_conflict", "environment is not owned by this coordinator")
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if env.stopped {
		return nil
	}
	if err := c.adapter.Stop(ctx, env.descriptor); err != nil {
		return failure("cleanup_failed", err.Error())
	}
	env.stopped = true
	return nil
}
