# BI-Chat Examples

Complete, runnable examples for common BI-Chat use cases.

## Table of Contents

1. [Example 1: Simple Chat Agent](#example-1-simple-chat-agent)
2. [Example 2: BI Agent with SQL Tools](#example-2-bi-agent-with-sql-tools)
3. [Example 3: Multi-Agent Orchestration](#example-3-multi-agent-orchestration)
4. [Example 4: HITL with Interrupts](#example-4-hitl-with-interrupts)
5. [Example 5: Custom Tools and Codecs](#example-5-custom-tools-and-codecs)
6. [Example 6: Knowledge Base Integration](#example-6-knowledge-base-integration)

---

## Example 1: Simple Chat Agent

A minimal chat agent with basic tools (time, calculator).

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

func main() {
    ctx := context.Background()

    // 1. Create model (use your provider implementation)
    model := createModel() // anthropic.NewModel(...) or openai.NewModel(...)

    // 2. Register tools
    toolRegistry := []agents.Tool{
        tools.NewTimeTool(),
        createCalculatorTool(),
    }

    // 3. Create session
    session := domain.NewSession(
        domain.WithTenantID(uuid.New()),
        domain.WithUserID(123),
        domain.WithTitle("Simple Chat"),
    )

    // 4. Process user message
    response := processMessage(ctx, model, toolRegistry, "What time is it?")
    fmt.Printf("Response: %s\n", response)
}

func processMessage(ctx context.Context, model agents.Model, tools []agents.Tool, userInput string) string {
    // Build context
    builder := context.NewBuilder()
    builder.System(
        codecs.NewSystemCodec(),
        codecs.SystemPayload{
            Content: "You are a helpful assistant with access to tools. Use them when needed.",
        },
    )
    builder.Turn(
        codecs.NewTurnCodec(),
        codecs.TurnPayload{
            Content: userInput,
        },
    )

    // Compile context
    renderer := renderers.NewAnthropicRenderer()
    policy := context.DefaultPolicy()
    compiled, err := builder.Compile(renderer, policy)
    if err != nil {
        log.Fatal(err)
    }

    // ReAct loop
    maxTurns := 10
    for turn := 0; turn < maxTurns; turn++ {
        // Generate response
        resp, err := model.Generate(ctx, agents.Request{
            Messages: convertMessages(compiled.Messages),
            Tools:    tools,
        })
        if err != nil {
            log.Fatal(err)
        }

        // Check for tool calls
        if len(resp.Message.ToolCalls) == 0 {
            // No tool calls - return final answer
            return resp.Message.Content
        }

        // Execute tool calls
        for _, toolCall := range resp.Message.ToolCalls {
            tool := findTool(tools, toolCall.Name)
            if tool == nil {
                continue
            }

            result, err := tool.Call(ctx, toolCall.Arguments)
            if err != nil {
                result = fmt.Sprintf("Error: %v", err)
            }

            // Add tool result to context
            builder.ToolOutput(
                codecs.NewToolOutputCodec(),
                codecs.ToolOutputPayload{
                    ToolCallID: toolCall.ID,
                    Content:    result,
                },
            )
        }

        // Recompile context with tool results
        compiled, err = builder.Compile(renderer, policy)
        if err != nil {
            log.Fatal(err)
        }
    }

    return "Max turns exceeded"
}

func createCalculatorTool() agents.Tool {
    return agents.NewTool(
        "calculator",
        "Perform arithmetic calculations",
        map[string]any{
            "type": "object",
            "properties": map[string]any{
                "expression": map[string]any{
                    "type":        "string",
                    "description": "Mathematical expression to evaluate (e.g., '2 + 2', '10 * 5')",
                },
            },
            "required": []string{"expression"},
        },
        func(ctx context.Context, input string) (string, error) {
            type Params struct {
                Expression string `json:"expression"`
            }
            params, err := agents.ParseToolInput[Params](input)
            if err != nil {
                return "", err
            }

            // Simple evaluation (use a proper math parser in production)
            result := evaluateExpression(params.Expression)
            return agents.FormatToolOutput(map[string]any{
                "result": result,
            })
        },
    )
}

func findTool(tools []agents.Tool, name string) agents.Tool {
    for _, t := range tools {
        if t.Name() == name {
            return t
        }
    }
    return nil
}

func convertMessages(compiled []any) []agents.Message {
    // Implementation depends on your compiled format
    // This is a simplified example
    return nil
}

func evaluateExpression(expr string) float64 {
    // Simplified - use a real math parser
    return 0.0
}

func createModel() agents.Model {
    // Return your model implementation
    return nil
}
```

---

## Example 2: BI Agent with SQL Tools

A BI assistant that can query databases and generate insights.

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
    "github.com/iota-uz/iota-sdk/pkg/bichat/services"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

type BIAgent struct {
    model         agents.Model
    queryExecutor services.QueryExecutorService
    tools         []agents.Tool
}

func NewBIAgent(model agents.Model, db *sql.DB) *BIAgent {
    queryExecutor := NewQueryExecutor(db)

    toolRegistry := []agents.Tool{
        tools.NewSchemaListTool(queryExecutor),
        tools.NewSchemaDescribeTool(queryExecutor),
        tools.NewSQLExecuteTool(queryExecutor),
        tools.NewChartTool(),
        tools.NewExportExcelTool(),
    }

    return &BIAgent{
        model:         model,
        queryExecutor: queryExecutor,
        tools:         toolRegistry,
    }
}

func (a *BIAgent) ProcessQuery(ctx context.Context, question string) (*BIResult, error) {
    // Build context with BI-specific system prompt
    builder := context.NewBuilder()

    builder.System(
        codecs.NewSystemCodec(),
        codecs.SystemPayload{
            Content: `You are a Business Intelligence assistant with access to a SQL database.

Your role:
1. Understand user questions about business data
2. Explore the database schema using schema_list and schema_describe tools
3. Write SQL queries to answer questions
4. Execute queries using sql_execute tool
5. Analyze results and provide insights
6. Generate charts when appropriate using chart tool

Always:
- Use schema tools before writing queries
- Write safe, read-only queries
- Explain your findings clearly
- Suggest visualizations when helpful`,
        },
    )

    // Add database schema overview
    schemas, err := a.queryExecutor.SchemaList(ctx)
    if err == nil {
        builder.Reference(
            codecs.NewDatabaseSchemaCodec(),
            codecs.DatabaseSchemaPayload{
                SchemaName: "public",
                Tables:     schemas,
            },
        )
    }

    // Add user question
    builder.Turn(
        codecs.NewTurnCodec(),
        codecs.TurnPayload{
            Content: question,
        },
    )

    // Compile context
    renderer := renderers.NewAnthropicRenderer()
    policy := context.ContextPolicy{
        ContextWindow:     180000,
        CompletionReserve: 8000,
        OverflowStrategy:  context.OverflowTruncate,
    }

    compiled, err := builder.Compile(renderer, policy)
    if err != nil {
        return nil, err
    }

    // ReAct loop
    var finalAnswer string
    var charts []ChartData
    var queries []string

    maxTurns := 15
    for turn := 0; turn < maxTurns; turn++ {
        resp, err := a.model.Generate(ctx, agents.Request{
            Messages: convertToMessages(compiled.Messages),
            Tools:    a.tools,
        })
        if err != nil {
            return nil, err
        }

        // No tool calls - final answer
        if len(resp.Message.ToolCalls) == 0 {
            finalAnswer = resp.Message.Content
            break
        }

        // Execute tool calls
        for _, toolCall := range resp.Message.ToolCalls {
            tool := findToolByName(a.tools, toolCall.Name)
            if tool == nil {
                continue
            }

            result, err := tool.Call(ctx, toolCall.Arguments)
            if err != nil {
                result = fmt.Sprintf("Error: %v", err)
            }

            // Track executed queries
            if toolCall.Name == "sql_execute" {
                type SQLParams struct {
                    Query string `json:"query"`
                }
                params, _ := agents.ParseToolInput[SQLParams](toolCall.Arguments)
                queries = append(queries, params.Query)
            }

            // Track generated charts
            if toolCall.Name == "chart" {
                var chartData ChartData
                // Parse chart data from result
                charts = append(charts, chartData)
            }

            // Add tool result to context
            builder.ToolOutput(
                codecs.NewToolOutputCodec(),
                codecs.ToolOutputPayload{
                    ToolCallID: toolCall.ID,
                    ToolName:   toolCall.Name,
                    Content:    result,
                },
            )
        }

        // Recompile
        compiled, err = builder.Compile(renderer, policy)
        if err != nil {
            return nil, err
        }
    }

    return &BIResult{
        Answer:  finalAnswer,
        Queries: queries,
        Charts:  charts,
    }, nil
}

type BIResult struct {
    Answer  string
    Queries []string
    Charts  []ChartData
}

type ChartData struct {
    Type   string
    Title  string
    Data   map[string]any
    Config map[string]any
}

func findToolByName(tools []agents.Tool, name string) agents.Tool {
    for _, t := range tools {
        if t.Name() == name {
            return t
        }
    }
    return nil
}

func convertToMessages(compiled any) []agents.Message {
    // Convert compiled context to messages
    return nil
}

// Example usage
func main() {
    ctx := context.Background()
    db, _ := sql.Open("postgres", "postgresql://localhost/mydb")
    defer db.Close()

    model := createModel()
    agent := NewBIAgent(model, db)

    result, err := agent.ProcessQuery(ctx, "What were our top 5 products by revenue last quarter?")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Answer: %s\n", result.Answer)
    fmt.Printf("Executed %d queries\n", len(result.Queries))
    for i, query := range result.Queries {
        fmt.Printf("  %d. %s\n", i+1, query)
    }
}
```

---

## Example 3: Multi-Agent Orchestration

Parent agent delegates tasks to specialized child agents.

```go
package main

import (
    "context"
    "fmt"

    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

type AgentOrchestrator struct {
    coordinator agents.Model
    sqlAgent    *BIAgent
    chartAgent  *ChartAgent
    reportAgent *ReportAgent
}

func NewAgentOrchestrator(model agents.Model) *AgentOrchestrator {
    return &AgentOrchestrator{
        coordinator: model,
        sqlAgent:    NewBIAgent(model, db),
        chartAgent:  NewChartAgent(model),
        reportAgent: NewReportAgent(model),
    }
}

func (o *AgentOrchestrator) ProcessRequest(ctx context.Context, request string) (string, error) {
    // Coordinator agent decides which sub-agents to use
    taskTool := agents.NewTool(
        "task",
        "Delegate a task to a specialized agent",
        map[string]any{
            "type": "object",
            "properties": map[string]any{
                "agent": map[string]any{
                    "type": "string",
                    "enum": []string{"sql", "chart", "report"},
                    "description": "Which specialized agent to use",
                },
                "task": map[string]any{
                    "type": "string",
                    "description": "Task description for the agent",
                },
            },
            "required": []string{"agent", "task"},
        },
        func(ctx context.Context, input string) (string, error) {
            type TaskParams struct {
                Agent string `json:"agent"`
                Task  string `json:"task"`
            }
            params, err := agents.ParseToolInput[TaskParams](input)
            if err != nil {
                return "", err
            }

            // Route to appropriate agent
            switch params.Agent {
            case "sql":
                result, err := o.sqlAgent.ProcessQuery(ctx, params.Task)
                if err != nil {
                    return "", err
                }
                return agents.FormatToolOutput(result)

            case "chart":
                result, err := o.chartAgent.GenerateChart(ctx, params.Task)
                if err != nil {
                    return "", err
                }
                return agents.FormatToolOutput(result)

            case "report":
                result, err := o.reportAgent.GenerateReport(ctx, params.Task)
                if err != nil {
                    return "", err
                }
                return agents.FormatToolOutput(result)

            default:
                return "", fmt.Errorf("unknown agent: %s", params.Agent)
            }
        },
    )

    // Coordinator executes with task delegation tool
    builder := context.NewBuilder()
    builder.System(
        codecs.NewSystemCodec(),
        codecs.SystemPayload{
            Content: `You are a coordinator agent that delegates tasks to specialized agents:

- sql: Answers data questions by querying the database
- chart: Creates visualizations from data
- report: Generates formatted reports

Break down complex requests into subtasks and use the task tool to delegate to appropriate agents.`,
        },
    )
    builder.Turn(
        codecs.NewTurnCodec(),
        codecs.TurnPayload{Content: request},
    )

    compiled, err := builder.Compile(renderers.NewAnthropicRenderer(), context.DefaultPolicy())
    if err != nil {
        return "", err
    }

    // Execute coordinator with delegation
    resp, err := o.coordinator.Generate(ctx, agents.Request{
        Messages: convertToMessages(compiled.Messages),
        Tools:    []agents.Tool{taskTool},
    })
    if err != nil {
        return "", err
    }

    return resp.Message.Content, nil
}

// Example usage
func main() {
    ctx := context.Background()
    orchestrator := NewAgentOrchestrator(createModel())

    response, err := orchestrator.ProcessRequest(ctx,
        "Create a quarterly sales report with revenue trends chart and top 10 products table")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response)
}
```

---

## Example 4: HITL with Interrupts

Agent asks user questions and waits for answers (Human-in-the-Loop).

```go
package main

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/services"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

func main() {
    ctx := context.Background()
    chatService := createChatService()

    sessionID := uuid.New()

    // Send initial message
    resp, err := chatService.SendMessage(ctx, services.SendMessageRequest{
        SessionID: sessionID,
        UserID:    123,
        Content:   "Generate a sales forecast for next quarter",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Check for interrupt (agent has questions)
    if resp.Interrupt != nil {
        fmt.Println("Agent needs more information:")
        for _, q := range resp.Interrupt.Questions {
            fmt.Printf("  %s: %s\n", q.ID, q.Text)
            if len(q.Options) > 0 {
                for _, opt := range q.Options {
                    fmt.Printf("    - %s: %s\n", opt.ID, opt.Label)
                }
            }
        }

        // Get user answers (from form, CLI, etc.)
        answers := map[string]string{
            "forecast_method": "linear_regression",
            "confidence_level": "95",
            "include_seasonality": "yes",
        }

        // Resume with answers
        finalResp, err := chatService.ResumeWithAnswer(ctx, services.ResumeRequest{
            SessionID:    sessionID,
            CheckpointID: resp.Interrupt.CheckpointID,
            Answers:      answers,
        })
        if err != nil {
            log.Fatal(err)
        }

        fmt.Printf("\nFinal Answer: %s\n", finalResp.AssistantMessage.Content)
    } else {
        fmt.Printf("Answer: %s\n", resp.AssistantMessage.Content)
    }
}

// Implement HITL tool
func createQuestionTool() agents.Tool {
    return agents.NewTool(
        "ask_user_question",
        "Ask the user for clarification or additional information",
        map[string]any{
            "type": "object",
            "properties": map[string]any{
                "questions": map[string]any{
                    "type": "array",
                    "items": map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "id": map[string]any{
                                "type": "string",
                                "description": "Unique question identifier",
                            },
                            "text": map[string]any{
                                "type": "string",
                                "description": "Question text",
                            },
                            "type": map[string]any{
                                "type": "string",
                                "enum": []string{"text", "single_choice", "multiple_choice"},
                            },
                            "options": map[string]any{
                                "type": "array",
                                "items": map[string]any{
                                    "type": "object",
                                    "properties": map[string]any{
                                        "id":    map[string]any{"type": "string"},
                                        "label": map[string]any{"type": "string"},
                                    },
                                },
                            },
                        },
                        "required": []string{"id", "text", "type"},
                    },
                },
            },
            "required": []string{"questions"},
        },
        func(ctx context.Context, input string) (string, error) {
            type QuestionParams struct {
                Questions []services.Question `json:"questions"`
            }
            params, err := agents.ParseToolInput[QuestionParams](input)
            if err != nil {
                return "", err
            }

            // Trigger interrupt (implementation-specific)
            // Save checkpoint and return special response
            checkpoint := saveCheckpoint(ctx, params.Questions)

            return agents.FormatToolOutput(map[string]any{
                "interrupt":      true,
                "checkpoint_id":  checkpoint.ID,
                "questions":      params.Questions,
            })
        },
    )
}
```

---

## Example 5: Custom Tools and Codecs

Extend BI-Chat with custom tools and context block types.

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// Custom Tool: Send Email
func NewEmailTool(emailService EmailService) agents.Tool {
    return agents.NewTool(
        "send_email",
        "Send an email to a recipient",
        map[string]any{
            "type": "object",
            "properties": map[string]any{
                "to": map[string]any{
                    "type":        "string",
                    "description": "Recipient email address",
                },
                "subject": map[string]any{
                    "type":        "string",
                    "description": "Email subject",
                },
                "body": map[string]any{
                    "type":        "string",
                    "description": "Email body (HTML or plain text)",
                },
                "attachments": map[string]any{
                    "type": "array",
                    "items": map[string]any{
                        "type": "string",
                        "description": "File path or URL",
                    },
                },
            },
            "required": []string{"to", "subject", "body"},
        },
        func(ctx context.Context, input string) (string, error) {
            type EmailParams struct {
                To          string   `json:"to"`
                Subject     string   `json:"subject"`
                Body        string   `json:"body"`
                Attachments []string `json:"attachments,omitempty"`
            }

            params, err := agents.ParseToolInput[EmailParams](input)
            if err != nil {
                return "", err
            }

            // Send email
            err = emailService.Send(ctx, Email{
                To:          params.To,
                Subject:     params.Subject,
                Body:        params.Body,
                Attachments: params.Attachments,
            })
            if err != nil {
                return "", err
            }

            return agents.FormatToolOutput(map[string]any{
                "status": "sent",
                "to":     params.To,
            })
        },
    )
}

// Custom Codec: Business Metrics
type BusinessMetricsCodec struct {
    *context.BaseCodec
}

func NewBusinessMetricsCodec() *BusinessMetricsCodec {
    return &BusinessMetricsCodec{
        BaseCodec: context.NewBaseCodec("business_metrics", "1.0"),
    }
}

type BusinessMetricsPayload struct {
    Period  string                 `json:"period"`
    Metrics map[string]MetricValue `json:"metrics"`
}

type MetricValue struct {
    Value      float64 `json:"value"`
    Change     float64 `json:"change"`      // Percent change from previous period
    Unit       string  `json:"unit"`        // "$", "%", "count", etc.
    Trend      string  `json:"trend"`       // "up", "down", "stable"
    Target     float64 `json:"target,omitempty"`
    TargetMet  bool    `json:"target_met"`
}

func (c *BusinessMetricsCodec) Validate(payload any) error {
    metrics, ok := payload.(BusinessMetricsPayload)
    if !ok {
        return fmt.Errorf("invalid payload type: expected BusinessMetricsPayload")
    }

    if metrics.Period == "" {
        return fmt.Errorf("period is required")
    }

    if len(metrics.Metrics) == 0 {
        return fmt.Errorf("at least one metric is required")
    }

    return nil
}

func (c *BusinessMetricsCodec) Canonicalize(payload any) ([]byte, error) {
    metrics := payload.(BusinessMetricsPayload)

    // Convert to canonical JSON (sorted keys)
    canonical := map[string]any{
        "period":  metrics.Period,
        "metrics": metrics.Metrics,
    }

    return json.Marshal(canonical)
}

// Use custom codec in context
func buildContextWithMetrics(period string, metrics map[string]MetricValue) *context.ContextBuilder {
    builder := context.NewBuilder()

    builder.System(
        codecs.NewSystemCodec(),
        codecs.SystemPayload{
            Content: "You are a business analyst. Use the provided metrics to answer questions.",
        },
    )

    builder.Reference(
        NewBusinessMetricsCodec(),
        BusinessMetricsPayload{
            Period:  period,
            Metrics: metrics,
        },
    )

    return builder
}

// Example usage
func main() {
    ctx := context.Background()

    metrics := map[string]MetricValue{
        "revenue": {
            Value:     1250000.00,
            Change:    15.3,
            Unit:      "$",
            Trend:     "up",
            Target:    1200000.00,
            TargetMet: true,
        },
        "customer_count": {
            Value:     523,
            Change:    8.2,
            Unit:      "count",
            Trend:     "up",
            Target:    500,
            TargetMet: true,
        },
        "churn_rate": {
            Value:     2.1,
            Change:    -0.5,
            Unit:      "%",
            Trend:     "down", // Lower is better for churn
            Target:    3.0,
            TargetMet: true,
        },
    }

    builder := buildContextWithMetrics("Q1 2024", metrics)
    builder.Turn(
        codecs.NewTurnCodec(),
        codecs.TurnPayload{
            Content: "Summarize our Q1 performance and suggest areas for improvement.",
        },
    )

    compiled, _ := builder.Compile(renderers.NewAnthropicRenderer(), context.DefaultPolicy())

    // Use compiled context with model
    model := createModel()
    resp, _ := model.Generate(ctx, agents.Request{
        Messages: convertToMessages(compiled.Messages),
    })

    fmt.Println(resp.Message.Content)
}
```

---

## Example 6: Knowledge Base Integration

Index documents and use KB search in agent conversations.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
    "github.com/iota-uz/iota-sdk/pkg/bichat/kb"
    "github.com/iota-uz/iota-sdk/pkg/bichat/kb/sources"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

func main() {
    ctx := context.Background()

    // 1. Create and populate knowledge base
    indexPath := "/tmp/kb_index"
    os.RemoveAll(indexPath) // Clean start

    indexer, err := kb.NewBleveIndexer(indexPath)
    if err != nil {
        log.Fatal(err)
    }
    defer indexer.Close()

    // Index documents from various sources
    if err := indexDocuments(ctx, indexer); err != nil {
        log.Fatal(err)
    }

    // 2. Create agent with KB search tool
    searcher := kb.NewBleveSearcher(indexer)
    model := createModel()

    agent := NewKBAgent(model, searcher)

    // 3. Ask questions that require KB lookup
    questions := []string{
        "What is our refund policy?",
        "How do I reset my password?",
        "What payment methods do you accept?",
    }

    for _, question := range questions {
        fmt.Printf("\nQ: %s\n", question)
        answer, err := agent.Answer(ctx, question)
        if err != nil {
            log.Printf("Error: %v", err)
            continue
        }
        fmt.Printf("A: %s\n", answer)
    }
}

func indexDocuments(ctx context.Context, indexer kb.KBIndexer) error {
    // Index from filesystem
    fsSource := sources.NewFilesystemSource(sources.FilesystemConfig{
        RootPath:   "./docs",
        Extensions: []string{".md", ".txt"},
        Recursive:  true,
    })

    if err := indexer.Rebuild(ctx, fsSource); err != nil {
        return err
    }

    // Index from database
    db, _ := sql.Open("postgres", "postgresql://localhost/mydb")
    defer db.Close()

    dbSource := sources.NewDatabaseSource(sources.DatabaseConfig{
        DB:    db,
        Query: "SELECT id, title, content, tags FROM knowledge_base WHERE published = true",
        IDColumn:      "id",
        TitleColumn:   "title",
        ContentColumn: "content",
        TagsColumn:    "tags",
    })

    docs, err := dbSource.FetchDocuments(ctx)
    if err != nil {
        return err
    }

    return indexer.IndexDocuments(ctx, docs)
}

type KBAgent struct {
    model    agents.Model
    searcher kb.KBSearcher
}

func NewKBAgent(model agents.Model, searcher kb.KBSearcher) *KBAgent {
    return &KBAgent{
        model:    model,
        searcher: searcher,
    }
}

func (a *KBAgent) Answer(ctx context.Context, question string) (string, error) {
    // Search KB for relevant documents
    results, err := a.searcher.Search(ctx, question, 3)
    if err != nil {
        return "", err
    }

    // Build context with KB results
    builder := context.NewBuilder()

    builder.System(
        codecs.NewSystemCodec(),
        codecs.SystemPayload{
            Content: `You are a helpful assistant with access to a knowledge base.
Answer questions using the provided search results.
Always cite your sources using [Source: Title] notation.`,
        },
    )

    // Add KB search results
    if len(results) > 0 {
        builder.Memory(
            codecs.NewKBResultsCodec(),
            codecs.KBResultsPayload{
                Query:   question,
                Results: results,
            },
        )
    }

    builder.Turn(
        codecs.NewTurnCodec(),
        codecs.TurnPayload{
            Content: question,
        },
    )

    // Compile and generate
    compiled, err := builder.Compile(
        renderers.NewAnthropicRenderer(),
        context.DefaultPolicy(),
    )
    if err != nil {
        return "", err
    }

    resp, err := a.model.Generate(ctx, agents.Request{
        Messages: convertToMessages(compiled.Messages),
        Tools: []agents.Tool{
            tools.NewKBSearchTool(a.searcher), // Agent can search for more info
        },
    })
    if err != nil {
        return "", err
    }

    return resp.Message.Content, nil
}

// Advanced: KB indexing with custom metadata
func indexWithMetadata(ctx context.Context, indexer kb.KBIndexer) {
    docs := []kb.Document{
        {
            ID:      "policy-001",
            Title:   "Refund Policy",
            Content: "Customers can request refunds within 30 days...",
            Tags:    []string{"policy", "refund", "customer-service"},
            Metadata: map[string]string{
                "category":    "policies",
                "last_updated": "2024-01-15",
                "author":      "legal-team",
                "version":     "2.1",
            },
            Source: "https://example.com/policies/refund",
        },
        {
            ID:      "guide-002",
            Title:   "Password Reset Guide",
            Content: "To reset your password: 1. Go to login page...",
            Tags:    []string{"guide", "security", "account"},
            Metadata: map[string]string{
                "category":    "guides",
                "difficulty":  "beginner",
                "last_updated": "2024-02-01",
            },
            Source: "https://example.com/guides/password-reset",
        },
    }

    indexer.IndexDocuments(ctx, docs)
}

// Advanced: Filtered search
func searchWithFilters(ctx context.Context, searcher kb.KBSearcher, query string) {
    // Search only in specific category
    results, _ := searcher.SearchWithFilters(ctx, query, kb.SearchFilters{
        Tags:     []string{"policy"},
        Metadata: map[string]string{"category": "policies"},
        Limit:    5,
    })

    for _, result := range results {
        fmt.Printf("Score: %.2f | %s\n", result.Score, result.Document.Title)
    }
}
```

---

## Running the Examples

### Prerequisites

```bash
# Install dependencies
go get github.com/iota-uz/iota-sdk/pkg/bichat
go get github.com/google/uuid

# Set up database (for BI examples)
createdb mydb
psql mydb < schema.sql

# Create knowledge base directory (for KB examples)
mkdir -p /tmp/kb_index
```

### Environment Variables

```bash
# LLM provider API keys
export ANTHROPIC_API_KEY="your-key"
export OPENAI_API_KEY="your-key"

# Database connection
export DATABASE_URL="postgresql://localhost/mydb"
```

### Running Individual Examples

```bash
# Example 1: Simple Chat Agent
go run example1_simple_chat.go

# Example 2: BI Agent
go run example2_bi_agent.go

# Example 3: Multi-Agent
go run example3_multi_agent.go

# Example 4: HITL
go run example4_hitl.go

# Example 5: Custom Tools
go run example5_custom_tools.go

# Example 6: Knowledge Base
go run example6_knowledge_base.go
```

## Next Steps

- **Architecture**: Understand the system design in [ARCHITECTURE.md](./ARCHITECTURE.md)
- **Migration**: Migrate from Ali/Shyona in [MIGRATION.md](./MIGRATION.md)
- **Getting Started**: Quick start guide in [GETTING_STARTED.md](./GETTING_STARTED.md)
- **API Reference**: Complete API docs via `go doc github.com/iota-uz/iota-sdk/pkg/bichat`

## Additional Resources

- **Test Examples**: See `*_test.go` files for unit test examples
- **Integration Tests**: See `pkg/bichat/*_integration_test.go` for full flow examples
- **Provider Implementations**: See LLM provider packages for `agents.Model` implementations
