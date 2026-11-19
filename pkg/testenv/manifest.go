package testenv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ManifestFormat identifies the replay manifest layout.
const ManifestFormat = "iota.testenv.manifest/v1"

// MaxManifestBytes bounds a manifest in every form: export refuses a manifest
// whose indented JSON exceeds it, and the HTTP and CLI replay entry points
// accept up to it, so every exported manifest can be replayed.
const MaxManifestBytes = 4 << 20

// BuildInfo identifies the application build and database baseline a scope
// was prepared on. Replay refuses a manifest from another build or baseline.
type BuildInfo struct {
	BuildRevision       string `json:"buildRevision,omitempty"`
	SchemaFingerprint   string `json:"schemaFingerprint,omitempty"`
	BaselineFingerprint string `json:"baselineFingerprint,omitempty"`
}

// ManifestControls records environment controls needed to reproduce a scope.
type ManifestControls struct {
	// Clock is the controlled environment time when the manifest was taken.
	Clock string `json:"clock,omitempty"`
	// Resources holds snapshots of attached SnapshotResources by name.
	Resources map[string]json.RawMessage `json:"resources,omitempty"`
}

// Manifest reproduces a scope without the original conversation: logical seed,
// scenario/version/params/time, fingerprints and the actual identifiers that
// were produced. It never contains credentials: test identities and control
// tokens are issued separately to the isolated client, and scenario params
// must not carry secrets.
type Manifest struct {
	Format      string           `json:"format"`
	Environment BuildInfo        `json:"environment"`
	Scenario    Input            `json:"scenario"`
	Result      Result           `json:"result"`
	Controls    ManifestControls `json:"controls"`
}

// SetBuildInfo records the build and baseline fingerprints of this environment.
func (r *Registry) SetBuildInfo(info BuildInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.build = info
}

// Manifest captures a ready scope.
func (r *Registry) Manifest(ctx context.Context, scopeID string) (Manifest, error) {
	r.mu.Lock()
	sc := r.scopes[scopeID]
	build, clock := r.build, r.clock
	r.mu.Unlock()
	if sc == nil {
		return Manifest{}, failure("scope_conflict", "scope is not owned by this environment")
	}
	if err := sc.lock(ctx); err != nil {
		return Manifest{}, failure("timeout", err.Error())
	}
	defer sc.unlock()
	if sc.disposed.Load() {
		return Manifest{}, failure("scope_conflict", "scope disposed")
	}
	if sc.result == nil {
		return Manifest{}, failure("scope_conflict", "scope is not ready")
	}
	var input Input
	if err := json.Unmarshal([]byte(sc.request), &input); err != nil {
		return Manifest{}, failure("execution_failed", err.Error())
	}
	manifest := Manifest{Format: ManifestFormat, Environment: build, Scenario: input, Result: cloneResult(*sc.result)}
	if clock != nil && sc.scenario.definition.Isolation == "dedicated" {
		manifest.Controls.Clock = clock.Now().Format(time.RFC3339Nano)
	}
	for _, resource := range r.attached() {
		snapshotter, ok := resource.resource.(SnapshotResource)
		if !ok {
			continue
		}
		snapshot, err := snapshotter.SnapshotScope(scopeID)
		if err != nil {
			var typed *Error
			if errors.As(err, &typed) {
				return Manifest{}, typed
			}
			return Manifest{}, failure("execution_failed", resource.name+": "+err.Error())
		}
		if len(snapshot) == 0 {
			continue
		}
		if manifest.Controls.Resources == nil {
			manifest.Controls.Resources = map[string]json.RawMessage{}
		}
		manifest.Controls.Resources[resource.name] = snapshot
	}
	if indented, err := json.MarshalIndent(manifest, "", "  "); err != nil {
		return Manifest{}, failure("execution_failed", err.Error())
	} else if len(indented)+1 > MaxManifestBytes {
		return Manifest{}, failure("invalid_input", fmt.Sprintf("manifest is %d bytes; the replay limit is %d", len(indented)+1, MaxManifestBytes))
	}
	return manifest, nil
}

