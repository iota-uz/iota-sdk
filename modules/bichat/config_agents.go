package bichat

import (
	"fmt"
	"strings"

	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// setupMultiAgentSystem initializes shared multi-agent registry state.
// Concrete sub-agent definitions are loaded during BuildServices.
func (c *ModuleConfig) setupMultiAgentSystem() error {
	if c.AgentRegistry == nil {
		c.AgentRegistry = agents.NewAgentRegistry()
	}
	return nil
}

func (c *ModuleConfig) setupConfiguredSubAgents(fileStorage storage.FileStorage) error {
	const op serrors.Op = "ModuleConfig.setupConfiguredSubAgents"

	if !c.Capabilities.MultiAgent {
		return nil
	}
	if c.subAgentsInitialized {
		return nil
	}
	if c.AgentRegistry == nil {
		c.AgentRegistry = agents.NewAgentRegistry()
	}

	definitions, err := bichatagents.LoadSubAgentDefinitions(
		c.SubAgentDefinitionsFS,
		c.SubAgentDefinitionsBasePath,
	)
	if err != nil {
		return serrors.E(op, err, "failed to load sub-agent definitions")
	}

	buildOpts := make([]bichatagents.SubAgentBuildOption, 0, 1)
	if c.Model != nil {
		if modelName := strings.TrimSpace(c.Model.Info().Name); modelName != "" {
			buildOpts = append(buildOpts, bichatagents.WithSubAgentModel(modelName))
		}
	}

	deps := bichatagents.SubAgentDependencies{
		QueryExecutor:  c.QueryExecutor,
		ChatRepository: c.ChatRepo,
		FileStorage:    fileStorage,
	}

	for _, def := range definitions {
		subAgent, err := bichatagents.BuildSubAgent(def, deps, buildOpts...)
		if err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to build sub-agent %q", def.Name))
		}
		if err := c.AgentRegistry.Register(subAgent); err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to register sub-agent %q", def.Name))
		}
	}

	for _, subAgent := range c.SubAgents {
		if subAgent == nil {
			return serrors.E(op, serrors.KindValidation, "custom sub-agent cannot be nil")
		}
		if err := c.AgentRegistry.Register(subAgent); err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to register custom sub-agent %q", subAgent.Name()))
		}
	}

	c.subAgentsInitialized = true
	c.Logger.WithField("count", len(c.AgentRegistry.All())).Info("Multi-agent system initialized from markdown definitions")
	return nil
}
