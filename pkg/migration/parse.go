package migration

import (
	"database/sql"
	"errors"
	"github.com/kulado/sqlxmigrate"
	"os"
	"path/filepath"
	"strings"
)

func LoadMigration(fn string) (*sqlxmigrate.Migration, error) {
	file, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(file), "-- +migrate Down")
	if len(parts) != 2 {
		return nil, errors.New("invalid migration")
	}
	up := strings.TrimSpace(parts[0])
	down := strings.TrimSpace(parts[1])
	return &sqlxmigrate.Migration{
		ID: strings.Split(filepath.Base(fn), ".")[0],
		Migrate: func(tx *sql.Tx) error {
			_, err := tx.Exec(up)
			return err
		},
		Rollback: func(tx *sql.Tx) error {
			_, err := tx.Exec(down)
			return err
		},
	}, nil
}
