package eval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval/dataset"
	"github.com/iota-uz/iota-sdk/pkg/bichat/testharness"
)

type CaseSource interface {
	LoadCases(ctx context.Context) ([]TestCase, error)
}

type PathCaseSource struct {
	Path string
}

func (s PathCaseSource) LoadCases(ctx context.Context) ([]TestCase, error) {
	_ = ctx
	path := strings.TrimSpace(s.Path)
	if path == "" {
		return nil, fmt.Errorf("cases path is required")
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return LoadTestCases(path)
	}
	return LoadTestCasesFromDir(path)
}

type DatasetPrepareRequest struct {
	DatasetIDs []string
	SeedTenant uuid.UUID
	Seed       bool
}

type DatasetPreparer interface {
	PrepareOracleFacts(ctx context.Context, req DatasetPrepareRequest) (map[string]dataset.Fact, error)
}

type DatasetSeeder interface {
	SeedAndBuildOracleMany(ctx context.Context, datasetIDs []string, tenantID uuid.UUID) (map[string]dataset.Fact, error)
	BuildOracleMany(ctx context.Context, datasetIDs []string, tenantID uuid.UUID) (map[string]dataset.Fact, error)
}

type SeededDatasetPreparer struct {
	Seeder DatasetSeeder
}

func (p SeededDatasetPreparer) PrepareOracleFacts(
	ctx context.Context,
	req DatasetPrepareRequest,
) (map[string]dataset.Fact, error) {
	if p.Seeder == nil {
		return nil, fmt.Errorf("dataset seeder is nil")
	}
	if len(req.DatasetIDs) == 0 {
		return map[string]dataset.Fact{}, nil
	}

	if req.Seed {
		return p.Seeder.SeedAndBuildOracleMany(ctx, req.DatasetIDs, req.SeedTenant)
	}
	return p.Seeder.BuildOracleMany(ctx, req.DatasetIDs, req.SeedTenant)
}

type SuiteRunner interface {
	Run(ctx context.Context, suite testharness.TestSuite) (testharness.RunReport, int, error)
}

type RunnerFactory interface {
	NewRunner(cfg testharness.Config) (SuiteRunner, error)
}

type TestHarnessRunnerFactory struct{}

func (TestHarnessRunnerFactory) NewRunner(cfg testharness.Config) (SuiteRunner, error) {
	return testharness.NewRunner(cfg)
}

type ExecuteRequest struct {
	Tag           string
	Category      string
	SeedTenantID  uuid.UUID
	Seed          bool
	HarnessConfig testharness.Config
}

type ExecuteResult struct {
	Cases       []TestCase
	DatasetIDs  []string
	OracleFacts map[string]dataset.Fact
	RunReport   testharness.RunReport
	ExitCode    int
}

type Pipeline struct {
	CaseSource      CaseSource
	DatasetPreparer DatasetPreparer
	RunnerFactory   RunnerFactory
}

func (p Pipeline) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	if p.CaseSource == nil {
		return ExecuteResult{}, fmt.Errorf("case source is nil")
	}
	if p.RunnerFactory == nil {
		return ExecuteResult{}, fmt.Errorf("runner factory is nil")
	}

	cases, err := p.CaseSource.LoadCases(ctx)
	if err != nil {
		return ExecuteResult{}, err
	}
	cases = FilterByTag(cases, req.Tag)
	cases = FilterByCategory(cases, req.Category)
	if len(cases) == 0 {
		return ExecuteResult{}, fmt.Errorf("no test cases to run after filtering")
	}

	datasetIDs := CollectDatasetIDs(cases)
	if len(datasetIDs) == 0 {
		return ExecuteResult{}, fmt.Errorf("no dataset_id values found in selected cases")
	}

	oracleFacts := map[string]dataset.Fact{}
	if p.DatasetPreparer != nil {
		oracleFacts, err = p.DatasetPreparer.PrepareOracleFacts(ctx, DatasetPrepareRequest{
			DatasetIDs: datasetIDs,
			SeedTenant: req.SeedTenantID,
			Seed:       req.Seed,
		})
		if err != nil {
			return ExecuteResult{}, err
		}
	}

	harnessCfg := req.HarnessConfig
	harnessCfg.OracleFacts = mergeHarnessOracleFacts(harnessCfg.OracleFacts, dataset.ToHarnessOracleFacts(oracleFacts))

	runner, err := p.RunnerFactory.NewRunner(harnessCfg)
	if err != nil {
		return ExecuteResult{}, err
	}

	runReport, exitCode, err := runner.Run(ctx, testharness.TestSuite{Tests: cases})
	if err != nil {
		return ExecuteResult{}, err
	}

	return ExecuteResult{
		Cases:       cases,
		DatasetIDs:  datasetIDs,
		OracleFacts: oracleFacts,
		RunReport:   runReport,
		ExitCode:    exitCode,
	}, nil
}

func CollectDatasetIDs(cases []TestCase) []string {
	set := make(map[string]struct{}, len(cases))
	for _, tc := range cases {
		id := strings.TrimSpace(tc.DatasetID)
		if id == "" {
			continue
		}
		set[id] = struct{}{}
	}

	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func mergeHarnessOracleFacts(base, extra map[string]testharness.OracleFact) map[string]testharness.OracleFact {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	out := make(map[string]testharness.OracleFact, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}

	return out
}
