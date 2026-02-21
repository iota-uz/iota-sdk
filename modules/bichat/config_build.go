package bichat

import (
	"errors"
	"strings"

	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	bichatskills "github.com/iota-uz/iota-sdk/pkg/bichat/skills"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	bichattools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/bichat/prompts"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (c *ModuleConfig) resolveProjectPromptExtension() error {
	if c.projectPromptExtensionResolved {
		return nil
	}

	staticExtension := prompts.NormalizeProjectPromptExtension(c.ProjectPromptExtension)
	resolved := staticExtension

	if c.ProjectPromptExtensionProvider != nil {
		providerExtension, err := c.ProjectPromptExtensionProvider.ProjectPromptExtension()
		if err != nil {
			return err
		}
		providerExtension = prompts.NormalizeProjectPromptExtension(providerExtension)
		if providerExtension != "" {
			resolved = providerExtension
		}
	}

	c.resolvedProjectPromptExtension = resolved
	c.projectPromptExtensionResolved = true

	return nil
}

func (c *ModuleConfig) resolveSkillsSelector() error {
	if c.skillsSelector != nil {
		return nil
	}

	skillsDir := strings.TrimSpace(c.SkillsDir)
	if skillsDir == "" {
		return nil
	}

	catalog, err := bichatskills.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	c.skillsCatalog = catalog
	c.skillsSelector = bichatskills.NewSelector(
		catalog,
		bichatskills.WithSelectionLimit(c.SkillsSelectionLimit),
		bichatskills.WithMaxChars(c.SkillsMaxChars),
	)
	return nil
}

func (c *ModuleConfig) createFileStorage() (storage.FileStorage, error) {
	switch c.AttachmentStorageMode {
	case AttachmentStorageModeNoOp:
		return storage.NewNoOpFileStorage(), nil
	case AttachmentStorageModeLocal:
		return storage.NewLocalFileStorage(
			c.AttachmentStorageBasePath,
			c.AttachmentStorageBaseURL,
		)
	default:
		return nil, errors.New("invalid attachment storage mode")
	}
}

// BuildServices creates and initializes all BiChat services from the configuration.
// This should be called after NewModuleConfig and before registering the module.
// Services are cached in the config and reused on subsequent calls.
//
// This method fails fast - any error in service creation returns immediately.
func (c *ModuleConfig) BuildServices() error {
	const op serrors.Op = "ModuleConfig.BuildServices"

	// Validate configuration first
	if err := c.Validate(); err != nil {
		return serrors.E(op, err)
	}

	if err := c.resolveProjectPromptExtension(); err != nil {
		return serrors.E(op, err, "failed to resolve project prompt extension")
	}
	if err := c.resolveSkillsSelector(); err != nil {
		return serrors.E(op, err, "failed to load skills catalog")
	}

	// Create file storage once for attachment/artifact services and artifact_reader tool wiring.
	var fileStorage storage.FileStorage
	needsFileStorage := c.ParentAgent == nil || c.attachmentService == nil || c.artifactService == nil || c.Capabilities.CodeInterpreter || c.Capabilities.MultiAgent
	if needsFileStorage {
		fs, err := c.createFileStorage()
		if err != nil {
			return serrors.E(op, err, "failed to create file storage")
		}
		fileStorage = fs
	}

	if c.Capabilities.CodeInterpreter {
		if configurable, ok := c.Model.(interface {
			SetCodeInterpreterArtifactSource(domain.ChatRepository, storage.FileStorage)
		}); ok {
			configurable.SetCodeInterpreterArtifactSource(c.ChatRepo, fileStorage)
		}
		if c.CodeInterpreterMemoryLimit != "" {
			if configurable, ok := c.Model.(interface{ SetCodeInterpreterMemoryLimit(string) error }); ok {
				if err := configurable.SetCodeInterpreterMemoryLimit(c.CodeInterpreterMemoryLimit); err != nil {
					return serrors.E(op, err)
				}
			}
		}
	}

	// Register markdown-defined and custom sub-agents before building default parent
	// so delegation descriptions include all available agents.
	if err := c.setupConfiguredSubAgents(fileStorage); err != nil {
		return serrors.E(op, err, "failed to setup configured sub-agents")
	}

	// Build default parent agent from config when caller did not provide one.
	if err := c.buildParentAgent(fileStorage); err != nil {
		return serrors.E(op, err)
	}

	// Build AgentService first (ChatService depends on it)
	if c.agentService == nil {
		c.agentService = services.NewAgentService(services.AgentServiceConfig{
			Agent:                  c.ParentAgent,
			Model:                  c.Model,
			Policy:                 c.ContextPolicy,
			Renderer:               c.Renderer, // Use configured renderer
			Checkpointer:           c.Checkpointer,
			EventBus:               c.EventBus,
			ChatRepo:               c.ChatRepo,
			AgentRegistry:          c.AgentRegistry,
			SchemaMetadata:         c.SchemaMetadataProvider,
			ProjectPromptExtension: c.resolvedProjectPromptExtension,
			SkillsSelector:         c.skillsSelector,
			Logger:                 c.Logger,
			FormatterRegistry:      formatters.DefaultFormatterRegistry(),
		})
	}

	if c.titleGenerationService == nil {
		titleService, err := services.NewTitleGenerationService(c.Model, c.ChatRepo, c.EventBus)
		if err != nil {
			return serrors.E(op, err, "failed to create title generation service")
		}
		c.titleGenerationService = titleService
	}

	if strings.TrimSpace(c.TitleQueueRedisURL) != "" && c.titleJobQueue == nil {
		queue, err := services.NewRedisTitleJobQueue(services.RedisTitleJobQueueConfig{
			RedisURL:  c.TitleQueueRedisURL,
			Stream:    c.TitleQueueStream,
			DedupeTTL: c.TitleQueueDedupeTTL,
		})
		if err != nil {
			return serrors.E(op, err, "failed to create redis title job queue")
		}
		c.titleJobQueue = queue
	}

	// Build ChatService
	if c.chatService == nil {
		c.chatService = services.NewChatService(
			c.ChatRepo,
			c.agentService,
			c.Model,
			c.titleGenerationService,
			c.titleJobQueue,
		)
	}

	// Build AttachmentService
	if c.attachmentService == nil {
		c.attachmentService = services.NewAttachmentService(fileStorage)
	}

	// Build ArtifactService
	if c.artifactService == nil {
		c.artifactService = bichatservices.NewArtifactService(c.ChatRepo, fileStorage, c.attachmentService)
	}

	return nil
}

