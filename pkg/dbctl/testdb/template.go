package testdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// templateBuildAdvisoryLockKey serializes building and pruning migrated
// templates across every test binary on the cluster. It is held while one
// template is migrated, so it is a separate key from the create-database lock,
// which only ever covers a single catalog statement.
const templateBuildAdvisoryLockKey int64 = 6_073_120_419_784_512_303

const (
	// MigratedTemplatePrefix starts the name of every template built by
	// [EnsureMigratedTemplate]. The rest of the name is the fingerprint of the
	// migration set, so two checkouts with different migrations never share
	// one, and the same checkout reuses its template across runs.
	MigratedTemplatePrefix = "itf_auto_tpl_"

	// templateReadyMarker is written as the database comment once the template
	// is fully migrated and sealed. A template without it is a build that died
	// halfway and is rebuilt.
	templateReadyMarker = "itf-template ready; last used "

	// templateFingerprintVersion changes when the way a template is built
	// changes, so templates built by an older harness are not reused.
	templateFingerprintVersion = "v1"

	// templateRetention is how long an unused template is kept before the next
	// build on the cluster drops it.
	templateRetention = 7 * 24 * time.Hour

	opEnsureMigratedTemplate = serrors.Op("testdb.EnsureMigratedTemplate")
)

// MigrationsFingerprint hashes the migration files sql-migrate would read from
// dir: the top-level *.sql files, by name and content. It changes whenever a
// migration is added, removed or edited.
func MigrationsFingerprint(dir string) (string, error) {
	return migrationsFingerprintFS(os.DirFS(dir))
}

func migrationsFingerprintFS(fsys fs.FS) (string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no *.sql migrations: %w", fs.ErrNotExist)
	}
	sort.Strings(names)
	hash := sha256.New()
	hash.Write([]byte(templateFingerprintVersion + "\x00"))
	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", name, len(body))
		hash.Write(body)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// MigratedTemplateName is the template database name for a fingerprint.
func MigratedTemplateName(fingerprint string) string {
	if len(fingerprint) > 16 {
		fingerprint = fingerprint[:16]
	}
	return Name(MigratedTemplatePrefix + fingerprint)
}

// TemplateBuilder migrates the freshly created, empty database name. It must
// close every connection it opened to name before returning: the template is
// sealed afterwards and Postgres refuses to clone a database that anything is
// connected to.
type TemplateBuilder func(ctx context.Context, name string) error

type templateEntry struct {
	once sync.Once
	err  error
}

var (
	templatesMu sync.Mutex
	templates   = map[string]*templateEntry{}
)

// EnsureMigratedTemplate returns the name of a sealed template database that
// build has migrated for fingerprint, building it first when no such template
// exists yet.
//
// Every test binary of a `go test ./...` run asks for the same template; the
// cluster-wide advisory lock lets exactly one of them build it while the others
// wait and then reuse it. A template left half-built by a process that died is
// detected by its missing ready marker and rebuilt. Inside one process the
// result is cached, so the lock is taken once per binary.
//
// Templates are kept between runs and dropped by a later build once they have
// gone unused for a week.
func EnsureMigratedTemplate(ctx context.Context, db dbconfig.Config, fingerprint string, build TemplateBuilder) (string, error) {
	if fingerprint == "" {
		return "", serrors.New(serrors.Invalid, "template fingerprint is required").WithOp(opEnsureMigratedTemplate)
	}
	name := MigratedTemplateName(fingerprint)
	key := db.Host + ":" + db.Port + "/" + name

	templatesMu.Lock()
	entry, ok := templates[key]
	if !ok {
		entry = &templateEntry{}
		templates[key] = entry
	}
	templatesMu.Unlock()

	entry.once.Do(func() {
		entry.err = ensureTemplate(ctx, db, name, build)
	})
	if entry.err != nil {
		// A failed build is not cached: a later harness in the same binary may
		// run with a healthier cluster, and a cached error would fail every
		// test of the package for one transient fault.
		templatesMu.Lock()
		if templates[key] == entry {
			delete(templates, key)
		}
		templatesMu.Unlock()
		return "", serrors.Wrap(opEnsureMigratedTemplate, entry.err)
	}
	return name, nil
}

