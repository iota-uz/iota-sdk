package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)

// ValidateNamespace checks the stable dotted owner prefix of a package.
func ValidateNamespace(ns string) error {
	if len(ns) > 100 || !namespacePattern.MatchString(ns) {
		return serrors.New(serrors.Invalid, fmt.Sprintf("invalid namespace %q", ns))
	}
	return nil
}

type packageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Iota    struct {
		Namespace string `json:"namespace"`
	} `json:"iota"`
}

// ParsePackageJSON reads the name, version and namespace of a tree.
func parsePackageJSON(t build.Tree) (packageJSON, error) {
	var pj packageJSON
	raw, ok := t["package.json"]
	if !ok {
		return pj, serrors.New(serrors.Invalid, "the tree has no package.json")
	}
	if err := json.Unmarshal(raw, &pj); err != nil {
		return pj, serrors.New(serrors.Invalid, "package.json is not valid JSON: "+err.Error())
	}
	return pj, nil
}

// TreeNamespace is the namespace declared by a tree's package.json.
func TreeNamespace(t build.Tree) (string, error) {
	pj, err := parsePackageJSON(t)
	if err != nil {
		return "", err
	}
	if err := ValidateNamespace(pj.Iota.Namespace); err != nil {
		return "", serrors.New(serrors.Invalid, "package.json iota.namespace: "+err.Error())
	}
	return pj.Iota.Namespace, nil
}

const packageColumns = `p.namespace, p.npm_name, p.provenance, p.repo_url, p.repo_commit, p.draft_rev,
	p.created_by, p.created_at, p.updated_by, p.updated_at,
	coalesce((SELECT v.version FROM extruntime.activations a JOIN extruntime.versions v ON v.id = a.version_id
	          WHERE a.namespace = p.namespace AND a.state = 'active'), '')`

func scanPackage(row pgx.Row) (Package, error) {
	var p Package
	var prov string
	err := row.Scan(&p.Namespace, &p.NPMName, &prov, &p.RepoURL, &p.RepoCommit, &p.DraftRev,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedBy, &p.UpdatedAt, &p.ActiveVersion)
	p.Provenance = Provenance(prov)
	return p, err
}

func notFound(what string) error { return serrors.New(serrors.NotFound, what+" not found") }

func (s *Service) packageRow(ctx context.Context, q querier, ns string, lock bool) (Package, error) {
	sql := `SELECT ` + packageColumns + ` FROM extruntime.packages p WHERE p.namespace = $1`
	if lock {
		sql += ` FOR UPDATE OF p`
	}
	p, err := scanPackage(q.QueryRow(ctx, sql, ns))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, notFound("package " + ns)
	}
	if err != nil {
		return p, serrors.FromDB("extruntime.manage.packageRow", err)
	}
	return p, nil
}

// GetPackage returns a package head.
func (s *Service) GetPackage(ctx context.Context, actor spi.Subject, ns string) (Package, error) {
	if err := require(actor, PermRead); err != nil {
		return Package{}, err
	}
	return s.packageRow(ctx, s.db, ns, false)
}