func manifestDigest(m Manifest) (string, error) {
	m.Scenario.ScopeID = ""
	encoded, err := json.Marshal(struct {
		Environment BuildInfo        `json:"environment"`
		Scenario    Input            `json:"scenario"`
		Controls    ManifestControls `json:"controls"`
	}{m.Environment, m.Scenario, m.Controls})
	if err != nil {
		return "", failure("invalid_input", err.Error())
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// Replay prepares the manifest's scenario in a new physical scope with the
// same logical seed, params and time, restoring controls inside that single
// preparation: scripts present before the original callback are restored
// before it, later scripts and the clock after it. The result is published
// only when everything succeeded; any failure compensates the scope, which then
// stays unready until disposed. Replaying the same manifest into the same scope
// again returns the same result; replay into a scope prepared otherwise
// conflicts.
func (r *Registry) Replay(ctx context.Context, scopeID string, m Manifest) (Result, error) {
	if m.Format != ManifestFormat {
		return Result{}, failure("invalid_input", "unsupported manifest format")
	}
	r.mu.Lock()
	build := r.build
	clock := r.clock
	r.mu.Unlock()
	if mismatch(m.Environment.BuildRevision, build.BuildRevision) || mismatch(m.Environment.SchemaFingerprint, build.SchemaFingerprint) || mismatch(m.Environment.BaselineFingerprint, build.BaselineFingerprint) {
		return Result{}, failure("version_mismatch", "manifest and environment must both declare the same build revision, schema and baseline fingerprints")
	}
	var at time.Time
	if m.Controls.Clock != "" {
		parsed, err := time.Parse(time.RFC3339Nano, m.Controls.Clock)
		if err != nil {
			return Result{}, failure("invalid_input", "manifest clock must be RFC3339")
		}
		if clock == nil || !r.dedicated {
			return Result{}, failure("missing_capability", "manifest controls the clock; a dedicated environment with a manual clock is required")
		}
		at = parsed
	}
	resources := map[string]SnapshotResource{}
	for _, resource := range r.attached() {
		if snapshotter, ok := resource.resource.(SnapshotResource); ok {
			resources[resource.name] = snapshotter
		}
	}
	names := make([]string, 0, len(m.Controls.Resources))
	for name := range m.Controls.Resources {
		if resources[name] == nil {
			return Result{}, failure("missing_capability", "manifest requires control "+name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	digest, err := manifestDigest(m)
	if err != nil {
		return Result{}, err
	}
	restore := func(phase RestorePhase) func(context.Context) error {
		return func(ctx context.Context) error {
			for _, name := range names {
				if err := resources[name].RestoreScope(ctx, scopeID, m.Controls.Resources[name], phase); err != nil {
					return fmt.Errorf("restore %s: %w", name, err)
				}
			}
			if phase == RestoreAfterPrepare && !at.IsZero() {
				if _, err := r.advanceTo(scopeID, at); err != nil {
					return fmt.Errorf("restore clock: %w", err)
				}
			}
			return nil
		}
	}
	input := m.Scenario
	input.ScopeID = scopeID
	return r.prepare(ctx, input, &replayPlan{digest: digest, before: restore(RestoreBeforePrepare), after: restore(RestoreAfterPrepare)})
}

// advanceTo sets the clock to an absolute time inside one Registry.mu
// critical section: no concurrent advance can interleave, and no duration
// conversion can saturate.
func (r *Registry) advanceTo(scopeID string, at time.Time) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ownedLocked(scopeID); err != nil {
		return time.Time{}, err
	}
	if r.clock == nil || !r.dedicated {
		return time.Time{}, failure("missing_capability", "clock control requires a dedicated environment with a manual clock")
	}
	if r.exclusiveScope != scopeID {
		return time.Time{}, failure("scope_conflict", "clock control requires the dedicated scenario lease")
	}
	if at.Before(r.clock.Now()) {
		return time.Time{}, failure("invalid_input", "manifest clock precedes scenario time")
	}
	r.clock.Set(at)
	return at, nil
}

// mismatch requires both sides to declare the fingerprint: an empty value is
// never a wildcard, so neither side can claim compatibility it cannot prove.
func mismatch(recorded, current string) bool {
	return recorded == "" || recorded != current
}
