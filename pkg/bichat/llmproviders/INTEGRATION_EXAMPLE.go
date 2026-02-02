package llmproviders

// Example: How to integrate Code Interpreter with BiChat module
//
// This file shows the complete integration flow for reference.
// Copy relevant parts to your module setup code.

/*

import (
	"context"
	"os"

	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/llmproviders"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/sashabaranov/go-openai"
)

// SetupBiChatWithCodeInterpreter demonstrates complete setup with code interpreter
func SetupBiChatWithCodeInterpreter(
	chatRepo domain.ChatRepository,
	queryExecutor bichatservices.QueryExecutorService,
) (*bichat.ModuleConfig, error) {
	// Step 1: Create OpenAI client
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY required")
	}
	openaiClient := openai.NewClient(apiKey)

	// Step 2: Create file storage for code outputs
	fileStorage, err := storage.NewLocalFileStorage(
		"/var/lib/bichat/code-outputs",
		"https://cdn.example.com/code-outputs",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storage: %w", err)
	}

	// Step 3: Create Assistants client for code execution
	assistantsClient := llmproviders.NewAssistantsClient(
		openaiClient,
		fileStorage,
	)

	// Step 4: Create code interpreter tool with executor
	codeInterpreterTool := tools.NewCodeInterpreterTool(
		tools.WithAssistantsExecutor(assistantsClient),
	)

	// Step 5: Create OpenAI model for main LLM calls
	model, err := llmproviders.NewOpenAIModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	// Step 6: Create parent agent with code interpreter tool
	parentAgent, err := bichatagents.NewDefaultBIAgent(
		queryExecutor,
		bichatagents.WithCodeInterpreterTool(codeInterpreterTool),
		// Add other tools as needed
		bichatagents.WithKBSearcher(kbSearcher),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Step 7: Create context policy
	policy := bichat.DefaultContextPolicy()

	// Step 8: Create module config
	cfg := bichat.NewModuleConfig(
		composables.UseTenantID,
		composables.UseUserID,
		chatRepo,
		model,
		policy,
		parentAgent,
		bichat.WithQueryExecutor(queryExecutor),
		bichat.WithCodeInterpreter(true), // Enable in frontend
	)

	return cfg, nil
}

// Example: Testing code interpreter manually
func ExampleCodeInterpreter() {
	ctx := context.Background()

	// Setup
	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)
	storage := storage.NewNoOpFileStorage() // Use real storage in production
	assistants := llmproviders.NewAssistantsClient(client, storage)

	// Execute code
	messageID := uuid.New()
	userMessage := "Create a bar chart showing quarterly sales: Q1=100, Q2=150, Q3=120, Q4=180"

	outputs, err := assistants.ExecuteCodeInterpreter(ctx, messageID, userMessage)
	if err != nil {
		panic(err)
	}

	// Use outputs
	for _, output := range outputs {
		fmt.Printf("Generated file: %s (%s)\n", output.Name, output.MimeType)
		fmt.Printf("Download URL: %s\n", output.URL)
		fmt.Printf("Size: %d bytes\n", output.Size)
	}
}

*/
