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
// Returns a ServiceContainer holding all built services.
//
// This method fails fast - any error in service creation returns immediately.
func (c *ModuleConfig) BuildServices() (*ServiceContainer, error) {
	const op serrors.Op = "ModuleConfig.BuildServices"

	// Validate configuration first
	if err := c.Validate(); err != nil {
		return nil, serrors.E(op, err)
	}

	if err := c.resolveProjectPromptExtension(); err != nil {
		return nil, serrors.E(op, err, "failed to resolve project prompt extension")
	}
	if err := c.resolveSkillsSelector(); err != nil {
		return nil, serrors.E(op, err, "failed to load skills catalog")
	}

	// Create file storage once for attachment/artifact services and artifact_reader tool wiring.
	fileStorage, err := c.createFileStorage()
	if err != nil {
		return nil, serrors.E(op, err, "failed to create file storage")
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
					return nil, serrors.E(op, err)
				}
			}
		}
	}

	// Register markdown-defined and custom sub-agents before building default parent
	// so delegation descriptions include all available agents.
	if err := c.setupConfiguredSubAgents(fileStorage); err != nil {
		return nil, serrors.E(op, err, "failed to setup configured sub-agents")
	}

	// Build default parent agent from config when caller did not provide one.
	if err := c.buildParentAgent(fileStorage); err != nil {
		return nil, serrors.E(op, err)
	}

	// Build AgentService first (ChatService depends on it)
	agentService := services.NewAgentService(services.AgentServiceConfig{
		Agent:                  c.ParentAgent,
		Model:                  c.Model,
		Policy:                 c.ContextPolicy,
		Renderer:               c.Renderer,
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

	titleService, err := services.NewSessionTitleService(c.Model, c.ChatRepo, c.EventBus)
	if err != nil {
		return nil, serrors.E(op, err, "failed to create title generation service")
	}

	var titleJobQueue *services.RedisTitleJobQueue
	if c.TitleQueue != nil {
		queue, err := services.NewRedisTitleJobQueue(services.RedisTitleJobQueueConfig{
			RedisURL:  c.TitleQueue.RedisURL,
			Stream:    c.TitleQueue.Stream,
			DedupeTTL: c.TitleQueue.DedupeTTL,
		})
		if err != nil {
			return nil, serrors.E(op, err, "failed to create redis title job queue")
		}
		titleJobQueue = queue
	}

	attachmentService := services.NewAttachmentService(fileStorage)
	artifactService := bichatservices.NewArtifactService(c.ChatRepo, fileStorage, attachmentService)

	chatService := services.NewChatService(
		c.ChatRepo,
		agentService,
		c.Model,
		titleService,
		titleJobQueue,
	)

	return &ServiceContainer{
		chatService:       chatService,
		agentService:      agentService,
		attachmentService: attachmentService,
		artifactService:   artifactService,
		titleService:      titleService,
		titleJobQueue:     titleJobQueue,
		titleQueueConfig:  c.TitleQueue,
		logger:            c.Logger,
	}, nil
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

