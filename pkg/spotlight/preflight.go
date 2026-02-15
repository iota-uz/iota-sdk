package spotlight

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func PreflightCheck(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("spotlight preflight: database pool is nil")
	}

	var versionNum int
	if err := pool.QueryRow(ctx, `SHOW server_version_num`).Scan(&versionNum); err != nil {
		return fmt.Errorf("spotlight preflight: failed to read PostgreSQL version: %w", err)
	}
	if versionNum < 170000 {
		return fmt.Errorf("spotlight preflight: PostgreSQL 17+ is required, got server_version_num=%d", versionNum)
	}

	requiredExtensions := []string{"pg_textsearch", "vector"}
	for _, extension := range requiredExtensions {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)`, extension).Scan(&exists); err != nil {
			return fmt.Errorf("spotlight preflight: failed to check extension %s: %w", extension, err)
		}
		if !exists {
			return fmt.Errorf("spotlight preflight: required extension %s is not installed", extension)
		}
	}

	return nil
}

func MustPreflight(ctx context.Context, pool *pgxpool.Pool) {
	if err := PreflightCheck(ctx, pool); err != nil {
		panic(err)
	}
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