// ListPackages lists all packages ordered by namespace.
func (s *Service) ListPackages(ctx context.Context, actor spi.Subject) ([]Package, error) {
	const op = serrors.Op("extruntime.manage.ListPackages")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT `+packageColumns+` FROM extruntime.packages p ORDER BY p.namespace`)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		p, err := scanPackage(rows)
		if err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, p)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// CreatePackage creates an empty runtime-issued package with a minimal
// scaffold: package.json, src/contracts.ts and src/server.ts.
func (s *Service) CreatePackage(ctx context.Context, actor spi.Subject, namespace, npmName string) (Package, error) {
	const op = serrors.Op("extruntime.manage.CreatePackage")
	e := &auditEntry{Action: ActionCreatePackage, Namespace: namespace}
	var out Package
	err := s.mutate(ctx, actor, PermAuthor, e, func(tx pgx.Tx) error {
		if err := ValidateNamespace(namespace); err != nil {
			return err
		}
		if s.catalog.Natives().Reserved(namespace) {
			return serrors.New(serrors.Invalid, "namespace "+namespace+" is reserved by a native module")
		}
		if npmName == "" {
			npmName = "@" + strings.ReplaceAll(namespace, ".", "/")
		}
		pj, _ := json.MarshalIndent(map[string]any{
			"name": npmName, "version": "0.1.0", "type": "module",
			"iota": map[string]any{
				"namespace": namespace, "contracts": "./src/contracts.ts", "server": "./src/server.ts",
			},
			"dependencies": map[string]string{"@iota-uz/sdk": "*"},
		}, "", "  ")
		tree := build.Tree{
			"package.json":     append(pj, '\n'),
			"src/contracts.ts": []byte("export {}\n"),
			"src/server.ts":    []byte("export {}\n"),
		}
		if _, err := tx.Exec(ctx, `INSERT INTO extruntime.packages (namespace, npm_name, provenance, created_by, updated_by)
			VALUES ($1, $2, 'runtime', $3, $3)`, namespace, npmName, actor.UserID); err != nil {
			if isUnique(err) {
				return serrors.New(serrors.AlreadyExists, "package "+namespace+" already exists").WithOp(op)
			}
			return serrors.FromDB(op, err)
		}
		if err := writeTree(ctx, tx, namespace, tree, actor.UserID); err != nil {
			return err
		}
		var err error
		out, err = s.packageRow(ctx, tx, namespace, false)
		return err
	})
	return out, err
}

func isUnique(err error) bool {
	var pe interface{ SQLState() string }
	return errors.As(err, &pe) && pe.SQLState() == "23505"
}

func writeTree(ctx context.Context, tx pgx.Tx, ns string, tree build.Tree, user int64) error {
	const op = serrors.Op("extruntime.manage.writeTree")
	if _, err := tx.Exec(ctx, `DELETE FROM extruntime.package_files WHERE namespace = $1`, ns); err != nil {
		return serrors.FromDB(op, err)
	}
	for _, p := range artifact.Paths(tree) {
		if _, err := tx.Exec(ctx, `INSERT INTO extruntime.package_files (namespace, path, content, hash, updated_by)
			VALUES ($1, $2, $3, $4, $5)`, ns, p, tree[p], artifact.HashBytes(tree[p]), user); err != nil {
			return serrors.FromDB(op, err)
		}
	}
	return nil
}

func (s *Service) draftTree(ctx context.Context, q querier, ns string) (build.Tree, error) {
	const op = serrors.Op("extruntime.manage.draftTree")
	rows, err := q.Query(ctx, `SELECT path, content FROM extruntime.package_files WHERE namespace = $1`, ns)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	t := build.Tree{}
	for rows.Next() {
		var p string
		var c []byte
		if err := rows.Scan(&p, &c); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		t[p] = c
	}
	return t, serrors.FromDB(op, rows.Err())
}

// DraftTree returns the whole draft source tree.
func (s *Service) DraftTree(ctx context.Context, actor spi.Subject, ns string) (build.Tree, error) {
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	if _, err := s.packageRow(ctx, s.db, ns, false); err != nil {
		return nil, err
	}
	return s.draftTree(ctx, s.db, ns)
}

// ListFiles lists the draft files (path, hash, size).
func (s *Service) ListFiles(ctx context.Context, actor spi.Subject, ns string) ([]File, error) {
	const op = serrors.Op("extruntime.manage.ListFiles")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	if _, err := s.packageRow(ctx, s.db, ns, false); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT path, hash, octet_length(content), updated_by, updated_at
		FROM extruntime.package_files WHERE namespace = $1 ORDER BY path`, ns)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.Hash, &f.Size, &f.UpdatedBy, &f.UpdatedAt); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, f)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// GetFile returns one draft file.
func (s *Service) GetFile(ctx context.Context, actor spi.Subject, ns, path string) ([]byte, File, error) {
	const op = serrors.Op("extruntime.manage.GetFile")
	if err := require(actor, PermRead); err != nil {
		return nil, File{}, err
	}
	var f File
	var c []byte
	err := s.db.QueryRow(ctx, `SELECT path, content, hash, octet_length(content), updated_by, updated_at
		FROM extruntime.package_files WHERE namespace = $1 AND path = $2`, ns, path).
		Scan(&f.Path, &c, &f.Hash, &f.Size, &f.UpdatedBy, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, File{}, notFound("file " + path)
	}
	if err != nil {
		return nil, File{}, serrors.FromDB(op, err)
	}
	return c, f, nil
}