// BuildParentAgent creates the default BI parent agent when ParentAgent is nil.
// It applies KB, learning, validated query, and code interpreter options from ModuleConfig.
func (c *ModuleConfig) BuildParentAgent() error {
	return c.buildParentAgent(nil)
}

func (c *ModuleConfig) buildParentAgent(fileStorage storage.FileStorage) error {
	const op serrors.Op = "ModuleConfig.BuildParentAgent"

	if c.ParentAgent != nil {
		return nil
	}

	if c.QueryExecutor == nil {
		return serrors.E(op, serrors.KindValidation, "ParentAgent or QueryExecutor is required")
	}

	opts := make([]bichatagents.BIAgentOption, 0, 6)
	if c.KBSearcher != nil {
		opts = append(opts, bichatagents.WithKBSearcher(c.KBSearcher))
	}
	if c.LearningStore != nil {
		opts = append(opts, bichatagents.WithLearningStore(c.LearningStore))
	}
	if c.ValidatedQueryStore != nil {
		opts = append(opts, bichatagents.WithValidatedQueryStore(c.ValidatedQueryStore))
	}
	if c.AgentRegistry != nil {
		opts = append(opts, bichatagents.WithAgentRegistry(c.AgentRegistry))
	}
	if c.Capabilities.CodeInterpreter {
		opts = append(opts, bichatagents.WithCodeInterpreter(true))
	}
	if c.Model != nil {
		modelName := strings.TrimSpace(c.Model.Info().Name)
		if modelName != "" {
			opts = append(opts, bichatagents.WithModel(modelName))
		}
	}
	if c.ChatRepo != nil && fileStorage != nil {
		opts = append(opts, bichatagents.WithArtifactReaderTool(
			bichattools.NewArtifactReaderTool(c.ChatRepo, fileStorage),
		))
	}

	parentAgent, err := bichatagents.NewDefaultBIAgent(c.QueryExecutor, opts...)
	if err != nil {
		return serrors.E(op, err)
	}
	c.ParentAgent = parentAgent

	return nil
}

// ChatService returns the cached ChatService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) ChatService() bichatservices.ChatService {
	return c.chatService
}

// AgentService returns the cached AgentService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) AgentService() bichatservices.AgentService {
	return c.agentService
}

// AttachmentService returns the cached AttachmentService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) AttachmentService() bichatservices.AttachmentService {
	return c.attachmentService
}

// ArtifactService returns the cached ArtifactService instance.
// Returns nil if BuildServices hasn't been called yet.
func (c *ModuleConfig) ArtifactService() bichatservices.ArtifactService {
	return c.artifactService
}

// NewTitleJobWorker builds a Redis-backed title generation worker when queueing is enabled.
// Returns ErrTitleJobWorkerDisabled when queueing is not configured.
func (c *ModuleConfig) NewTitleJobWorker(pool *pgxpool.Pool) (*services.TitleJobWorker, error) {
	if c.titleJobQueue == nil || c.titleGenerationService == nil {
		return nil, ErrTitleJobWorkerDisabled
	}
	if pool == nil {
		return nil, errors.New("database pool is required for title job worker")
	}

	return services.NewTitleJobWorker(services.TitleJobWorkerConfig{
		Queue:          c.titleJobQueue,
		TitleService:   c.titleGenerationService,
		Pool:           pool,
		Logger:         c.Logger,
		Group:          c.TitleQueueGroup,
		Consumer:       c.TitleQueueConsumer,
		BatchSize:      c.TitleQueueBatchSize,
		PollInterval:   c.TitleQueuePollInterval,
		ReadBlock:      c.TitleQueueReadBlock,
		MaxRetries:     c.TitleQueueMaxRetries,
		RetryBaseDelay: c.TitleQueueRetryBaseDelay,
		RetryMaxDelay:  c.TitleQueueRetryMaxDelay,
		PendingIdle:    c.TitleQueuePendingIdle,
		ReconcileEvery: c.TitleQueueReconcileEvery,
		ReconcileBatch: c.TitleQueueReconcileBatch,
		JobTimeout:     c.TitleQueueJobTimeout,
	})
}

// CloseTitleQueue releases queue resources when Redis queueing is configured.
func (c *ModuleConfig) CloseTitleQueue() error {
	if c.titleJobQueue == nil {
		return nil
	}
	err := c.titleJobQueue.Close()
	c.titleJobQueue = nil
	return err
}
