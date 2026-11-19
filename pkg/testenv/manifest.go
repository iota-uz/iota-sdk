package testenv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// ManifestFormat identifies the replay manifest layout.
const ManifestFormat = "iota.testenv.manifest/v1"

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
func (r *Registry) Manifest(scopeID string) (Manifest, error) {
	r.mu.Lock()
	sc := r.scopes[scopeID]
	build, clock := r.build, r.clock
	r.mu.Unlock()
	if sc == nil {
		return Manifest{}, failure("scope_conflict", "scope is not owned by this environment")
	}
	if err := sc.lock(context.Background()); err != nil {
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
	return manifest, nil
}

func manifestDigest(m Manifest) string {
	m.Scenario.ScopeID = ""
	encoded, _ := json.Marshal(struct {
		Environment BuildInfo
		Scenario    Input
		Controls    ManifestControls
	}{m.Environment, m.Scenario, m.Controls})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// Replay prepares the manifest's scenario in a new physical scope with the
// same logical seed, params and time, then restores clock and controls. A
// repeated identical replay returns the same result without re-applying
// controls. Restoration failure disposes the scope's state and reports it.
func (r *Registry) Replay(ctx context.Context, scopeID string, m Manifest) (Result, error) {
	if m.Format != ManifestFormat {
		return Result{}, failure("invalid_input", "unsupported manifest format")
	}
	r.mu.Lock()
	build := r.build
	clock := r.clock
	r.mu.Unlock()
	if mismatch(m.Environment.BuildRevision, build.BuildRevision) || mismatch(m.Environment.SchemaFingerprint, build.SchemaFingerprint) || mismatch(m.Environment.BaselineFingerprint, build.BaselineFingerprint) {
		return Result{}, failure("version_mismatch", "manifest was recorded on another build or baseline")
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
	for name := range m.Controls.Resources {
		if resources[name] == nil {
			return Result{}, failure("missing_capability", "manifest requires control "+name)
		}
	}
	input := m.Scenario
	input.ScopeID = scopeID
	result, err := r.Prepare(ctx, input)
	if err != nil {
		return Result{}, err
	}
	digest := manifestDigest(m)
	r.mu.Lock()
	sc := r.scopes[scopeID]
	r.mu.Unlock()
	if err := sc.lock(ctx); err != nil {
		return Result{}, failure("timeout", err.Error())
	}
	defer sc.unlock()
	if sc.disposed.Load() {
		return Result{}, failure("scope_conflict", "scope disposed")
	}
	if sc.replayed == digest {
		return result, nil
	}
	if sc.replayed != "" {
		return Result{}, failure("scope_conflict", "scope replayed another manifest")
	}
	var restoreErr error
	for name, snapshot := range m.Controls.Resources {
		if err := resources[name].RestoreScope(ctx, scopeID, snapshot); err != nil {
			restoreErr = errors.Join(restoreErr, err)
		}
	}
	if restoreErr == nil && !at.IsZero() {
		_, restoreErr = r.advanceTo(scopeID, at)
	}
	if restoreErr != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		// The scope keeps its prepared records until disposed, but its
		// controls are not ready: drop them so nothing half-restored runs.
		cleanupErr := r.forgetResources(cleanupCtx, scopeID)
		sc.replayed = "failed"
		return Result{}, lifecycleError(restoreErr, "execution_failed", "replay", Descriptor{EnvironmentID: r.environmentID}, cleanupErr)
	}
	sc.replayed = digest
	return result, nil
}

func (r *Registry) advanceTo(scopeID string, at time.Time) (time.Time, error) {
	now, ok := r.ClockNow()
	if !ok {
		return time.Time{}, failure("missing_capability", "no manual clock")
	}
	if at.Before(now) {
		return time.Time{}, failure("invalid_input", "manifest clock precedes scenario time")
	}
	return r.AdvanceClock(scopeID, at.Sub(now))
}

// mismatch treats an unknown current fingerprint as drift: an environment that
// cannot prove its build/baseline cannot claim to reproduce the manifest.
func mismatch(recorded, current string) bool {
	return recorded != "" && recorded != current
}
