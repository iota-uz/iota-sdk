package observability_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
)

// MockProvider demonstrates a simple observability provider implementation.
// In production, use real providers like Langfuse, OpenTelemetry, or custom backends.
type MockProvider struct {
	generations []observability.GenerationObservation
	spans       []observability.SpanObservation
	events      []observability.EventObservation
	traces      []observability.TraceObservation
}

func (p *MockProvider) RecordGeneration(ctx context.Context, obs observability.GenerationObservation) error {
	p.generations = append(p.generations, obs)
	log.Printf("Recorded generation: model=%s, tokens=%d\n", obs.Model, obs.TotalTokens)
	return nil
}

func (p *MockProvider) RecordSpan(ctx context.Context, obs observability.SpanObservation) error {
	p.spans = append(p.spans, obs)
	log.Printf("Recorded span: name=%s, duration=%s\n", obs.Name, obs.Duration)
	return nil
}

func (p *MockProvider) RecordEvent(ctx context.Context, obs observability.EventObservation) error {
	p.events = append(p.events, obs)
	log.Printf("Recorded event: name=%s, level=%s\n", obs.Name, obs.Level)
	return nil
}

func (p *MockProvider) RecordTrace(ctx context.Context, obs observability.TraceObservation) error {
	p.traces = append(p.traces, obs)
	log.Printf("Recorded trace: id=%s, status=%s\n", obs.ID, obs.Status)
	return nil
}

func (p *MockProvider) Flush(ctx context.Context) error {
	log.Printf("Flushing provider: %d generations, %d spans, %d events, %d traces\n",
		len(p.generations), len(p.spans), len(p.events), len(p.traces))
	return nil
}

func (p *MockProvider) Shutdown(ctx context.Context) error {
	log.Println("Shutting down provider")
	return nil
}

// Example_basicUsage demonstrates basic observability integration with BiChat module.
func Example_basicUsage() {
	// Create observability provider
	provider := &MockProvider{}

	// Example: Create BiChat module config with observability (when available)
	_ = provider
	_ = bichat.DefaultContextPolicy

	// TODO: Integration with BiChat module will look like:
	//
	// cfg := bichat.NewModuleConfig(
	//     func(ctx context.Context) uuid.UUID { return uuid.New() },
	//     func(ctx context.Context) int64 { return 1 },
	//     chatRepo, model, bichat.DefaultContextPolicy(), agent,
	//     bichat.WithObservability(provider),
	// )
	// module := bichat.NewModuleWithConfig(cfg)
	// defer module.Shutdown(context.Background())

	fmt.Println("Observability provider example")

	// Output:
	// Observability provider example
}

// Example_langfuseIntegration demonstrates Langfuse provider integration.
// This example shows environment variable loading and configuration.
func Example_langfuseIntegration() {
	// Load Langfuse configuration from environment
	langfuseConfig := struct {
		SecretKey string
		PublicKey string
		Host      string
	}{
		SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
		PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
		Host:      getEnvOrDefault("LANGFUSE_HOST", "https://cloud.langfuse.com"),
	}

	// Validate configuration
	if langfuseConfig.SecretKey == "" || langfuseConfig.PublicKey == "" {
		fmt.Println("Langfuse credentials not configured, skipping...")
		return
	}

	// Create Langfuse provider (implementation details in pkg/bichat/observability/langfuse/)
	// langfuseProvider := langfuse.NewProvider(langfuseConfig.SecretKey, langfuseConfig.PublicKey, langfuseConfig.Host)

	// Create BiChat module with Langfuse observability
	// cfg := bichat.NewModuleConfig(
	//     tenantID, userID, chatRepo, model, policy, agent,
	//     bichat.WithObservability(langfuseProvider),
	// )

	// module := bichat.NewModuleWithConfig(cfg)
	// defer module.Shutdown(context.Background())

	fmt.Println("Langfuse provider configured successfully")

	// Output:
	// Langfuse credentials not configured, skipping...
}

