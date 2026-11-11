package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DiagnosticsError is carried (through errors.As) by a refused publish.
type DiagnosticsError struct {
	Diagnostics []build.Diagnostic
}

func (e *DiagnosticsError) Error() string {
	for _, d := range e.Diagnostics {
		if d.Severity == build.SeverityError {
			return fmt.Sprintf("%s: %s", d.File, d.Message)
		}
	}
	return "build diagnostics"
}

func hasErrors(ds []build.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == build.SeverityError {
			return true
		}
	}
	return false
}

func (s *Service) loadForBuild(ctx context.Context, ns string) (Package, build.Tree, error) {
	pkg, err := s.packageRow(ctx, s.db, ns, false)
	if err != nil {
		return Package{}, nil, err
	}
	tree, err := s.draftTree(ctx, s.db, ns)
	if err != nil {
		return Package{}, nil, err
	}
	return pkg, tree, nil
}

// Check typechecks the draft and validates its graphs against the current
// kit snapshot without producing artifacts.
func (s *Service) Check(ctx context.Context, actor spi.Subject, ns string) (CheckReport, error) {
	e := &auditEntry{Action: ActionCheck, Namespace: ns, Detail: map[string]any{}}
	var out CheckReport
	err := s.observe(ctx, actor, PermAuthor, e, func() error {
		_, tree, err := s.loadForBuild(ctx, ns)
		if err != nil {
			return err
		}
		e.SourceHash = artifact.TreeHash(tree)
		kit, err := s.Kit(ctx)
		if err != nil {
			return err
		}
		ds, err := s.builder.Check(ctx, tree, kit)
		if err != nil {
			return serrors.Wrap("extruntime.manage.Check", err)
		}
		out = CheckReport{Diagnostics: nonNil(ds), HasErrors: hasErrors(ds), KitHash: kit.Hash}
		e.Detail["hasErrors"], e.Detail["diagnostics"], e.Detail["kitHash"] = out.HasErrors, len(ds), kit.Hash
		return nil
	})
	return out, err
}

func nonNil(ds []build.Diagnostic) []build.Diagnostic {
	if ds == nil {
		return []build.Diagnostic{}
	}
	return ds
}

// buildDraft runs the builder; diagnostics with errors come back as a report,
// not as an error.
func (s *Service) buildDraft(ctx context.Context, tree build.Tree) (build.Result, build.Kit, error) {
	res, kit, _, err := s.buildDraftContracts(ctx, tree)
	return res, kit, err
}

// buildDraftContracts also returns the contract hashes of the kit used.
func (s *Service) buildDraftContracts(ctx context.Context, tree build.Tree) (build.Result, build.Kit, map[string]string, error) {
	kit, contracts, err := s.kitWithContracts(ctx)
	if err != nil {
		return build.Result{}, kit, nil, err
	}
	res, err := s.builder.Build(ctx, tree, kit)
	var failed *build.Failed
	if errors.As(err, &failed) {
		res.Diagnostics = failed.Diagnostics
		return res, kit, contracts, nil
	}
	if err != nil {
		return res, kit, contracts, serrors.Wrap("extruntime.manage.Build", err)
	}
	return res, kit, contracts, nil
}

// Build builds the draft against the current kit snapshot. Diagnostics with
// errors are reported in the result, not as an error.
func (s *Service) Build(ctx context.Context, actor spi.Subject, ns string) (BuildReport, error) {
	e := &auditEntry{Action: ActionBuild, Namespace: ns, Detail: map[string]any{}}
	var out BuildReport
	err := s.observe(ctx, actor, PermAuthor, e, func() error {
		_, tree, err := s.loadForBuild(ctx, ns)
		if err != nil {
			return err
		}
		res, kit, err := s.buildDraft(ctx, tree)
		if err != nil {
			return err
		}
		out = BuildReport{Diagnostics: nonNil(res.Diagnostics), HasErrors: hasErrors(res.Diagnostics), SourceHash: res.SourceHash, ArtifactHash: res.ArtifactHash, KitHash: kit.Hash}
		if !out.HasErrors {
			m := res.Manifest
			out.Manifest = &m
		}
		e.SourceHash, e.ArtifactHash = res.SourceHash, res.ArtifactHash
		e.Detail["hasErrors"], e.Detail["diagnostics"], e.Detail["kitHash"] = out.HasErrors, len(res.Diagnostics), kit.Hash
		return nil
	})
	return out, err
}

