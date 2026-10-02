package serrors

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func constraintName(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.ConstraintName
	}
	return ""
}
