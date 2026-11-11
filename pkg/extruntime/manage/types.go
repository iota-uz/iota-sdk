// Package manage is the single management service for extension packages:
// draft trees, Git import/export, check/build/test, immutable versions,
// reviews, activation, bindings, trigger owners, the developer kit and the
// audit log. UI, HTTP and MCP all call it, so rights and audit are the same.
package manage

import (
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Provenance says where a package tree came from.
type Provenance string

const (
	ProvenanceGit     Provenance = "git"
	ProvenanceRuntime Provenance = "runtime"
)

// Package is a package draft head. Packages are application-wide and keyed by
// namespace.
type Package struct {
	Namespace  string     `json:"namespace"`
	NPMName    string     `json:"npmName,omitempty"`
	Provenance Provenance `json:"provenance"`
	RepoURL    string     `json:"repoUrl,omitempty"`
	RepoCommit string     `json:"repoCommit,omitempty"`
	DraftRev   int64      `json:"draftRev"`
	CreatedBy  int64      `json:"createdBy"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedBy  int64      `json:"updatedBy"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	// ActiveVersion is the activated version label, empty when none.
	ActiveVersion string `json:"activeVersion,omitempty"`
}

// File describes one draft file.
type File struct {
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
	Size      int       `json:"size"`
	UpdatedBy int64     `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Review is one review decision bound to the exact hashes it covered.
type Review struct {
	ID           int64     `json:"id"`
	Approve      bool      `json:"approve"`
	ReviewerID   int64     `json:"reviewerId"`
	Comment      string    `json:"comment,omitempty"`
	SourceHash   string    `json:"sourceHash"`
	ArtifactHash string    `json:"artifactHash"`
	At           time.Time `json:"at"`
}

// Version is an immutable published build of a package.
type Version struct {
	ID           uuid.UUID          `json:"id"`
	Namespace    string             `json:"namespace"`
	Version      string             `json:"version"`
	SourceHash   string             `json:"sourceHash"`
	TreeHash     string             `json:"treeHash"`
	ArtifactHash string             `json:"artifactHash"`
	LockfileName string             `json:"lockfileName,omitempty"`
	LockfileHash string             `json:"lockfileHash,omitempty"`
	KitHash      string             `json:"kitHash"`
	Provenance   Provenance         `json:"provenance"`
	RepoURL      string             `json:"repoUrl,omitempty"`
	RepoCommit   string             `json:"repoCommit,omitempty"`
	Diagnostics  []build.Diagnostic `json:"diagnostics"`
	AuthorID     int64              `json:"authorId"`
	CreatedAt    time.Time          `json:"createdAt"`
	// LatestReview is the newest review, nil when none exists. Approved is
	// true when it approves this version.
	LatestReview *Review `json:"latestReview,omitempty"`
	Approved     bool    `json:"approved"`
	Active       bool    `json:"active"`
}

// VersionDetail adds the manifest.
type VersionDetail struct {
	Version
	Manifest manifest.Manifest `json:"manifest"`
}

// ActivationState is the lifecycle of an activation row.
type ActivationState string

const (
	ActivationActive   ActivationState = "active"
	ActivationDisabled ActivationState = "disabled"
	ActivationPurged   ActivationState = "purged"
)

// Activation is one entry of the activation history of a package.
type Activation struct {
	ID          uuid.UUID       `json:"id"`
	Namespace   string          `json:"namespace"`
	Version     string          `json:"version"`
	State       ActivationState `json:"state"`
	ActivatedBy int64           `json:"activatedBy"`
	ActivatedAt time.Time       `json:"activatedAt"`
	EndedBy     *int64          `json:"endedBy,omitempty"`
	EndedAt     *time.Time      `json:"endedAt,omitempty"`
	SchemaSteps []string        `json:"schemaSteps,omitempty"`
}

// Slot kinds of explicit bindings.
const (
	SlotCommand = "command"
	SlotScreen  = "screen"
	SlotShell   = "shell"
)

// Binding is a row of extruntime.bindings.
type Binding struct {
	ID          uuid.UUID  `json:"id"`
	SlotKind    string     `json:"slotKind"`
	TargetID    string     `json:"targetId"`
	Package     string     `json:"package"`
	Version     string     `json:"version"`
	Export      string     `json:"export"`
	ActivatedBy *int64     `json:"activatedBy,omitempty"`
	ActiveFrom  time.Time  `json:"activeFrom"`
	ActiveTo    *time.Time `json:"activeTo,omitempty"`
}

// ActivationResult reports what activating a version did.
type ActivationResult struct {
	Activation Activation `json:"activation"`
	// SchemaSteps are the DDL statements applied.
	SchemaSteps []string `json:"schemaSteps,omitempty"`
	// Bound lists the slots the package's own implementations now serve.
	Bound []Binding `json:"bound,omitempty"`
	// Rebound lists replacement bindings carried over to the new version.
	Rebound []Binding `json:"rebound,omitempty"`
	// Released lists replacement bindings the new version no longer
	// implements; the slot returned to its previous implementation.
	Released []Binding `json:"released,omitempty"`
}

// AuditEntry is one row of the management audit log.
type AuditEntry struct {
	ID           int64          `json:"id"`
	At           time.Time      `json:"at"`
	ActorUserID  int64          `json:"actorUserId"`
	Credential   string         `json:"credential,omitempty"`
	Action       string         `json:"action"`
	Namespace    string         `json:"namespace,omitempty"`
	Version      string         `json:"version,omitempty"`
	Target       string         `json:"target,omitempty"`
	SourceHash   string         `json:"sourceHash,omitempty"`
	ArtifactHash string         `json:"artifactHash,omitempty"`
	Outcome      string         `json:"outcome"`
	Detail       map[string]any `json:"detail,omitempty"`
}

// Audit actions.
const (
	ActionCreatePackage = "package.create"
	ActionPutFile       = "draft.put"
	ActionDeleteFile    = "draft.delete"
	ActionMoveFile      = "draft.move"
	ActionImportTree    = "import.tree"
	ActionImportGit     = "import.git"
	ActionExport        = "export"
	ActionCheck         = "check"
	ActionBuild         = "build"
	ActionTest          = "test"
	ActionPublish       = "publish"
	ActionReview        = "review"
	ActionActivate      = "activate"
	ActionDeactivate    = "deactivate"
	ActionPurge         = "purge"
	ActionBind          = "bind"
	ActionUnbind        = "unbind"
	ActionAssignOwner   = "owner.assign"
	ActionRemoveOwner   = "owner.remove"
	ActionKit           = "kit.download"
)

// Outcomes of an audited action.
const (
	OutcomeOK     = "ok"
	OutcomeFailed = "failed"
	OutcomeDenied = "denied"
)

// BuildReport is the result of Build.
type BuildReport struct {
	Diagnostics  []build.Diagnostic `json:"diagnostics"`
	HasErrors    bool               `json:"hasErrors"`
	SourceHash   string             `json:"sourceHash,omitempty"`
	ArtifactHash string             `json:"artifactHash,omitempty"`
	Manifest     *manifest.Manifest `json:"manifest,omitempty"`
	KitHash      string             `json:"kitHash"`
}

// CheckReport is the result of Check.
type CheckReport struct {
	Diagnostics []build.Diagnostic `json:"diagnostics"`
	HasErrors   bool               `json:"hasErrors"`
	KitHash     string             `json:"kitHash"`
}

// ExportFormat selects the archive type of an export.
type ExportFormat string

const (
	FormatTar ExportFormat = "tar"
	FormatZip ExportFormat = "zip"
)

// Archive is a downloadable export.
type Archive struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Data        []byte `json:"-"`
	TreeHash    string `json:"treeHash"`
}