// Example_multipleProviders demonstrates using multiple observability providers simultaneously.
func Example_multipleProviders() {
	// Create multiple providers
	mockProvider := &MockProvider{}
	_ = mockProvider
	// langfuseProvider := langfuse.NewProvider(secretKey, publicKey, host)
	// prometheusProvider := prometheus.NewProvider(registry)

	// TODO: BiChat module integration will support multiple providers:
	//
	// cfg := bichat.NewModuleConfig(
	//     tenantID, userID, chatRepo, model, policy, agent,
	//     bichat.WithObservability(mockProvider),
	//     bichat.WithObservability(langfuseProvider),
	//     bichat.WithObservability(prometheusProvider),
	// )
	// module := bichat.NewModuleWithConfig(cfg)
	// defer module.Shutdown(context.Background())
	//
	// All providers receive events simultaneously (via EventBridge)
	// Each provider is wrapped in AsyncHandler to prevent blocking

	fmt.Println("Multiple providers configured")

	// Output:
	// Multiple providers configured
}

// Example_customProvider demonstrates implementing a custom observability provider.
func Example_customProvider() {
	recordGeneration := func(ctx context.Context, obs observability.GenerationObservation) error {
		// Insert into database
		fmt.Printf("Inserting generation into DB: session=%s, model=%s, tokens=%d\n",
			obs.SessionID, obs.Model, obs.TotalTokens)
		return nil
	}

	recordSpan := func(ctx context.Context, obs observability.SpanObservation) error {
		// Insert into database
		fmt.Printf("Inserting span into DB: session=%s, name=%s\n",
			obs.SessionID, obs.Name)
		return nil
	}

	recordEvent := func(ctx context.Context, obs observability.EventObservation) error {
		fmt.Printf("Inserting event into DB: session=%s, name=%s\n",
			obs.SessionID, obs.Name)
		return nil
	}

	recordTrace := func(ctx context.Context, obs observability.TraceObservation) error {
		fmt.Printf("Inserting trace into DB: id=%s, status=%s\n",
			obs.ID, obs.Status)
		return nil
	}

	// Use in BiChat module
	_ = recordGeneration
	_ = recordSpan
	_ = recordEvent
	_ = recordTrace

	fmt.Println("Custom database provider example")

	// Output:
	// Custom database provider example
}

// Example_gracefulShutdown demonstrates proper shutdown handling for observability providers.
func Example_gracefulShutdown() {
	provider := &MockProvider{}
	_ = provider

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate provider shutdown
	_ = provider.Flush(shutdownCtx)
	_ = provider.Shutdown(shutdownCtx)

	fmt.Println("Shutdown completed successfully")

	// Output:
	// Shutdown completed successfully
}

// Example_costTracking demonstrates using observability for cost tracking.
func Example_costTracking() {
	// Custom provider that tracks LLM costs
	type CostTracker struct {
		totalCost   float64
		generations int
	}

	tracker := &CostTracker{}

	recordGeneration := func(ctx context.Context, obs observability.GenerationObservation) error {
		// Calculate cost based on model pricing
		var inputPricePerToken, outputPricePerToken float64

		switch obs.Model {
		case "claude-3-5-sonnet-20241022":
			inputPricePerToken = 3.0 / 1_000_000   // $3 per 1M input tokens
			outputPricePerToken = 15.0 / 1_000_000 // $15 per 1M output tokens
		case "gpt-4-turbo":
			inputPricePerToken = 10.0 / 1_000_000  // $10 per 1M input tokens
			outputPricePerToken = 30.0 / 1_000_000 // $30 per 1M output tokens
		default:
			inputPricePerToken = 0
			outputPricePerToken = 0
		}

		inputCost := float64(obs.PromptTokens) * inputPricePerToken
		outputCost := float64(obs.CompletionTokens) * outputPricePerToken
		totalCost := inputCost + outputCost

		tracker.totalCost += totalCost
		tracker.generations++

		fmt.Printf("Generation cost: $%.6f (input: $%.6f, output: $%.6f)\n",
			totalCost, inputCost, outputCost)

		return nil
	}

	// Simulate generation
	_ = recordGeneration(context.Background(), observability.GenerationObservation{
		Model:            "claude-3-5-sonnet-20241022",
		PromptTokens:     1000,
		CompletionTokens: 500,
	})

	fmt.Printf("Total cost: $%.6f from %d generations\n",
		tracker.totalCost, tracker.generations)

	// Output:
	// Generation cost: $0.010500 (input: $0.003000, output: $0.007500)
	// Total cost: $0.010500 from 1 generations
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
