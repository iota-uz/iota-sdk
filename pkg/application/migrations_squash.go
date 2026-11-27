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
// migration it replaces, one id (file name) per line:
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
		// One id per line, taken whole: sql-migrate ids are file names, which
		// may contain spaces.
		id := strings.TrimSpace(strings.TrimPrefix(line, SquashDirective))
		if _, dup := seen[id]; id == "" || dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
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

// squashAdoption is what a run writes to the tracking table for the
// baselines: the baselines to record as applied in place of their replaced
// ids, and leftover replaced ids to clear next to a baseline already applied.
type squashAdoption struct {
	adopt []squashedBaseline
	stale []string
}

func (a squashAdoption) empty() bool { return len(a.adopt) == 0 && len(a.stale) == 0 }

// planSquashAdoption decides, without writing anything, how the tracking table
// moves onto the baselines, and applies that to applied in place:
//
//   - every replaced id applied: the baseline is adopted (recorded without
//     running) and its replaced ids go;
//   - some applied: PartialSquashedHistoryError, and applied is left untouched;
//   - none applied: the baseline stays pending and runs like any migration;
//   - baseline already applied with replaced ids left behind (adopted by hand
//     or by other tooling): those ids are cleared, since after the baseline is
//     rolled back they would make the next run adopt it instead of running it.
//
// Every baseline is classified before applied changes, so a refusal for one
// leaves no adoption of another half-planned.
func planSquashAdoption(baselines []squashedBaseline, applied map[string]struct{}) (squashAdoption, error) {
	var plan squashAdoption
	for _, b := range baselines {
		var present, missing []string
		for _, id := range b.replaces {
			if _, ok := applied[id]; ok {
				present = append(present, id)
			} else {
				missing = append(missing, id)
			}
		}
		if _, ok := applied[b.id]; ok {
			plan.stale = append(plan.stale, present...)
			continue
		}
		switch {
		case len(present) == 0:
		case len(missing) > 0:
			sort.Strings(missing)
			return squashAdoption{}, serrors.Wrap(opMigrationsAdoptBaseline, &PartialSquashedHistoryError{
				Baseline: b.id,
				Applied:  len(present),
				Total:    len(b.replaces),
				Missing:  missing,
			})
		default:
			plan.adopt = append(plan.adopt, b)
		}
	}
	for _, id := range plan.stale {
		delete(applied, id)
	}
	for _, b := range plan.adopt {
		for _, id := range b.replaces {
			delete(applied, id)
		}
		applied[b.id] = struct{}{}
	}
	return plan, nil
}

// writeSquashAdoption applies plan to the tracking table in one transaction.
func writeSquashAdoption(ctx context.Context, conn *pgx.Conn, plan squashAdoption) error {
	if plan.empty() {
		return nil
	}
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if len(plan.stale) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM `+migrationTable+` WHERE id = ANY($1)`, plan.stale); err != nil {
				return err
			}
		}
		for _, b := range plan.adopt {
			if _, err := tx.Exec(ctx, `DELETE FROM `+migrationTable+` WHERE id = ANY($1)`, b.replaces); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO `+migrationTable+` (id, applied_at) VALUES ($1, $2)`, b.id, time.Now()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return serrors.Wrap(opMigrationsAdoptBaseline, fmt.Errorf("record squashed baselines: %w", err))
	}
	return nil
}

// checkBaselineBoundary refuses a bounded rollback whose plan reaches a
// baseline. Its Down drops the whole schema it created, which a "roll back the
// last N" request never means; that takes an explicit RollbackAll.
func checkBaselineBoundary(plan []plannedMigration, baselines []squashedBaseline, dir migrationDirection, limit int) error {
	if dir != migrationDown || limit == 0 {
		return nil
	}
	isBaseline := make(map[string]struct{}, len(baselines))
	for _, b := range baselines {
		isBaseline[b.id] = struct{}{}
	}
	for i, mig := range plan {
		if _, ok := isBaseline[mig.id]; ok {
			return serrors.New(serrors.Invalid, fmt.Sprintf(
				"cannot roll back %d migrations: step %d is the squashed baseline %s, whose down drops the whole schema; "+
					"roll back at most %d, or use RollbackAll (migrate down --all) to reset the database",
				limit, i+1, mig.id, i)).WithOp(opMigrationsRollback)
		}
	}
	return nil
}
