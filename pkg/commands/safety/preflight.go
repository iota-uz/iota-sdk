package safety

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/jackc/pgx/v5/pgxpool"
)

var backupNameMarkerRegex = regexp.MustCompile(`(?i)(prod|stage|backup|restore)`)

var knownSeedTables = []string{"tenants", "users", "roles", "permissions", "groups"}

func RunPreflight(ctx context.Context, pool *pgxpool.Pool, op OperationKind) (PreflightResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	conf := configuration.Use()
	res := PreflightResult{
		Operation: op,
		Target: TargetInfo{
			Environment: strings.TrimSpace(conf.GoAppEnvironment),
			Host:        strings.TrimSpace(conf.Database.Host),
			Port:        strings.TrimSpace(conf.Database.Port),
			Name:        strings.TrimSpace(conf.Database.Name),
			User:        strings.TrimSpace(conf.Database.User),
			Password:    "***REDACTED***",
		},
		IsDestructive: isDestructive(op),
		Actions:       actionPreview(op),
	}

	res.DBState.IsLocalHost = isLocalHost(res.Target.Host)
	if !res.DBState.IsLocalHost {
		res.Risks = append(res.Risks, Risk{
			Code:     "remote_db",
			Severity: "high",
			Message:  fmt.Sprintf("target host %q is not recognized as local", res.Target.Host),
		})
	}

	if backupNameMarkerRegex.MatchString(res.Target.Name) {
		res.DBState.LooksLikeBackup = true
		res.DBState.DetectedMarkers = append(res.DBState.DetectedMarkers, "database_name_marker")
		res.Risks = append(res.Risks, Risk{
			Code:     "backup_name_marker",
			Severity: "high",
			Message:  fmt.Sprintf("database name %q looks like production/stage/backup data", res.Target.Name),
		})
	}

	if pool == nil {
		return res, nil
	}

	counts, err := collectTableCounts(ctx, pool, knownSeedTables)
	if err != nil {
		return res, err
	}
	res.DBState.CheckedTables = counts
	for _, tc := range counts {
		if tc.Count > 0 {
			res.DBState.IsNonEmpty = true
		}
		switch tc.Table {
		case "tenants":
			res.DBState.ExistingTenants = tc.Count
		case "users":
			res.DBState.ExistingUsers = tc.Count
		case "roles":
			res.DBState.ExistingRoles = tc.Count
		case "groups":
			res.DBState.ExistingGroups = tc.Count
		case "permissions":
			res.DBState.ExistingPerms = tc.Count
		}
	}

	tenantDomains, err := collectTenantDomains(ctx, pool)
	if err != nil {
		return res, err
	}
	res.DBState.DetectedDomains = tenantDomains
	for _, domain := range tenantDomains {
		if domainLooksNonLocal(domain) {
			res.DBState.LooksLikeBackup = true
			res.DBState.DetectedMarkers = append(res.DBState.DetectedMarkers, "tenant_domain_non_local")
			res.Risks = append(res.Risks, Risk{
				Code:     "backup_tenant_marker",
				Severity: "high",
				Message:  fmt.Sprintf("tenant domain %q looks non-local", domain),
			})
			break
		}
	}

	if res.DBState.IsNonEmpty {
		res.Risks = append(res.Risks, Risk{
			Code:     "non_empty_db",
			Severity: "high",
			Message:  "database already contains data; seed operations may skip/merge existing records",
		})
	}

	return res, nil
}

func isDestructive(op OperationKind) bool {
	switch op {
	case OperationE2ECreate, OperationE2EDrop, OperationE2EReset:
		return true
	default:
		return false
	}
}

func actionPreview(op OperationKind) []string {
	switch op {
	case OperationSeedMain:
		return []string{"check current DB state", "seed default tenant", "seed currencies", "seed permissions", "seed users", "seed subscription entitlements", "seed website AI config"}
	case OperationSeedSuperadmin:
		return []string{"check current DB state", "seed default tenant", "seed currencies", "seed permissions", "seed superadmin user", "seed subscription entitlements"}
	case OperationE2ESeed:
		return []string{"check current DB state", "seed test tenant", "seed test users", "seed default entities for e2e"}
	case OperationE2ECreate:
		return []string{"drop e2e database if exists", "create fresh e2e database"}
	case OperationE2EDrop:
		return []string{"drop e2e database if exists"}
	case OperationE2EReset:
		return []string{"drop/create e2e database", "run migrations", "seed e2e data"}
	default:
		return []string{"execute operation"}
	}
}

func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	switch h {
	case "localhost", "127.0.0.1", "::1", "db", "postgres":
		return true
	default:
		return strings.HasSuffix(h, ".localhost")
	}
}

func collectTableCounts(ctx context.Context, pool *pgxpool.Pool, tables []string) ([]TableCount, error) {
	results := make([]TableCount, 0, len(tables))
	for _, table := range tables {
		exists, err := tableExists(ctx, pool, table)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		var count int64
		if err := pool.QueryRow(ctx, query).Scan(&count); err != nil {
			return nil, fmt.Errorf("count rows in %s: %w", table, err)
		}
		results = append(results, TableCount{Table: table, Count: count})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Table < results[j].Table })
	return results, nil
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, table string) (bool, error) {
	const q = `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1
	)`
	var exists bool
	if err := pool.QueryRow(ctx, q, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("check table existence for %s: %w", table, err)
	}
	return exists, nil
}

func collectTenantDomains(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	exists, err := tableExists(ctx, pool, "tenants")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	rows, err := pool.Query(ctx, "SELECT domain FROM tenants WHERE domain IS NOT NULL AND domain <> '' LIMIT 20")
	if err != nil {
		return nil, fmt.Errorf("query tenant domains: %w", err)
	}
	defer rows.Close()
	domains := make([]string, 0)
	for rows.Next() {
		var domain string
		if scanErr := rows.Scan(&domain); scanErr != nil {
			return nil, fmt.Errorf("scan tenant domain: %w", scanErr)
		}
		domains = append(domains, strings.TrimSpace(domain))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant domains: %w", err)
	}
	sort.Strings(domains)
	return domains, nil
}

func domainLooksNonLocal(domain string) bool {
	d := strings.ToLower(strings.TrimSpace(domain))
	if d == "" {
		return false
	}
	if strings.Contains(d, "localhost") || strings.Contains(d, ".local") || strings.HasSuffix(d, ".test") || strings.HasSuffix(d, ".internal") {
		return false
	}
	return true
}
