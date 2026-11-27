package application

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	migrate "github.com/rubenv/sql-migrate"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SquashDirective is the header line by which a baseline migration names one
// migration it replaces:
//
//	-- +squashes changes-1740741698.sql
//
// The lines go before "-- +migrate Up"; sql-migrate itself ignores them. A
// database that already applied every replaced migration has the baseline's
// schema, so the migrator records the baseline as applied without running it
// (see [MigrationManager]). The files of the replaced migrations are removed
// from the migrations directory.
const SquashDirective = "-- +squashes "

const opMigrationsAdoptBaseline serrors.Op = "application.MigrationManager.adoptBaseline"

// squashedBaseline is one baseline migration and the ids it replaces.
type squashedBaseline struct {
	id       string
	replaces []string
}

// readSquashedBaselines returns the baselines among source, read from the
// directive header of each migration file in dir.
func readSquashedBaselines(dir string, source []*migrate.Migration) ([]squashedBaseline, error) {
	baselines := make([]squashedBaseline, 0, 1)
	present := make(map[string]struct{}, len(source))
	for _, mig := range source {
		present[mig.Id] = struct{}{}
	}
	replacedBy := map[string]string{}
	for _, mig := range source {
		replaces, err := readSquashDirectives(filepath.Join(dir, mig.Id))
		if err != nil {
			return nil, fmt.Errorf("read squash header of %s: %w", mig.Id, err)
		}
		if len(replaces) == 0 {
			continue
		}
		for _, id := range replaces {
			if _, ok := present[id]; ok {
				return nil, fmt.Errorf("baseline %s squashes %s, but %s is still in the migrations directory", mig.Id, id, id)
			}
			if other, ok := replacedBy[id]; ok {
				return nil, fmt.Errorf("migration %s is squashed by both %s and %s", id, other, mig.Id)
			}
			replacedBy[id] = mig.Id
		}
		baselines = append(baselines, squashedBaseline{id: mig.Id, replaces: replaces})
	}
	return baselines, nil
}

// readSquashDirectives reads the "-- +squashes" lines of a migration file's
// header, which ends at the first "-- +migrate" command.
func readSquashDirectives(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var ids []string
	seen := map[string]struct{}{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "-- +migrate ") {
			break
		}
		if !strings.HasPrefix(line, SquashDirective) {
			continue
		}
		for _, id := range strings.Fields(strings.TrimPrefix(line, SquashDirective)) {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids, scanner.Err()
}

// squashedIDs is the set of every id some baseline replaces. The planner
// tolerates them in the tracking table: a database adopted by an older
// migrator, or by hand, may still list them next to the baseline.
func squashedIDs(baselines []squashedBaseline) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, b := range baselines {
		for _, id := range b.replaces {
			ids[id] = struct{}{}
		}
	}
	return ids
}

// PartialSquashedHistoryError is returned when a database applied some, but
// not all, of the migrations a baseline replaces. Such a database has neither
// the old schema nor the baseline's, and no migration in the directory can
// bring it forward. Migrate it with the last release that still ships the
// replaced migrations first, then upgrade.
type PartialSquashedHistoryError struct {
	Baseline string
	Applied  int
	Total    int
	Missing  []string
}

func (e *PartialSquashedHistoryError) Error() string {
	shown := e.Missing
	more := ""
	if len(shown) > 5 {
		more = fmt.Sprintf(" and %d more", len(shown)-5)
		shown = shown[:5]
	}
	return fmt.Sprintf(
		"database applied %d of the %d migrations squashed into %s (missing %s%s): "+
			"migrate it up with the last release that still ships those migrations, then upgrade; "+
			"a database that never applied any of them gets the baseline instead",
		e.Applied, e.Total, e.Baseline, strings.Join(shown, ", "), more,
	)
}

// adoptSquashedBaselines records each baseline whose replaced migrations are
// all applied as applied itself, without running it, and drops the replaced
// ids from the tracking table in the same transaction. It updates applied in
// place. A baseline none of whose migrations are applied is left pending, and
// runs like any other migration.
func adoptSquashedBaselines(ctx context.Context, conn *pgx.Conn, baselines []squashedBaseline, applied map[string]struct{}) ([]string, error) {
	adopted := make([]string, 0, len(baselines))
	for _, b := range baselines {
		if _, ok := applied[b.id]; ok {
			// Adopted elsewhere (by hand, or by a consumer's own tooling) with
			// the old ids left behind. Drop them: once the baseline is rolled
			// back they would otherwise make the next run adopt it again
			// instead of running it.
			var stale []string
			for _, id := range b.replaces {
				if _, ok := applied[id]; ok {
					stale = append(stale, id)
				}
			}
			if len(stale) > 0 {
				if _, err := conn.Exec(ctx, `DELETE FROM `+migrationTable+` WHERE id = ANY($1)`, stale); err != nil {
					return adopted, serrors.Wrap(opMigrationsAdoptBaseline, fmt.Errorf("clear ids squashed into %s: %w", b.id, err))
				}
				for _, id := range stale {
					delete(applied, id)
				}
			}
			continue
		}
		var missing []string
		for _, id := range b.replaces {
			if _, ok := applied[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) == len(b.replaces) {
			continue
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return adopted, serrors.Wrap(opMigrationsAdoptBaseline, &PartialSquashedHistoryError{
				Baseline: b.id,
				Applied:  len(b.replaces) - len(missing),
				Total:    len(b.replaces),
				Missing:  missing,
			})
		}
		if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM `+migrationTable+` WHERE id = ANY($1)`, b.replaces); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO `+migrationTable+` (id, applied_at) VALUES ($1, $2)`, b.id, time.Now())
			return err
		}); err != nil {
			return adopted, serrors.Wrap(opMigrationsAdoptBaseline, fmt.Errorf("adopt baseline %s: %w", b.id, err))
		}
		for _, id := range b.replaces {
			delete(applied, id)
		}
		applied[b.id] = struct{}{}
		adopted = append(adopted, b.id)
	}
	return adopted, nil
}
