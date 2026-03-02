package safety

import "strings"

type RunOptions struct {
	DryRun bool
	Yes    bool
	Force  bool
}

type OperationKind string

const (
	OperationSeedMain       OperationKind = "seed_main"
	OperationSeedSuperadmin OperationKind = "seed_superadmin"
	OperationE2ESeed        OperationKind = "e2e_seed"
	OperationE2ECreate      OperationKind = "e2e_create"
	OperationE2EDrop        OperationKind = "e2e_drop"
	OperationE2EReset       OperationKind = "e2e_reset"
)

type Risk struct {
	Code     string
	Severity string
	Message  string
}

type TargetInfo struct {
	Environment string
	Host        string
	Port        string
	Name        string
	User        string
	Password    string
}

type TableCount struct {
	Table string
	Count int64
}

type DBState struct {
	IsLocalHost     bool
	IsNonEmpty      bool
	LooksLikeBackup bool
	CheckedTables   []TableCount
	DetectedMarkers []string
	DetectedDomains []string
	ExistingTenants int64
	ExistingUsers   int64
	ExistingRoles   int64
	ExistingGroups  int64
	ExistingPerms   int64
}

type PreflightResult struct {
	Operation     OperationKind
	Target        TargetInfo
	DBState       DBState
	Risks         []Risk
	IsDestructive bool
	Actions       []string
}

func (r PreflightResult) HasHighRisk() bool {
	for _, risk := range r.Risks {
		if strings.EqualFold(risk.Severity, "high") {
			return true
		}
	}
	return false
}