// bumpDraft records an edit of the draft.
func bumpDraft(ctx context.Context, tx pgx.Tx, ns string, user int64) error {
	_, err := tx.Exec(ctx, `UPDATE extruntime.packages SET draft_rev = draft_rev + 1, updated_by = $2, updated_at = CURRENT_TIMESTAMP
		WHERE namespace = $1`, ns, user)
	return serrors.FromDB("extruntime.manage.bumpDraft", err)
}

// PutFile creates or replaces one draft file. expectHash guards concurrent
// editors: nil skips the check, a pointer to "" requires the file to be new,
// otherwise the current content hash must match.
func (s *Service) PutFile(ctx context.Context, actor spi.Subject, ns, path string, content []byte, expectHash *string) (File, error) {
	const op = serrors.Op("extruntime.manage.PutFile")
	e := &auditEntry{Action: ActionPutFile, Namespace: ns, Target: path}
	var out File
	err := s.mutate(ctx, actor, PermAuthor, e, func(tx pgx.Tx) error {
		clean, err := artifact.CleanPath(path)
		if err != nil {
			return err
		}
		e.Target = clean
		if len(content) > artifact.MaxFileBytes {
			return serrors.New(serrors.Invalid, fmt.Sprintf("file %q is larger than %d bytes", clean, artifact.MaxFileBytes))
		}
		if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		var current string
		var exists bool
		switch err := tx.QueryRow(ctx, `SELECT hash FROM extruntime.package_files WHERE namespace = $1 AND path = $2`, ns, clean).Scan(&current); {
		case err == nil:
			exists = true
		case !errors.Is(err, pgx.ErrNoRows):
			return serrors.FromDB(op, err)
		}
		if expectHash != nil && *expectHash != current {
			return serrors.New(serrors.Conflict, "file "+clean+" was changed by someone else").WithOp(op)
		}
		if !exists {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM extruntime.package_files WHERE namespace = $1`, ns).Scan(&n); err != nil {
				return serrors.FromDB(op, err)
			}
			if n >= artifact.MaxFiles {
				return serrors.New(serrors.Invalid, "the draft has too many files")
			}
		}
		hash := artifact.HashBytes(content)
		if _, err := tx.Exec(ctx, `INSERT INTO extruntime.package_files (namespace, path, content, hash, updated_by)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (namespace, path) DO UPDATE SET content = EXCLUDED.content, hash = EXCLUDED.hash,
				updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`, ns, clean, content, hash, actor.UserID); err != nil {
			return serrors.FromDB(op, err)
		}
		e.SourceHash = hash
		if err := bumpDraft(ctx, tx, ns, actor.UserID); err != nil {
			return err
		}
		out = File{Path: clean, Hash: hash, Size: len(content), UpdatedBy: actor.UserID}
		return nil
	})
	return out, err
}

// DeleteFile removes one draft file.
func (s *Service) DeleteFile(ctx context.Context, actor spi.Subject, ns, path string) error {
	const op = serrors.Op("extruntime.manage.DeleteFile")
	e := &auditEntry{Action: ActionDeleteFile, Namespace: ns, Target: path}
	return s.mutate(ctx, actor, PermAuthor, e, func(tx pgx.Tx) error {
		if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		if path == "package.json" {
			return serrors.New(serrors.Invalid, "package.json cannot be deleted")
		}
		tag, err := tx.Exec(ctx, `DELETE FROM extruntime.package_files WHERE namespace = $1 AND path = $2`, ns, path)
		if err != nil {
			return serrors.FromDB(op, err)
		}
		if tag.RowsAffected() == 0 {
			return notFound("file " + path)
		}
		return bumpDraft(ctx, tx, ns, actor.UserID)
	})
}

// MoveFile renames a draft file, keeping its content.
func (s *Service) MoveFile(ctx context.Context, actor spi.Subject, ns, from, to string) error {
	const op = serrors.Op("extruntime.manage.MoveFile")
	e := &auditEntry{Action: ActionMoveFile, Namespace: ns, Target: from + " -> " + to}
	return s.mutate(ctx, actor, PermAuthor, e, func(tx pgx.Tx) error {
		cleanTo, err := artifact.CleanPath(to)
		if err != nil {
			return err
		}
		if from == "package.json" || cleanTo == "package.json" {
			return serrors.New(serrors.Invalid, "package.json cannot be moved")
		}
		if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM extruntime.package_files WHERE namespace = $1 AND path = $2)`, ns, cleanTo).Scan(&exists); err != nil {
			return serrors.FromDB(op, err)
		}
		if exists {
			return serrors.New(serrors.AlreadyExists, "file "+cleanTo+" already exists")
		}
		tag, err := tx.Exec(ctx, `UPDATE extruntime.package_files SET path = $3, updated_by = $4, updated_at = CURRENT_TIMESTAMP
			WHERE namespace = $1 AND path = $2`, ns, from, cleanTo, actor.UserID)
		if err != nil {
			return serrors.FromDB(op, err)
		}
		if tag.RowsAffected() == 0 {
			return notFound("file " + from)
		}
		return bumpDraft(ctx, tx, ns, actor.UserID)
	})
}

