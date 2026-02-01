package bichat

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/generated"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/resolvers"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	bichatagents "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/bichat-schema.sql
var MigrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct{}

func (m *Module) Register(app application.Application) error {
	// Register migrations
	app.Migrations().RegisterSchema(&MigrationFiles)

	// Register locale files
	app.RegisterLocaleFiles(&LocaleFiles)

	// Register repository
	chatRepo := persistence.NewPostgresChatRepository()

	// ========================================
	// Configure LLM Model
	// ========================================
	// TODO: Replace StubModel with real LLM provider (OpenAI, Anthropic, etc.)
	// See modules/bichat/infrastructure/llmproviders/stub_model.go for examples
	//
	// Example with OpenAI:
	//   client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	//   model := NewOpenAIModel(client, "gpt-4")
	model := llmproviders.NewStubModel()

	// ========================================
	// Configure Query Executor
	// ========================================
	// TODO: Replace StubQueryExecutor with real database executor
	// See modules/bichat/infrastructure/stub_executor.go for examples
	executor := infrastructure.NewStubQueryExecutor()

	// ========================================
	// Configure BI Agent
	// ========================================
	// Create default BI agent with SQL tools
	// For production, add WithKBSearcher option if you have a knowledge base
	biAgent, err := agents.NewDefaultBIAgent(
		executor,
		agents.WithModel("stub-model"), // Matches model name
	)
	if err != nil {
		return err
	}

	// ========================================
	// Configure Context Policy
	// ========================================
	// Use default policy optimized for Claude 3.5 Sonnet (180k context window)
	policy := DefaultContextPolicy()

	// ========================================
	// Configure Renderer
	// ========================================
	// Use Anthropic renderer (also works with OpenAI models via compatibility)
	renderer := renderers.NewAnthropicRenderer()

	// ========================================
	// Configure Observability
	// ========================================
	// Create event bus for observability and metrics
	eventBus := hooks.NewEventBus()

	// Create in-memory checkpointer for HITL (Human-in-the-Loop)
	// For production, consider using PostgreSQL checkpointer for persistence
	checkpointer := bichatagents.NewInMemoryCheckpointer()

	// ========================================
	// Create AgentService
	// ========================================
	agentService := services.NewAgentService(services.AgentServiceConfig{
		Agent:        biAgent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		EventBus:     eventBus,
		ChatRepo:     chatRepo,
	})

	// ========================================
	// Create ChatService
	// ========================================
	chatService := services.NewChatService(
		chatRepo,
		agentService,
		model,
	)

	// ========================================
	// Register Controllers
	// ========================================
	app.RegisterControllers(
		controllers.NewChatController(app, chatService, chatRepo),
		controllers.NewStreamController(app, chatService),
	)

	// ========================================
	// Register GraphQL Schema
	// ========================================
	resolver := resolvers.NewResolver(app, chatService, agentService)
	schema := generated.NewExecutableSchema(generated.Config{
		Resolvers: resolver,
	})

	app.RegisterGraphSchema(application.GraphSchema{
		Value:    schema,
		BasePath: "/bichat",
	})

	// ========================================
	// Register Quick Links
	// ========================================
	app.QuickLinks().Add(
		spotlight.NewQuickLink(BiChatLink.Icon, BiChatLink.Name, BiChatLink.Href),
	)

	return nil
}

func (m *Module) Name() string {
	return "bichat"
}
