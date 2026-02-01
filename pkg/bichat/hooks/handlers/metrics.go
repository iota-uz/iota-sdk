package handlers

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/prometheus/client_golang/prometheus"
)

// MetricsHandler exports Prometheus metrics for BI-chat events.
// It tracks LLM calls, tool usage, context compilation, and errors.
type MetricsHandler struct {
	// LLM metrics
	llmRequests         *prometheus.CounterVec
	llmResponseDuration *prometheus.HistogramVec
	llmTokensUsed       *prometheus.CounterVec

	// Tool metrics
	toolExecutions *prometheus.CounterVec
	toolDuration   *prometheus.HistogramVec
	toolErrors     *prometheus.CounterVec

	// Context metrics
	contextCompilations *prometheus.CounterVec
	contextTokens       *prometheus.HistogramVec
	contextOverflows    *prometheus.CounterVec

	// Session metrics
	sessionsCreated *prometheus.CounterVec
	messagesSaved   *prometheus.CounterVec
	interrupts      *prometheus.CounterVec
}

// NewMetricsHandler creates a new MetricsHandler and registers metrics with the provided registry.
func NewMetricsHandler(registry *prometheus.Registry) *MetricsHandler {
	h := &MetricsHandler{
		llmRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_llm_requests_total",
				Help: "Total number of LLM API requests",
			},
			[]string{"provider", "model", "tenant_id"},
		),
		llmResponseDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bichat_llm_response_duration_seconds",
				Help:    "LLM response latency in seconds",
				Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
			},
			[]string{"provider", "model", "tenant_id"},
		),
		llmTokensUsed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_llm_tokens_total",
				Help: "Total tokens consumed (prompt + completion)",
			},
			[]string{"provider", "model", "token_type", "tenant_id"},
		),
		toolExecutions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_tool_executions_total",
				Help: "Total number of tool executions",
			},
			[]string{"tool_name", "status", "tenant_id"},
		),
		toolDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bichat_tool_duration_seconds",
				Help:    "Tool execution duration in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"tool_name", "tenant_id"},
		),
		toolErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_tool_errors_total",
				Help: "Total number of tool execution errors",
			},
			[]string{"tool_name", "tenant_id"},
		),
		contextCompilations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_context_compilations_total",
				Help: "Total number of context compilations",
			},
			[]string{"provider", "compacted", "truncated", "tenant_id"},
		),
		contextTokens: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bichat_context_tokens",
				Help:    "Tokens in compiled context",
				Buckets: []float64{1000, 5000, 10000, 20000, 50000, 100000, 200000},
			},
			[]string{"provider", "tenant_id"},
		),
		contextOverflows: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_context_overflows_total",
				Help: "Total number of context overflow events",
			},
			[]string{"strategy", "resolved", "tenant_id"},
		),
		sessionsCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_sessions_created_total",
				Help: "Total number of chat sessions created",
			},
			[]string{"tenant_id"},
		),
		messagesSaved: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_messages_saved_total",
				Help: "Total number of messages saved",
			},
			[]string{"role", "tenant_id"},
		),
		interrupts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bichat_interrupts_total",
				Help: "Total number of HITL interrupts",
			},
			[]string{"interrupt_type", "agent_name", "tenant_id"},
		),
	}

	// Register all metrics
	registry.MustRegister(
		h.llmRequests,
		h.llmResponseDuration,
		h.llmTokensUsed,
		h.toolExecutions,
		h.toolDuration,
		h.toolErrors,
		h.contextCompilations,
		h.contextTokens,
		h.contextOverflows,
		h.sessionsCreated,
		h.messagesSaved,
		h.interrupts,
	)

	return h
}

// Handle implements EventHandler.
func (h *MetricsHandler) Handle(ctx context.Context, event hooks.Event) error {
	tenantID := event.TenantID().String()

	switch e := event.(type) {
	case *events.LLMRequestEvent:
		h.llmRequests.WithLabelValues(e.Provider, e.Model, tenantID).Inc()

	case *events.LLMResponseEvent:
		// Convert milliseconds to seconds for Prometheus base unit compliance
		h.llmResponseDuration.WithLabelValues(e.Provider, e.Model, tenantID).Observe(float64(e.LatencyMs) / 1000.0)
		h.llmTokensUsed.WithLabelValues(e.Provider, e.Model, "prompt", tenantID).Add(float64(e.PromptTokens))
		h.llmTokensUsed.WithLabelValues(e.Provider, e.Model, "completion", tenantID).Add(float64(e.CompletionTokens))

	case *events.ToolCompleteEvent:
		h.toolExecutions.WithLabelValues(e.ToolName, "success", tenantID).Inc()
		// Convert milliseconds to seconds for Prometheus base unit compliance
		h.toolDuration.WithLabelValues(e.ToolName, tenantID).Observe(float64(e.DurationMs) / 1000.0)

	case *events.ToolErrorEvent:
		h.toolExecutions.WithLabelValues(e.ToolName, "error", tenantID).Inc()
		h.toolErrors.WithLabelValues(e.ToolName, tenantID).Inc()

	case *events.ContextCompileEvent:
		compactedStr := boolToString(e.Compacted)
		truncatedStr := boolToString(e.Truncated)
		h.contextCompilations.WithLabelValues(e.Provider, compactedStr, truncatedStr, tenantID).Inc()
		h.contextTokens.WithLabelValues(e.Provider, tenantID).Observe(float64(e.TotalTokens))

	case *events.ContextOverflowEvent:
		resolvedStr := boolToString(e.Resolved)
		h.contextOverflows.WithLabelValues(e.OverflowStrategy, resolvedStr, tenantID).Inc()

	case *events.SessionCreateEvent:
		h.sessionsCreated.WithLabelValues(tenantID).Inc()

	case *events.MessageSaveEvent:
		h.messagesSaved.WithLabelValues(e.Role, tenantID).Inc()

	case *events.InterruptEvent:
		h.interrupts.WithLabelValues(e.InterruptType, e.AgentName, tenantID).Inc()
	}

	return nil
}

// boolToString converts a boolean to "true" or "false" string.
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
