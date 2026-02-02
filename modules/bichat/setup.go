package bichat

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ValidateBiChatDependencies validates that required dependencies are available.
// This should be called during application startup to fail fast.
//
// Example:
//
//	if err := bichat.ValidateBiChatDependencies(app.DB()); err != nil {
//	    log.Fatal("BiChat dependencies missing:", err)
//	}
func ValidateBiChatDependencies(pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("database connection is required for BiChat")
	}

	// Note: OpenAI API key validation happens when creating the model
	// Use llmproviders.NewOpenAIModel() to validate and create model

	return nil
}

// Setup documentation:
//
// 1. Import required packages:
//    import (
//        "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
//        "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
//    )
//
// 2. Create OpenAI model:
//    model, err := llmproviders.NewOpenAIModel()
//    if err != nil {
//        log.Fatal("Failed to create OpenAI model:", err)
//    }
//
// 3. Create PostgreSQL query executor:
//    executor := infrastructure.NewPostgresQueryExecutor(app.DB())
//
// 4. Configure module:
//    cfg := bichat.NewModuleConfig(
//        composables.UseTenantID,
//        composables.UseUserID,
//        chatRepo,
//        model,
//        bichat.DefaultContextPolicy(),
//        parentAgent,
//        bichat.WithQueryExecutor(executor),
//        bichat.WithAttachmentStorage(
//            "/var/lib/bichat/attachments",
//            "https://example.com/bichat/attachments",
//        ),
//    )
//
// 5. Register module (must check error):
//    module := bichat.NewModuleWithConfig(cfg)
//    if err := app.RegisterModule(module); err != nil {
//        log.Fatalf("Failed to register BiChat: %v", err)
//    }
//
// See CLAUDE.md for complete setup instructions.
