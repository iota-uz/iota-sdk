# BiChat Improvements & Roadmap

## 1. Developer Experience: Generics-Based Typed Tools
**Goal:** Single source of truth for tool schemas and parsing logic.
- **Problem:** Tools manually implement `Parameters()` (JSON Schema) and `Call()` (JSON parsing). These can drift apart, causing runtime errors.
- **Proposal:** Create a `TypedTool[T]` generic struct.
- **Implementation:**
    - Use Go reflection/generics to generate OpenAI-compatible JSON Schema from struct tags (`jsonschema:"..."`).
    - Auto-unmarshal input JSON into struct `T` inside `Call()`.
    - Pass typed struct to the handler function.

## 2. Performance: "Speculative" Tool Execution
**Goal:** Reduce latency for multi-step requests.
- **Problem:** Tools run sequentially. The LLM waits for Tool A to finish before generating the call for Tool B (or next tokens).
- **Proposal:** Optimistic parallel execution.
- **Implementation:**
    - Detect independent tool calls.
    - Support streaming tool calls: start executing as soon as the tool call JSON is parsed from the stream, before the sentence ends.
    - Run independent tools in parallel goroutines.

## 3. Infrastructure: Standardized Evaluation (Eval) Framework
**Goal:** Scientific measurement of agent quality and regression testing.
- **Problem:** Tests currently use mocks (`mockModel`). They test code logic, not agent intelligence/SQL generation quality.
- **Proposal:** Create a `bichat/eval` package.
- **Reference Implementation:** See `../shy-trucks/core/cmd/shyona-test` and `../shy-trucks/core/modules/shyona`.
    - Key components to replicate:
        - `judge.go`: LLM-as-a-Judge implementation for grading outputs.
        - `test_runner.go`: Harness to execute test suites against the agent.
        - `test_suite.go`: Definition of test scenarios/golden datasets.
- **Implementation:**
    - Maintain a dataset of "Golden Scenarios" (input + expected SQL/Result).
    - Build a test runner that uses a strong model (LLM-as-a-Judge) to grade actual agent outputs.
    - Grade on: Correctness, SQL Safety, formatting.
    - Integrate "Smoke Evals" into CI.

## 4. Observability: Standardized Trace Context Propagation
**Goal:** "Drop-in" observability without manual `Record*` calls.
- **Problem:** Observability relies on developers manually calling `provider.Record*`. Missing calls break traces.
- **Proposal:** Automatic context propagation via `context.Context`.
- **Implementation:**
    - Inject `trace_id` and `span_id` into `context.Context`.
    - Wrap `QueryExecutorPool` and `LLMClient` with decorators.
    - Decorators automatically extract IDs from context and emit spans to the registered provider.
    - Ensure standard OpenTelemetry compatibility.