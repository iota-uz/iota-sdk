package manage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ListVersions lists the versions of a package, newest first.
func (s *Service) ListVersions(ctx context.Context, actor spi.Subject, ns string) ([]Version, error) {
	const op = serrors.Op("extruntime.manage.ListVersions")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	if _, err := s.packageRow(ctx, s.db, ns, false); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT `+versionColumns+versionFrom+` WHERE v.namespace = $1 ORDER BY v.created_at DESC, v.version DESC`, ns)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, v.Version)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// GetVersion returns one version with its manifest.
func (s *Service) GetVersion(ctx context.Context, actor spi.Subject, ns, version string) (VersionDetail, error) {
	if err := require(actor, PermRead); err != nil {
		return VersionDetail{}, err
	}
	return s.versionRow(ctx, s.db, ns, version)
}

// ListActivations returns the activation history of a package, newest first.
func (s *Service) ListActivations(ctx context.Context, actor spi.Subject, ns string) ([]Activation, error) {
	const op = serrors.Op("extruntime.manage.ListActivations")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT a.id, a.namespace, v.version, a.state, a.activated_by, a.activated_at, a.ended_by, a.ended_at, a.schema_steps
		FROM extruntime.activations a JOIN extruntime.versions v ON v.id = a.version_id
		WHERE a.namespace = $1 ORDER BY a.activated_at DESC, a.id`, ns)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []Activation
	for rows.Next() {
		var a Activation
		var state string
		var steps []byte
		if err := rows.Scan(&a.ID, &a.Namespace, &a.Version, &state, &a.ActivatedBy, &a.ActivatedAt, &a.EndedBy, &a.EndedAt, &steps); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		a.State = ActivationState(state)
		if err := json.Unmarshal(steps, &a.SchemaSteps); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		out = append(out, a)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// ListBindings lists open bindings, or the full history when history is true.
func (s *Service) ListBindings(ctx context.Context, actor spi.Subject, history bool) ([]Binding, error) {
	const op = serrors.Op("extruntime.manage.ListBindings")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	sql := `SELECT id, slot_kind, target_id, package, version, export, activated_by, active_from, active_to FROM extruntime.bindings`
	if !history {
		sql += ` WHERE active_to IS NULL`
	}
	rows, err := s.db.Query(ctx, sql+` ORDER BY slot_kind, target_id, active_from`)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []Binding
	for rows.Next() {
		b, err := scanBinding(rows)
		if err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, b)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// AuditFilter narrows ListAudit.
type AuditFilter struct {
	Namespace string
	Action    string
	// BeforeID pages: only rows with a smaller id.
	BeforeID int64
	Limit    int
}

// ListAudit returns the audit log, newest first.
func (s *Service) ListAudit(ctx context.Context, actor spi.Subject, f AuditFilter) ([]AuditEntry, error) {
	const op = serrors.Op("extruntime.manage.ListAudit")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := s.db.Query(ctx, `SELECT id, at, actor_user_id, credential, action, namespace, version, target, source_hash, artifact_hash, outcome, detail
		FROM extruntime.audit_log
		WHERE ($1 = '' OR namespace = $1) AND ($2 = '' OR action = $2) AND ($3 = 0 OR id < $3)
		ORDER BY id DESC LIMIT $4`, f.Namespace, f.Action, f.BeforeID, f.Limit)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var a AuditEntry
		var detail []byte
		if err := rows.Scan(&a.ID, &a.At, &a.ActorUserID, &a.Credential, &a.Action, &a.Namespace, &a.Version, &a.Target, &a.SourceHash, &a.ArtifactHash, &a.Outcome, &detail); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		if err := json.Unmarshal(detail, &a.Detail); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		out = append(out, a)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// meta loads the immutable facts of a published version (cached).
func (s *Service) meta(ctx context.Context, pkg, version string) (versionMeta, error) {
	key := pkg + "@" + version
	s.verMu.RLock()
	m, ok := s.verCache[key]
	s.verMu.RUnlock()
	if ok {
		return m, nil
	}
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT artifact_hash, manifest FROM extruntime.versions WHERE namespace = $1 AND version = $2`, pkg, version).
		Scan(&m.artifactHash, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, notFound("version " + key)
	}
	if err != nil {
		return m, serrors.FromDB("extruntime.manage.meta", err)
	}
	if err := json.Unmarshal(raw, &m.manifest); err != nil {
		return m, serrors.Wrap("extruntime.manage.meta", err)
	}
	m.secrets = append([]string(nil), m.manifest.Capabilities.Secrets...)
	s.verMu.Lock()
	s.verCache[key] = m
	s.verMu.Unlock()
	return m, nil
}

// ManifestOf returns the manifest of a published version. Versions are
// immutable, so workflow instances and notification snapshots may keep
// resolving a version that is no longer active.
func (s *Service) ManifestOf(ctx context.Context, pkg, version string) (manifest.Manifest, error) {
	m, err := s.meta(ctx, pkg, version)
	return m.manifest, err
}

// ResolveArtifact maps a package version to its materialized immutable
// artifact directory and declared secrets. It implements host.ArtifactResolver
// and is the single door through which package code is handed to the pool: a
// version runs only while its newest review is an approval bound to the
// version's exact source and artifact hashes (the rule Activate enforces).
// Unreviewed and rejected versions fail with FailedPrecondition.
func (s *Service) ResolveArtifact(ctx context.Context, pkg, version string) (pool.Artifact, []string, error) {
	art, secrets, approved, err := s.resolve(ctx, pkg, version)
	if err != nil {
		return pool.Artifact{}, nil, err
	}
	if !approved {
		return pool.Artifact{}, nil, notApproved(pkg, version)
	}
	return art, secrets, nil
}

func notApproved(pkg, version string) error {
	return serrors.New(serrors.FailedPrecondition, "version "+pkg+"@"+version+" is not approved")
}

// resolve is ResolveArtifact with the approval verdict returned instead of
// raised, for callers that skip a revoked version rather than fail. The
// artifact is materialized only for an approved version.
func (s *Service) resolve(ctx context.Context, pkg, version string) (pool.Artifact, []string, bool, error) {
	m, err := s.meta(ctx, pkg, version)
	if err != nil {
		return pool.Artifact{}, nil, false, err
	}
	approved, err := s.approved(ctx, pkg, version)
	if err != nil || !approved {
		return pool.Artifact{}, nil, false, err
	}
	dir, err := s.arts.Dir(ctx, m.artifactHash)
	if err != nil {
		return pool.Artifact{}, nil, false, err
	}
	return pool.Artifact{Hash: m.artifactHash, Dir: dir}, m.secrets, true, nil
}

// approved reports whether the newest review of a version approves its exact
// hashes. Reviews are appended over time, so this is read on every call
// rather than cached with the immutable facts of the version.
func (s *Service) approved(ctx context.Context, pkg, version string) (bool, error) {
	var ok *bool
	err := s.db.QueryRow(ctx, `SELECT r.approve AND r.source_hash = v.source_hash AND r.artifact_hash = v.artifact_hash
		FROM extruntime.versions v
		LEFT JOIN LATERAL (SELECT * FROM extruntime.version_reviews WHERE version_id = v.id ORDER BY id DESC LIMIT 1) r ON true
		WHERE v.namespace = $1 AND v.version = $2`, pkg, version).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, notFound("version " + pkg + "@" + version)
	}
	if err != nil {
		return false, serrors.FromDB("extruntime.manage.approved", err)
	}
	return ok != nil && *ok, nil
}

// ReviewInput is a review decision. ExpectSourceHash and ExpectArtifactHash,
// when set, must equal the version's hashes: the reviewer approves exactly
// what they looked at.
type ReviewInput struct {
	Approve            bool
	Comment            string
	ExpectSourceHash   string
	ExpectArtifactHash string
}

// Review records an approval or rejection of one version, bound to its exact
// source and artifact hashes. An author may review their own version when
// they hold the review permission.
func (s *Service) Review(ctx context.Context, actor spi.Subject, ns, version string, approve bool, comment string) (Review, error) {
	return s.ReviewExact(ctx, actor, ns, version, ReviewInput{Approve: approve, Comment: comment})
}

// ReviewExact is Review with optional hash expectations.
func (s *Service) ReviewExact(ctx context.Context, actor spi.Subject, ns, version string, in ReviewInput) (Review, error) {
	const op = serrors.Op("extruntime.manage.Review")
	e := &auditEntry{Action: ActionReview, Namespace: ns, Version: version, Detail: map[string]any{"approve": in.Approve, "comment": in.Comment}}
	var out Review
	err := s.mutate(ctx, actor, PermReview, e, func(tx pgx.Tx) error {
		if !in.Approve && in.Comment == "" {
			return serrors.New(serrors.Invalid, "a rejection needs a comment")
		}
		v, err := s.versionRow(ctx, tx, ns, version)
		if err != nil {
			return err
		}
		e.SourceHash, e.ArtifactHash = v.SourceHash, v.ArtifactHash
		if in.ExpectSourceHash != "" && in.ExpectSourceHash != v.SourceHash ||
			in.ExpectArtifactHash != "" && in.ExpectArtifactHash != v.ArtifactHash {
			return serrors.New(serrors.Conflict, "the reviewed hashes do not match the version")
		}
		out = Review{Approve: in.Approve, ReviewerID: actor.UserID, Comment: in.Comment, SourceHash: v.SourceHash, ArtifactHash: v.ArtifactHash}
		if err := tx.QueryRow(ctx, `INSERT INTO extruntime.version_reviews (version_id, source_hash, artifact_hash, reviewer_id, approve, comment)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
			v.ID, v.SourceHash, v.ArtifactHash, actor.UserID, in.Approve, in.Comment).Scan(&out.ID, &out.At); err != nil {
			return serrors.FromDB(op, err)
		}
		return nil
	})
	return out, err
}