func ensureTemplate(ctx context.Context, db dbconfig.Config, name string, build TemplateBuilder) error {
	conn, err := sql.Open("postgres", AdminConnectionString(db))
	if err != nil {
		return err
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Printf("[WARNING] template %s: closing admin connection: %v", name, cerr)
		}
	}()

	return WithAdvisoryLockContext(ctx, db, templateBuildAdvisoryLockKey, "template build", lockRequired, func() error {
		ready, exists, err := templateState(ctx, conn, name)
		if err != nil {
			return err
		}
		if ready {
			return markTemplateUsed(ctx, conn, name)
		}
		if exists {
			// Only a builder holding this lock creates the database, so one that
			// exists without the marker belongs to a build that died.
			log.Printf("[WARNING] template %s: dropping an unfinished build", name)
			if err := dropTemplate(ctx, conn, name); err != nil {
				return err
			}
		}
		pruneStaleTemplates(ctx, conn, name)

		if err := createDatabase(ctx, name, "", db, false, false); err != nil {
			return fmt.Errorf("create template %s: %w", name, err)
		}
		if err := build(ctx, name); err != nil {
			if dropErr := dropTemplate(context.WithoutCancel(ctx), conn, name); dropErr != nil {
				err = errors.Join(err, fmt.Errorf("drop failed template %s: %w", name, dropErr))
			}
			return fmt.Errorf("build template %s: %w", name, err)
		}
		// Sealing the way template0 is sealed keeps autovacuum and stray
		// clients out, which is what makes every later clone succeed first time.
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER DATABASE "%s" ALLOW_CONNECTIONS false`, name)); err != nil {
			return fmt.Errorf("seal template %s: %w", name, err)
		}
		if err := terminateConnections(ctx, conn, name); err != nil {
			return err
		}
		return markTemplateUsed(ctx, conn, name)
	})
}

func templateState(ctx context.Context, conn *sql.DB, name string) (ready, exists bool, err error) {
	var allowConn bool
	var comment sql.NullString
	err = conn.QueryRowContext(ctx, `
		SELECT datallowconn, shobj_description(oid, 'pg_database')
		FROM pg_database WHERE datname = $1`, name).Scan(&allowConn, &comment)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("probe template %s: %w", name, err)
	}
	return !allowConn && strings.HasPrefix(comment.String, templateReadyMarker), true, nil
}

func markTemplateUsed(ctx context.Context, conn *sql.DB, name string) error {
	comment := templateReadyMarker + time.Now().UTC().Format(time.RFC3339)
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`COMMENT ON DATABASE "%s" IS '%s'`, name, comment)); err != nil {
		return fmt.Errorf("mark template %s ready: %w", name, err)
	}
	return nil
}

func dropTemplate(ctx context.Context, conn *sql.DB, name string) error {
	if err := terminateConnections(ctx, conn, name); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, name))
	return err
}

// pruneStaleTemplates drops migrated templates other than keep that no
// harness has used for templateRetention. It runs only while a new template is
// being built, which is when a checkout's migrations changed and the old
// template stopped being useful to it. Failures are logged, never returned:
// pruning is housekeeping and must not fail a test run.
func pruneStaleTemplates(ctx context.Context, conn *sql.DB, keep string) {
	rows, err := conn.QueryContext(ctx, `
		SELECT datname, COALESCE(shobj_description(oid, 'pg_database'), '')
		FROM pg_database WHERE datname LIKE $1`, MigratedTemplatePrefix+"%")
	if err != nil {
		log.Printf("[WARNING] list migrated templates: %v", err)
		return
	}
	var stale []string
	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			log.Printf("[WARNING] list migrated templates: %v", err)
			break
		}
		if name == keep || !strings.HasPrefix(comment, templateReadyMarker) {
			continue
		}
		used, err := time.Parse(time.RFC3339, strings.TrimPrefix(comment, templateReadyMarker))
		if err == nil && time.Since(used) > templateRetention {
			stale = append(stale, name)
		}
	}
	_ = rows.Close()
	for _, name := range stale {
		if err := dropTemplate(ctx, conn, name); err != nil {
			log.Printf("[WARNING] drop stale template %s: %v", name, err)
		}
	}
}
