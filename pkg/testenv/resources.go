package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScopeResource owns state that a scope may create outside the scenario's
// committed records: jobs, provider scripts, files, queue namespaces. ForgetScope
// must be idempotent and remove only state owned by scopeID.
type ScopeResource interface {
	ForgetScope(ctx context.Context, scopeID string) error
}

// SnapshotResource can also be captured in a replay manifest and restored in a
// fresh environment. Snapshots must not contain credentials.
type SnapshotResource interface {
	ScopeResource
	SnapshotScope(scopeID string) (json.RawMessage, error)
	RestoreScope(ctx context.Context, scopeID string, snapshot json.RawMessage) error
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
func (r *Registry) Attach(name string, resource ScopeResource) error {
	if name == "" || resource == nil {
		return failure("invalid_input", "resource name and implementation required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (f *ScopedFiles) path(scopeID string) (string, error) {
	if scopeID == "" || strings.ContainsAny(scopeID, "/\\\x00") || scopeID == "." || scopeID == ".." {
		return "", failure("invalid_input", "scope cannot name a directory")
	}
	return filepath.Join(f.root, scopeID), nil
}

// Dir creates and returns the directory of a live scope.
func (f *ScopedFiles) Dir(scopeID string) (string, error) {
	if err := f.registry.owned(scopeID); err != nil {
		return "", err
	}
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