// ExportDraft packs the draft tree as a tar or zip archive. Paths and bytes
// are preserved, so importing the archive again yields the same tree hash.
func (s *Service) ExportDraft(ctx context.Context, actor spi.Subject, ns string, format ExportFormat) (Archive, error) {
	e := &auditEntry{Action: ActionExport, Namespace: ns, Target: "draft", Detail: map[string]any{"format": string(format)}}
	var out Archive
	err := s.observe(ctx, actor, PermRead, e, func() error {
		if _, err := s.packageRow(ctx, s.db, ns, false); err != nil {
			return err
		}
		tree, err := s.draftTree(ctx, s.db, ns)
		if err != nil {
			return err
		}
		out, err = packArchive(ns+"-draft", format, tree)
		e.SourceHash = out.TreeHash
		return err
	})
	return out, err
}

// ExportVersion packs the exact source tree of a published version.
func (s *Service) ExportVersion(ctx context.Context, actor spi.Subject, ns, version string, format ExportFormat) (Archive, error) {
	e := &auditEntry{Action: ActionExport, Namespace: ns, Version: version, Target: "version", Detail: map[string]any{"format": string(format)}}
	var out Archive
	err := s.observe(ctx, actor, PermRead, e, func() error {
		var archive string
		err := s.db.QueryRow(ctx, `SELECT source_archive FROM extruntime.versions WHERE namespace = $1 AND version = $2`, ns, version).Scan(&archive)
		if errors.Is(err, pgx.ErrNoRows) {
			return notFound("version " + ns + "@" + version)
		}
		if err != nil {
			return serrors.FromDB("extruntime.manage.ExportVersion", err)
		}
		tree, err := s.arts.Tree(ctx, archive)
		if err != nil {
			return err
		}
		out, err = packArchive(ns+"-"+version, format, tree)
		e.SourceHash = out.TreeHash
		return err
	})
	return out, err
}

func packArchive(name string, format ExportFormat, tree build.Tree) (Archive, error) {
	var data []byte
	var err error
	out := Archive{TreeHash: artifact.TreeHash(tree)}
	switch format {
	case FormatTar, "":
		data, err = artifact.Pack(tree)
		out.Filename, out.ContentType = name+".tar", "application/x-tar"
	case FormatZip:
		data, err = artifact.PackZip(tree)
		out.Filename, out.ContentType = name+".zip", "application/zip"
	default:
		return Archive{}, serrors.New(serrors.Invalid, "unknown export format "+string(format))
	}
	out.Data = data
	return out, err
}

// UnpackArchive reads an uploaded tar or zip tree (format by content).
func UnpackArchive(data []byte) (build.Tree, error) {
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' {
		return artifact.UnpackZip(data)
	}
	return artifact.Unpack(data)
}
