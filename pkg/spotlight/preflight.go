package spotlight

import (
	"context"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

func PreflightCheck(ctx context.Context, pool *pgxpool.Pool) error {
	const op serrors.Op = "spotlight.PreflightCheck"

	if pool == nil {
		return serrors.E(op, "database pool is nil")
	}

	var versionStr string
	if err := pool.QueryRow(ctx, `SHOW server_version_num`).Scan(&versionStr); err != nil {
		return serrors.E(op, "failed to read PostgreSQL version", err)
	}
	versionNum, err := strconv.Atoi(strings.TrimSpace(versionStr))
	if err != nil {
		return serrors.E(op, "invalid PostgreSQL version number "+versionStr, err)
	}
	if versionNum < 170000 {
		return serrors.E(op, "PostgreSQL 17+ is required, got server_version_num="+strconv.Itoa(versionNum))
	}

	requiredExtensions := []string{"pg_textsearch", "vector"}
	for _, extension := range requiredExtensions {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)`, extension).Scan(&exists); err != nil {
			return serrors.E(op, "failed to check extension "+extension, err)
		}
		if !exists {
			return serrors.E(op, "required extension "+extension+" is not installed")
		}
	}

	return nil
}

func ReadinessErrorDetails(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.TrimSpace(msg) == "" {
		return "spotlight readiness failed"
	}
	return msg
}
