package application

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	migrate "github.com/rubenv/sql-migrate"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SquashDirective is the header line by which a baseline migration names one
// migration it replaces, one id (file name) per line:
//
//	-- +squashes changes-1740741698.sql
//
// The lines go before "-- +migrate Up"; sql-migrate itself ignores them. The
// files of the replaced migrations are removed from the migrations directory.
//
// A baseline does not upgrade databases built from the history it replaces: a
// database whose tracking table lists any replaced id is refused with
// [SquashedHistoryError] and has to be recreated. On a new database the
// baseline runs like any other migration.
const SquashDirective = "-- +squashes "

const opMigrationsSquashedHistory serrors.Op = "application.MigrationManager.squashedHistory"

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

// SquashedHistoryError is returned for a database built from migrations a
// baseline replaced. The release that squashed them does not upgrade such a
// database: recreate it (a new database gets the baseline).
type SquashedHistoryError struct {
	Baseline string
	// Found are the replaced ids the database lists, sorted.
	Found []string
}

func (e *SquashedHistoryError) Error() string {
	shown := e.Found
	more := ""
	if len(shown) > 3 {
		more = fmt.Sprintf(" and %d more", len(shown)-3)
		shown = shown[:3]
	}
	return fmt.Sprintf(
		"database was built from migrations squashed into %s (it lists %s%s); "+
			"this SDK release does not upgrade such databases: recreate the database",
		e.Baseline, strings.Join(shown, ", "), more,
	)
}

// checkSquashedHistory refuses a database whose tracking table lists any id a
// baseline replaced, before anything is planned or written.
func checkSquashedHistory(baselines []squashedBaseline, applied map[string]struct{}) error {
	for _, b := range baselines {
		var found []string
		for _, id := range b.replaces {
			if _, ok := applied[id]; ok {
				found = append(found, id)
			}
		}
		if len(found) > 0 {
			sort.Strings(found)
			return serrors.Wrap(opMigrationsSquashedHistory, &SquashedHistoryError{Baseline: b.id, Found: found})
		}
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
