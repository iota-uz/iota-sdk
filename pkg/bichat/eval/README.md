# BiChat Eval

BiChat eval is analytics-only and requires:

1. Live BiChat HTTP/SSE endpoints
2. Deterministic dataset seeding (or an already seeded database)
3. OpenAI LLM judge

The library is composable. External SDK users can plug:

1. `eval.CaseSource` for custom suite storage/loaders
2. `eval.DatasetPreparer` for custom seeding/oracle generation
3. `eval.RunnerFactory` for alternate execution engines

## Run

```bash
command bichat eval run \
  --cases ./pkg/bichat/eval/testdata/analytics/suite.json \
  --server-url http://127.0.0.1:3200 \
  --rpc-path /bi-chat/rpc \
  --stream-path /bi-chat/stream \
  --session-token '<granite_sid>' \
  --hitl-model 'gpt-4o-mini' \
  --openai-api-key "$OPENAI_API_KEY" \
  --seed-dsn 'postgres://postgres:postgres@localhost:5432/iota?sslmode=disable' \
  --seed-tenant-id '00000000-0000-0000-0000-000000000001'
```

## Suite schema

`pkg/bichat/eval/testdata/analytics/suite.json` uses `tests[]` with:

- `id`, `description`, `dataset_id`
- `category`, `tags`
- `turns[]` with `prompt`, `oracle_refs`, `judge_instructions`
- `expect` flags

Legacy primitive fields (`question`, `expected_sql`, `expected_content`, `golden_answer`) are not supported.

## Metrics

Per eval (test case), the report includes:

- `tool_use_efficiency` (number of tool calls used to produce answers)
- `cost`
- `input_tokens` and `output_tokens` (plus totals and assistant/judge/HITL breakdown)

## Composable API

Use `eval.Pipeline` to orchestrate cases + dataset prep + execution:

```go
pipeline := eval.Pipeline{
  CaseSource:      eval.PathCaseSource{Path: "./custom_suite.json"},
  DatasetPreparer: eval.SeededDatasetPreparer{Seeder: mySeeder},
  RunnerFactory:   eval.TestHarnessRunnerFactory{},
}

result, err := pipeline.Execute(ctx, eval.ExecuteRequest{
  Tag:           "capability",
  Category:      "analytics",
  SeedTenantID:  tenantID,
  Seed:          true,
  HarnessConfig: harnessCfg,
})
```

For dataset extension, register your own dataset implementations:

```go
registry, err := dataset.NewRegistry(myDatasetV1, myDatasetV2)
if err != nil {
  return err
}
```
