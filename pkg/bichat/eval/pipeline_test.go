package eval_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval/dataset"
	"github.com/iota-uz/iota-sdk/pkg/bichat/testharness"
	"github.com/stretchr/testify/require"
)

type fakeCaseSource struct {
	cases []eval.TestCase
	err   error
}

func (s fakeCaseSource) LoadCases(ctx context.Context) ([]eval.TestCase, error) {
	_ = ctx
	if s.err != nil {
		return nil, s.err
	}
	return s.cases, nil
}

type fakeDatasetPreparer struct {
	lastReq eval.DatasetPrepareRequest
	facts   map[string]dataset.Fact
	err     error
}

func (p *fakeDatasetPreparer) PrepareOracleFacts(ctx context.Context, req eval.DatasetPrepareRequest) (map[string]dataset.Fact, error) {
	_ = ctx
	p.lastReq = req
	if p.err != nil {
		return nil, p.err
	}
	return p.facts, nil
}

type fakeRunner struct {
	lastSuite testharness.TestSuite
	report    testharness.RunReport
	exitCode  int
	err       error
}

func (r *fakeRunner) Run(ctx context.Context, suite testharness.TestSuite) (testharness.RunReport, int, error) {
	_ = ctx
	r.lastSuite = suite
	if r.err != nil {
		return testharness.RunReport{}, 0, r.err
	}
	return r.report, r.exitCode, nil
}

type fakeRunnerFactory struct {
	lastCfg testharness.Config
	runner  *fakeRunner
	err     error
}

func (f *fakeRunnerFactory) NewRunner(cfg testharness.Config) (eval.SuiteRunner, error) {
	f.lastCfg = cfg
	if f.err != nil {
		return nil, f.err
	}
	return f.runner, nil
}

func TestPipeline_Execute_ComposableDependencies(t *testing.T) {
	t.Parallel()

	caseSource := fakeCaseSource{
		cases: []eval.TestCase{
			{
				ID:        "keep_case",
				DatasetID: "ext_ds",
				Category:  "analytics",
				Tags:      []string{"capability"},
				Turns:     []eval.Turn{{Prompt: "p1"}},
			},
			{
				ID:        "filtered_out_case",
				DatasetID: "ext_ds_2",
				Category:  "ops",
				Tags:      []string{"other"},
				Turns:     []eval.Turn{{Prompt: "p2"}},
			},
		},
	}

	preparer := &fakeDatasetPreparer{
		facts: map[string]dataset.Fact{
			"ext_ds.fact_1": {
				Key:           "ext_ds.fact_1",
				ExpectedValue: "42",
				ValueType:     "number",
			},
		},
	}

	runner := &fakeRunner{
		report: testharness.RunReport{
			Summary: testharness.Summary{Total: 1, Passed: 1},
		},
		exitCode: 0,
	}
	factory := &fakeRunnerFactory{runner: runner}

	pipeline := eval.Pipeline{
		CaseSource:      caseSource,
		DatasetPreparer: preparer,
		RunnerFactory:   factory,
	}

	result, err := pipeline.Execute(context.Background(), eval.ExecuteRequest{
		Tag:          "capability",
		Category:     "analytics",
		SeedTenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Seed:         true,
		HarnessConfig: testharness.Config{
			OracleFacts: map[string]testharness.OracleFact{
				"preconfigured.fact": {
					Key:           "preconfigured.fact",
					ExpectedValue: "x",
				},
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, result.Cases, 1)
	require.Equal(t, "keep_case", result.Cases[0].ID)
	require.Equal(t, []string{"ext_ds"}, result.DatasetIDs)
	require.Equal(t, 1, result.RunReport.Summary.Total)
	require.Equal(t, 0, result.ExitCode)

	require.Equal(t, []string{"ext_ds"}, preparer.lastReq.DatasetIDs)
	require.True(t, preparer.lastReq.Seed)
	require.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000001"), preparer.lastReq.SeedTenant)

	require.Len(t, runner.lastSuite.Tests, 1)
	require.Equal(t, "keep_case", runner.lastSuite.Tests[0].ID)

	require.Contains(t, factory.lastCfg.OracleFacts, "preconfigured.fact")
	require.Contains(t, factory.lastCfg.OracleFacts, "ext_ds.fact_1")
	require.Equal(t, "42", factory.lastCfg.OracleFacts["ext_ds.fact_1"].ExpectedValue)
}

func TestPipeline_Execute_WithoutDatasetPreparer(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{report: testharness.RunReport{Summary: testharness.Summary{Total: 1}}}
	factory := &fakeRunnerFactory{runner: runner}

	pipeline := eval.Pipeline{
		CaseSource: fakeCaseSource{
			cases: []eval.TestCase{
				{
					ID:        "case_1",
					DatasetID: "external_dataset",
					Turns:     []eval.Turn{{Prompt: "hello"}},
				},
			},
		},
		RunnerFactory: factory,
	}

	_, err := pipeline.Execute(context.Background(), eval.ExecuteRequest{
		HarnessConfig: testharness.Config{},
	})
	require.NoError(t, err)
}

func TestCollectDatasetIDs_DeduplicatesAndSorts(t *testing.T) {
	t.Parallel()

	ids := eval.CollectDatasetIDs([]eval.TestCase{
		{DatasetID: "b"},
		{DatasetID: "a"},
		{DatasetID: "b"},
		{DatasetID: " "},
	})

	require.Equal(t, []string{"a", "b"}, ids)
}
