package testenv

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ScopeResource owns state that a scope may create outside the scenario's
// committed records: jobs, provider scripts, files, queue namespaces. ForgetScope
// must be idempotent and remove only state owned by scopeID.
type ScopeResource interface {
	ForgetScope(ctx context.Context, scopeID string) error
}

// ScopeStopper owns running work (jobs, workers) of a scope. StopScope cancels
// it and returns only after it has exited; until then no destructive cleanup
// runs, so work cannot write after its records are removed.
type ScopeStopper interface {
	StopScope(ctx context.Context, scopeID string) error
}

// RestorePhase says when replay restores a snapshot part: before the scenario
// callback (state the callback consumes) or after it (state for later work).
type RestorePhase int

const (
	RestoreBeforePrepare RestorePhase = iota
	RestoreAfterPrepare
)

// SnapshotResource can also be captured in a replay manifest and restored in a
// fresh environment. Snapshots must not contain credentials: refuse export of
// state that does.
type SnapshotResource interface {
	ScopeResource
	SnapshotScope(scopeID string) (json.RawMessage, error)
	RestoreScope(ctx context.Context, scopeID string, snapshot json.RawMessage, phase RestorePhase) error
}

// prepareObserver is told right before a scenario callback runs, so it can
// record which of its state existed at the start of preparation.
type prepareObserver interface {
	beforePrepare(scopeID string)
}

type namedResource struct {
	name     string
	resource ScopeResource
}

type cleanupResource CleanupFunc

func (f cleanupResource) ForgetScope(ctx context.Context, scopeID string) error {
	return f(ctx, scopeID)
}

// Attach registers a resource disposed together with every scope of r.
// Resources are forgotten in reverse attach order before scenario cleanup.
// Attach before allowing any scope: a resource attached later could miss a
// scope's disposal.
func (r *Registry) Attach(name string, resource ScopeResource) error {
	if name == "" || resource == nil {
		return failure("invalid_input", "resource name and implementation required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.scopes) > 0 {
		return failure("scope_conflict", "attach resources before allowing scopes")
	}
	for _, existing := range r.resources {
		if existing.name == name {
			return failure("scope_conflict", "resource already attached: "+name)
		}
	}
	r.resources = append(r.resources, namedResource{name: name, resource: resource})
	return nil
}

// OnDispose registers scope cleanup for consumer-owned state such as a queue
// namespace (see QueueNamespace) or an external fake.
func (r *Registry) OnDispose(name string, cleanup CleanupFunc) error {
	if cleanup == nil {
		return failure("invalid_input", "cleanup required")
	}
	return r.Attach(name, cleanupResource(cleanup))
}

func (r *Registry) attached() []namedResource {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]namedResource(nil), r.resources...)
}

func (r *Registry) stopResources(ctx context.Context, scopeID string) error {
	var errs []error
	for _, resource := range r.attached() {
		if stopper, ok := resource.resource.(ScopeStopper); ok {
			if err := stopper.StopScope(ctx, scopeID); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", resource.name, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (r *Registry) markPrepared(scopeID string) {
	for _, resource := range r.attached() {
		if observer, ok := resource.resource.(prepareObserver); ok {
			observer.beforePrepare(scopeID)
		}
	}
}

func (r *Registry) forgetResources(ctx context.Context, scopeID string) error {
	resources := r.attached()
	var errs []error
	for i := len(resources) - 1; i >= 0; i-- {
		if err := resources[i].resource.ForgetScope(ctx, scopeID); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", resources[i].name, err))
		}
	}
	return errors.Join(errs...)
}

// ScopedFiles gives each scope its own directory below root and removes it on
// dispose. Point the application's scoped upload/export path at Dir(scopeID).
type ScopedFiles struct {
	registry *Registry
	root     string
}

// NewScopedFiles attaches a file owner to r. root must be an owned directory.
func NewScopedFiles(r *Registry, root string) (*ScopedFiles, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, failure("invalid_input", "absolute file root required")
	}
	files := &ScopedFiles{registry: r, root: filepath.Clean(root)}
	if err := r.Attach("files", files); err != nil {
		return nil, err
	}
	return files, nil
}

// path names the directory by the hex encoding of the scope ID: lossless,
// separator-free and distinct on case-insensitive volumes.
func (f *ScopedFiles) path(scopeID string) (string, error) {
	if scopeID == "" {
		return "", failure("invalid_input", "empty scope")
	}
	return filepath.Join(f.root, "scope-"+hex.EncodeToString([]byte(scopeID))), nil
}

// Dir creates and returns the directory of a live scope.
func (f *ScopedFiles) Dir(scopeID string) (string, error) {
	release, err := f.registry.admit(scopeID)
	if err != nil {
		return "", err
	}
	defer release()
	dir, err := f.path(scopeID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ForgetScope removes the scope directory. IDs that cannot name a directory
// never had one, so there is nothing to remove.
func (f *ScopedFiles) ForgetScope(_ context.Context, scopeID string) error {
	if dir, err := f.path(scopeID); err == nil {
		return os.RemoveAll(dir)
	}
	return nil
}
