// Package langfuse provides Langfuse observability integration for BiChat.
//
// This package implements the observability.Provider interface using the Langfuse
// observability platform. It sends LLM generations, spans, events, and traces to
// Langfuse for analysis, debugging, and cost tracking.
//
// # Features
//
//   - Generation tracking: Records LLM API calls with token usage and costs
//   - Span tracking: Monitors tool executions and operations
//   - Event tracking: Captures discrete events (interrupts, state changes)
//   - Trace tracking: Links related observations into session traces
//   - Cost calculation: Automatic cost tracking using model pricing
//   - Sample rate control: Configurable observation sampling (0.0-1.0)
//   - Non-blocking: All errors are logged, none propagated
//   - Cache token support: Tracks cache_write_tokens and cache_read_tokens
//
// # Usage
//
// Create a Langfuse provider:
//
//	config := langfuse.Config{
//	    PublicKey: "pk_...",
//	    SecretKey: "sk_...",
//	    Host: "https://cloud.langfuse.com",
//	    FlushInterval: 1 * time.Second,
//	    SampleRate: 1.0, // 100% of observations
//	    Environment: "production",
//	    Version: "1.0.0",
//	    Tags: []string{"bichat", "production"},
//	    Enabled: true,
//	}
//
//	provider, err := langfuse.NewLangfuseProvider(ctx, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Shutdown(ctx)
//
// Record observations:
//
//	// Record an LLM generation
//	provider.RecordGeneration(ctx, observability.GenerationObservation{
//	    ID: "gen-123",
//	    SessionID: sessionID,
//	    TenantID: tenantID,
//	    Model: "claude-3-5-sonnet-20241022",
//	    Provider: "anthropic",
//	    PromptTokens: 1500,
//	    CompletionTokens: 500,
//	    TotalTokens: 2000,
//	    Duration: 2 * time.Second,
//	    FinishReason: "stop",
//	})
//
//	// Record a span (tool execution)
//	provider.RecordSpan(ctx, observability.SpanObservation{
//	    ID: "span-456",
//	    SessionID: sessionID,
//	    TenantID: tenantID,
//	    Name: "sql_execute",
//	    Type: "tool",
//	    Duration: 100 * time.Millisecond,
//	    Input: `{"query": "SELECT * FROM users"}`,
//	    Output: `{"rows": [...]}`,
//	})
//
// # Environment Variables
//
// The Langfuse SDK reads credentials from environment variables:
//
//   - LANGFUSE_HOST: Langfuse API endpoint (set from config.Host)
//   - LANGFUSE_PUBLIC_KEY: Langfuse public API key (set from config.PublicKey)
//   - LANGFUSE_SECRET_KEY: Langfuse secret API key (set from config.SecretKey)
//
// These are automatically set by NewLangfuseProvider.
//
// # Cost Calculation
//
// Costs are calculated using pricing metadata in observation attributes:
//
//	obs.Attributes = map[string]interface{}{
//	    "input_price_per_1m": 3.0,  // $3 per 1M input tokens
//	    "output_price_per_1m": 15.0, // $15 per 1M output tokens
//	}
//
// Alternatively, pre-calculated costs can be provided:
//
//	obs.Attributes = map[string]interface{}{
//	    "cost": 0.0042, // $0.0042 for this generation
//	}
//
// # Sampling
//
// Sample rate controls what percentage of observations are sent to Langfuse:
//
//   - 1.0 (default): Send all observations (100%)
//   - 0.5: Send 50% of observations (random sampling)
//   - 0.0: Send no observations (disabled)
//
// Sampling is deterministic per-instance (uses seeded RNG).
//
// # Non-blocking Design
//
// All Record* methods are non-blocking. Errors are logged via logrus but not returned.
// This ensures observability failures never break the application.
//
// # Shutdown
//
// Always call Shutdown before application exit to flush pending observations:
//
//	defer provider.Shutdown(ctx)
//
// # Dependencies
//
//   - github.com/henomis/langfuse-go: Langfuse Go SDK
//   - github.com/sirupsen/logrus: Structured logging
package langfuse