// Test runs the package's synthetic tests against fake effects.
func (s *Service) Test(ctx context.Context, actor spi.Subject, ns string) (build.TestReport, error) {
	e := &auditEntry{Action: ActionTest, Namespace: ns, Detail: map[string]any{}}
	var out build.TestReport
	err := s.observe(ctx, actor, PermAuthor, e, func() error {
		_, tree, err := s.loadForBuild(ctx, ns)
		if err != nil {
			return err
		}
		e.SourceHash = artifact.TreeHash(tree)
		kit, err := s.Kit(ctx)
		if err != nil {
			return err
		}
		out, err = s.builder.Test(ctx, tree, kit)
		if err != nil {
			return serrors.Wrap("extruntime.manage.Test", err)
		}
		e.Detail["passed"], e.Detail["cases"], e.Detail["kitHash"] = out.Passed, len(out.Cases), kit.Hash
		return nil
	})
	return out, err
}

var versionLabelPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$`)

const versionColumns = `v.id, v.namespace, v.version, v.source_hash, v.tree_hash, v.artifact_hash, v.lockfile_name, v.lockfile_hash,
	v.kit_hash, v.provenance, v.repo_url, v.repo_commit, v.diagnostics, v.author_id, v.created_at, v.manifest,
	r.id, r.approve, r.reviewer_id, r.comment, r.source_hash, r.artifact_hash, r.created_at,
	EXISTS (SELECT 1 FROM extruntime.activations a WHERE a.version_id = v.id AND a.state = 'active')`

const versionFrom = ` FROM extruntime.versions v
	LEFT JOIN LATERAL (SELECT * FROM extruntime.version_reviews WHERE version_id = v.id ORDER BY id DESC LIMIT 1) r ON true`

func scanVersion(row pgx.Row) (VersionDetail, error) {
	var v VersionDetail
	var prov string
	var diag, man []byte
	var rid *int64
	var rapprove *bool
	var rrev *int64
	var rcomment, rsrc, rart *string
	var rat *time.Time
	err := row.Scan(&v.ID, &v.Namespace, &v.Version.Version, &v.SourceHash, &v.TreeHash, &v.ArtifactHash, &v.LockfileName, &v.LockfileHash,
		&v.KitHash, &prov, &v.RepoURL, &v.RepoCommit, &diag, &v.AuthorID, &v.CreatedAt, &man,
		&rid, &rapprove, &rrev, &rcomment, &rsrc, &rart, &rat, &v.Active)
	if err != nil {
		return v, err
	}
	v.Provenance = Provenance(prov)
	if err := json.Unmarshal(diag, &v.Diagnostics); err != nil {
		return v, err
	}
	if err := json.Unmarshal(man, &v.Manifest); err != nil {
		return v, err
	}
	if rid != nil {
		v.LatestReview = &Review{ID: *rid, Approve: *rapprove, ReviewerID: *rrev, Comment: *rcomment, SourceHash: *rsrc, ArtifactHash: *rart, At: *rat}
		v.Approved = v.LatestReview.Approve && v.LatestReview.SourceHash == v.SourceHash && v.LatestReview.ArtifactHash == v.ArtifactHash
	}
	return v, nil
}

func (s *Service) versionRow(ctx context.Context, q querier, ns, version string) (VersionDetail, error) {
	v, err := scanVersion(q.QueryRow(ctx, `SELECT `+versionColumns+versionFrom+` WHERE v.namespace = $1 AND v.version = $2`, ns, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, notFound("version " + ns + "@" + version)
	}
	if err != nil {
		return v, serrors.FromDB("extruntime.manage.versionRow", err)
	}
	return v, nil
}

// Publish builds the current draft against the current kit snapshot and
// stores the result as a new immutable version. It is refused when the build
// has error diagnostics. version defaults to the package.json version and
// must be new for the package.
func (s *Service) Publish(ctx context.Context, actor spi.Subject, ns, version string) (VersionDetail, error) {
	const op = serrors.Op("extruntime.manage.Publish")
	e := &auditEntry{Action: ActionPublish, Namespace: ns, Version: version, Detail: map[string]any{}}
	if !actor.Has(PermPublish) {
		return VersionDetail{}, s.deny(ctx, actor, PermPublish, *e)
	}
	fail := func(err error) (VersionDetail, error) {
		s.failed(ctx, actor, *e, err)
		return VersionDetail{}, err
	}
	pkg, tree, err := s.loadForBuild(ctx, ns)
	if err != nil {
		return fail(err)
	}
	treeHash := artifact.TreeHash(tree)
	e.SourceHash = treeHash
	if version == "" {
		pj, perr := parsePackageJSON(tree)
		if perr != nil {
			return fail(perr)
		}
		version = pj.Version
		e.Version = version
	}
	if !versionLabelPattern.MatchString(version) {
		return fail(serrors.New(serrors.Invalid, fmt.Sprintf("invalid version %q", version)))
	}
	res, kit, built, err := s.buildDraftContracts(ctx, tree)
	if err != nil {
		return fail(err)
	}
	e.SourceHash, e.ArtifactHash = res.SourceHash, res.ArtifactHash
	e.Detail["kitHash"] = kit.Hash
	var foreign map[string]string
	if !hasErrors(res.Diagnostics) {
		extra, f, cerr := s.checkImplementations(ctx, ns, res.Manifest, built)
		if cerr != nil {
			return fail(cerr)
		}
		res.Diagnostics = append(res.Diagnostics, extra...)
		foreign = f
	}
	if hasErrors(res.Diagnostics) {
		e.Detail["diagnostics"] = len(res.Diagnostics)
		return fail(serrors.New(serrors.FailedPrecondition, "publish refused: the build has errors").
			WithOp(op).WithCause(&DiagnosticsError{Diagnostics: res.Diagnostics}))
	}
	m := res.Manifest
	if m.Namespace != ns {
		return fail(serrors.New(serrors.FailedPrecondition, fmt.Sprintf("manifest namespace %q differs from package %q", m.Namespace, ns)))
	}
	if err := m.Validate(); err != nil {
		return fail(serrors.New(serrors.FailedPrecondition, err.Error()).WithOp(op))
	}
	if res.SourceHash == "" || res.ArtifactHash == "" || len(res.Artifacts) == 0 {
		return fail(serrors.New(serrors.Internal, "the builder returned no artifact").WithOp(op))
	}
	resources := make([]manifest.Resource, len(m.Resources))
	copy(resources, m.Resources)
	for i := range resources {
		resources[i].ContractHash = manifest.ContractHashOf(resources[i])
	}
	m.Resources = resources

	srcTar, err := artifact.Pack(tree)
	if err != nil {
		return fail(err)
	}
	buildTar, err := artifact.Pack(res.Artifacts)
	if err != nil {
		return fail(err)
	}
	srcArchive := artifact.HashBytes(srcTar)
	manJSON, err := json.Marshal(m)
	if err != nil {
		return fail(serrors.Wrap(op, err))
	}
	diagJSON, err := json.Marshal(nonNil(res.Diagnostics))
	if err != nil {
		return fail(serrors.Wrap(op, err))
	}
	foreignJSON, err := json.Marshal(foreign)
	if err != nil {
		return fail(serrors.Wrap(op, err))
	}
	lockName, lockHash := lockfileOf(tree)

	var out VersionDetail
	err = s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		cur, err := s.packageRow(ctx, tx, ns, true)
		if err != nil {
			return err
		}
		if cur.DraftRev != pkg.DraftRev {
			return serrors.New(serrors.Conflict, "the draft changed while it was being built; publish again")
		}
		var imported string
		if err := tx.QueryRow(ctx, `SELECT imported_tree_hash FROM extruntime.packages WHERE namespace = $1`, ns).Scan(&imported); err != nil {
			return serrors.FromDB(op, err)
		}
		prov, repo, commit := ProvenanceRuntime, "", ""
		if cur.Provenance == ProvenanceGit && imported == treeHash {
			prov, repo, commit = ProvenanceGit, cur.RepoURL, cur.RepoCommit
		}
		if err := s.arts.Put(ctx, tx, srcArchive, artifact.KindSource, srcTar); err != nil {
			return err
		}
		if err := s.arts.Put(ctx, tx, res.ArtifactHash, artifact.KindBuild, buildTar); err != nil {
			return err
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO extruntime.versions
			(namespace, version, source_hash, tree_hash, source_archive, artifact_hash, lockfile_name, lockfile_hash, manifest,
			 kit_hash, provenance, repo_url, repo_commit, diagnostics, foreign_contracts, author_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $14::jsonb, $15::jsonb, $16) RETURNING id::text`,
			ns, version, res.SourceHash, treeHash, srcArchive, res.ArtifactHash, lockName, lockHash, string(manJSON),
			kit.Hash, string(prov), repo, commit, string(diagJSON), string(foreignJSON), actor.UserID).Scan(&id); err != nil {
			if isUnique(err) {
				return serrors.New(serrors.AlreadyExists, fmt.Sprintf("version %s of %s is already published", version, ns)).WithOp(op)
			}
			return serrors.FromDB(op, err)
		}
		out, err = s.versionRow(ctx, tx, ns, version)
		return err
	})
	return out, err
}
