# BiChat Improvements & Roadmap

## 1. Security & Reliability: AST-Based SQL Injection (Refactoring)
**Goal:** Replace regex/string-based tenant isolation with robust AST parsing.
- **Problem:** `postgres_executor.go` uses regex to check for `tenant_id` filters. This is fragile for complex queries (CTEs, subqueries) and relies on the LLM to "remember" isolation rules.
- **Proposal:** Use `pg_query_go` to parse SQL into an AST.
- **Implementation:**
    - Create an interceptor layer in `infrastructure`.
    - Walk the AST recursively.
    - Inject `WHERE tenant_id = $param` into every `SelectStmt`, `UpdateStmt`, etc.
    - Deparse back to SQL string.
    - Remove `tenant_id` from system prompts (LLM acts as DB owner).

## 2. Developer Experience: Generics-Based Typed Tools
**Goal:** Single source of truth for tool schemas and parsing logic.
- **Problem:** Tools manually implement `Parameters()` (JSON Schema) and `Call()` (JSON parsing). These can drift apart, causing runtime errors.
- **Proposal:** Create a `TypedTool[T]` generic struct.
- **Implementation:**
    - Use Go reflection/generics to generate OpenAI-compatible JSON Schema from struct tags (`jsonschema:"..."`).
    - Auto-unmarshal input JSON into struct `T` inside `Call()`.
    - Pass typed struct to the handler function.

## 3. Architecture: Finite State Machine (FSM) over ReAct
**Goal:** Structured, predictable control flow for complex BI workflows.
- **Problem:** Current `Executor` is a free-form ReAct loop. Complex flows (Plan -> Query -> Validate -> Viz) rely on prompt instructions which can be flaky.
- **Proposal:** Adopt a Graph/FSM approach (similar to LangGraph).
- **Implementation:**
    - Define explicit states (Nodes) and transitions (Edges).
    - Allow conditional edges (e.g., `if Error -> goto Planner`, `if Success -> goto Viz`).
    - Enforce "Review" steps before "Answer".

## 4. Intelligence: Semantic Memory / User Profile Graph
**Goal:** Long-term personalization and "memory" of user preferences.
- **Problem:** Current `KindMemory` uses RAG which retrieves chunks based on semantic similarity. It misses structured facts (e.g., "User prefers dark mode charts").
- **Proposal:** Implement an Entity Graph / Fact Store.
- **Implementation:**
    - Create a background extraction agent that runs after sessions.
    - Extract facts: `User(ID) -> PREFERS -> Chart(Type:Bar)`.
    - Store in structured SQL or Graph DB.
    - Inject relevant facts into `KindPinned` context before runs.

## 5. Performance: "Speculative" Tool Execution
**Goal:** Reduce latency for multi-step requests.
- **Problem:** Tools run sequentially. The LLM waits for Tool A to finish before generating the call for Tool B (or next tokens).
- **Proposal:** Optimistic parallel execution.
- **Implementation:**
    - Detect independent tool calls.
    - Support streaming tool calls: start executing as soon as the tool call JSON is parsed from the stream, before the sentence ends.
    - Run independent tools in parallel goroutines.

## 6. Infrastructure: Standardized Evaluation (Eval) Framework
**Goal:** Scientific measurement of agent quality and regression testing.
- **Problem:** Tests currently use mocks (`mockModel`). They test code logic, not agent intelligence/SQL generation quality.
- **Proposal:** Create a `bichat/eval` package.
- **Implementation:**
    - Maintain a dataset of "Golden Scenarios" (input + expected SQL/Result).
    - Build a test runner that uses a strong model (LLM-as-a-Judge) to grade actual agent outputs.
    - Grade on: Correctness, SQL Safety, formatting.
    - Integrate "Smoke Evals" into CI.

## 7. Observability: Standardized Trace Context Propagation
**Goal:** "Drop-in" observability without manual `Record*` calls.
- **Problem:** Observability relies on developers manually calling `provider.Record*`. Missing calls break traces.
- **Proposal:** Automatic context propagation via `context.Context`.
- **Implementation:**
    - Inject `trace_id` and `span_id` into `context.Context`.
    - Wrap `QueryExecutorPool` and `LLMClient` with decorators.
    - Decorators automatically extract IDs from context and emit spans to the registered provider.
    - Ensure standard OpenTelemetry compatibility.