// ImportMeta says where an imported tree came from.
type ImportMeta struct {
	Provenance Provenance
	RepoURL    string
	RepoCommit string
}

// ImportTree replaces (or creates) the draft of the package named by the
// tree's package.json with the given tree. Paths and content are kept
// byte-for-byte, so exporting the draft again gives the same tree hash.
func (s *Service) ImportTree(ctx context.Context, actor spi.Subject, tree build.Tree, meta ImportMeta) (Package, error) {
	return s.importTree(ctx, actor, ActionImportTree, tree, meta)
}

func (s *Service) importTree(ctx context.Context, actor spi.Subject, action string, tree build.Tree, meta ImportMeta) (Package, error) {
	const op = serrors.Op("extruntime.manage.ImportTree")
	e := &auditEntry{Action: action, Detail: map[string]any{"provenance": string(meta.Provenance), "repoUrl": meta.RepoURL, "repoCommit": meta.RepoCommit}}
	var out Package
	err := s.mutate(ctx, actor, PermAuthor, e, func(tx pgx.Tx) error {
		if err := artifact.Validate(tree); err != nil {
			return err
		}
		ns, err := TreeNamespace(tree)
		if err != nil {
			return err
		}
		e.Namespace = ns
		if s.catalog.Natives().Reserved(ns) {
			return serrors.New(serrors.Invalid, "namespace "+ns+" is reserved by a native module")
		}
		if meta.Provenance == "" {
			meta.Provenance = ProvenanceRuntime
		}
		if meta.Provenance == ProvenanceGit && (meta.RepoURL == "" || meta.RepoCommit == "") {
			return serrors.New(serrors.Invalid, "git provenance needs a repository URL and a commit")
		}
		if meta.Provenance == ProvenanceRuntime {
			meta.RepoURL, meta.RepoCommit = "", ""
		}
		pj, _ := parsePackageJSON(tree)
		hash := artifact.TreeHash(tree)
		e.SourceHash = hash
		imported := ""
		if meta.Provenance == ProvenanceGit {
			imported = hash
		}
		tag, err := tx.Exec(ctx, `UPDATE extruntime.packages SET npm_name = $2, provenance = $3, repo_url = $4, repo_commit = $5,
				imported_tree_hash = $6, draft_rev = draft_rev + 1, updated_by = $7, updated_at = CURRENT_TIMESTAMP
			WHERE namespace = $1`, ns, pj.Name, string(meta.Provenance), meta.RepoURL, meta.RepoCommit, imported, actor.UserID)
		if err != nil {
			return serrors.FromDB(op, err)
		}
		if tag.RowsAffected() == 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO extruntime.packages
				(namespace, npm_name, provenance, repo_url, repo_commit, imported_tree_hash, draft_rev, created_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7)`, ns, pj.Name, string(meta.Provenance), meta.RepoURL, meta.RepoCommit, imported, actor.UserID); err != nil {
				return serrors.FromDB(op, err)
			}
		} else if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		if err := writeTree(ctx, tx, ns, tree, actor.UserID); err != nil {
			return err
		}
		out, err = s.packageRow(ctx, tx, ns, false)
		return err
	})
	return out, err
}

// lockfileOf finds the package manager lockfile of a tree.
func lockfileOf(t build.Tree) (string, string) {
	for _, n := range []string{"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"} {
		if c, ok := t[n]; ok {
			return n, artifact.HashBytes(c)
		}
	}
	return "", ""
}
