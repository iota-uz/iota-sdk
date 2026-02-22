package agents

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"time"

	coreagents "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/definition"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/artifacts"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/hitl"
	toolsql "github.com/iota-uz/iota-sdk/pkg/bichat/tools/sql"
)

const DefaultSubAgentDefinitionsBasePath = "definitions"

//go:embed definitions/*.md
var defaultSubAgentDefinitionsFS embed.FS

func DefaultSubAgentDefinitionsFS() fs.FS {
	return defaultSubAgentDefinitionsFS
}

type SubAgentDefinition struct {
	Name         string
	Description  string
	Model        string
	Tools        []string
	SystemPrompt string
	SourcePath   string
}

type subAgentFrontMatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Tools       []string `yaml:"tools"`
}

func ParseSubAgentDefinition(content, sourcePath string) (SubAgentDefinition, error) {
	doc, err := definition.ParseDocument[subAgentFrontMatter](
		[]byte(content),
		sourcePath,
		definition.ParseDocumentOptions{
			KnownFields: true,
			RequireBody: true,
		},
	)
	if err != nil {
		return SubAgentDefinition{}, err
	}

	def := SubAgentDefinition{
		Name:         doc.FrontMatter.Name,
		Description:  doc.FrontMatter.Description,
		Model:        doc.FrontMatter.Model,
		Tools:        doc.FrontMatter.Tools,
		SystemPrompt: doc.Body,
		SourcePath:   sourcePath,
	}

	if err := def.Validate(); err != nil {
		return SubAgentDefinition{}, fmt.Errorf("validate %q: %w", sourcePath, err)
	}

	return def, nil
}

func (d SubAgentDefinition) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(d.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(d.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(d.SystemPrompt) == "" {
		return fmt.Errorf("system prompt body is required")
	}
	if len(d.Tools) == 0 {
		return fmt.Errorf("tools is required")
	}

	seenTools := make(map[string]struct{}, len(d.Tools))
	for _, toolName := range d.Tools {
		trimmed := strings.TrimSpace(toolName)
		if trimmed == "" {
			return fmt.Errorf("tools cannot contain empty values")
		}
		if _, exists := seenTools[trimmed]; exists {
			return fmt.Errorf("tools contains duplicate value %q", trimmed)
		}
		seenTools[trimmed] = struct{}{}
	}

	return nil
}

func LoadSubAgentDefinitions(sourceFS fs.FS, basePath string) ([]SubAgentDefinition, error) {
	if sourceFS == nil {
		return nil, fmt.Errorf("definition source fs is required")
	}

	dir := strings.TrimSpace(basePath)
	if dir == "" {
		dir = "."
	}

	files, err := definition.LoadFiles(sourceFS, definition.LoadFilesOptions{
		Root:      dir,
		Recursive: false,
		Match: func(path string, entry fs.DirEntry) bool {
			return strings.HasSuffix(strings.ToLower(path), ".md")
		},
	})
	if err != nil {
		return nil, fmt.Errorf("read definitions directory %q: %w", dir, err)
	}

	definitions := make([]SubAgentDefinition, 0, len(files))
	seenNames := map[string]string{}

	for _, file := range files {
		def, err := ParseSubAgentDefinition(string(file.Content), file.Path)
		if err != nil {
			return nil, err
		}

		if existing, exists := seenNames[def.Name]; exists {
			return nil, fmt.Errorf("duplicate sub-agent name %q in %q and %q", def.Name, existing, file.Path)
		}
		seenNames[def.Name] = file.Path
		definitions = append(definitions, def)
	}

	if len(definitions) == 0 {
		return nil, fmt.Errorf("no sub-agent definition markdown files found in %q", dir)
	}

	return definitions, nil
}

type SubAgentDependencies struct {
	QueryExecutor      bichatsql.QueryExecutor
	ChatRepository     domain.ChatRepository
	FileStorage        storage.FileStorage
	ViewAccess         permissions.ViewAccessControl
	ArtifactReaderTool coreagents.Tool
}

type SubAgentBuildOption func(*subAgentBuildConfig)

type subAgentBuildConfig struct {
	modelOverride string
}

func WithSubAgentModel(model string) SubAgentBuildOption {
	return func(cfg *subAgentBuildConfig) {
		cfg.modelOverride = strings.TrimSpace(model)
	}
}

func BuildSubAgent(def SubAgentDefinition, deps SubAgentDependencies, opts ...SubAgentBuildOption) (coreagents.ExtendedAgent, error) {
	if err := def.Validate(); err != nil {
		return nil, err
	}

	cfg := subAgentBuildConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	agentTools, err := resolveSubAgentTools(def.Tools, deps)
	if err != nil {
		return nil, fmt.Errorf("build agent %q tools: %w", def.Name, err)
	}

	model := strings.TrimSpace(def.Model)
	if cfg.modelOverride != "" {
		model = cfg.modelOverride
	}

	agent := coreagents.NewBaseAgent(
		coreagents.WithName(strings.TrimSpace(def.Name)),
		coreagents.WithDescription(strings.TrimSpace(def.Description)),
		coreagents.WithWhenToUse(strings.TrimSpace(def.Description)),
		coreagents.WithModel(model),
		coreagents.WithTools(agentTools...),
		coreagents.WithSystemPrompt(strings.TrimSpace(def.SystemPrompt)),
	)

	return agent, nil
}

func resolveSubAgentTools(toolKeys []string, deps SubAgentDependencies) ([]coreagents.Tool, error) {
	resolved := make([]coreagents.Tool, 0, len(toolKeys))
	seen := make(map[string]struct{}, len(toolKeys))

	var schemaLister bichatsql.SchemaLister
	var schemaDescriber bichatsql.SchemaDescriber

	for _, toolKey := range toolKeys {
		name := strings.TrimSpace(toolKey)
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate tool %q", name)
		}
		seen[name] = struct{}{}

		switch name {
		case "schema_list":
			if deps.QueryExecutor == nil {
				return nil, fmt.Errorf("tool %q requires query executor", name)
			}
			if schemaLister == nil {
				schemaLister = bichatsql.NewQueryExecutorSchemaLister(deps.QueryExecutor,
					bichatsql.WithCountCacheTTL(10*time.Minute),
					bichatsql.WithCacheKeyFunc(tenantCacheKey),
				)
			}
			resolved = append(resolved, toolsql.NewSchemaListTool(schemaLister, toolsql.WithSchemaListViewAccess(deps.ViewAccess)))
		case "schema_describe":
			if deps.QueryExecutor == nil {
				return nil, fmt.Errorf("tool %q requires query executor", name)
			}
			if schemaDescriber == nil {
				schemaDescriber = bichatsql.NewQueryExecutorSchemaDescriber(deps.QueryExecutor)
			}
			resolved = append(resolved, toolsql.NewSchemaDescribeTool(schemaDescriber, toolsql.WithSchemaDescribeViewAccess(deps.ViewAccess)))
		case "sql_execute":
			if deps.QueryExecutor == nil {
				return nil, fmt.Errorf("tool %q requires query executor", name)
			}
			resolved = append(resolved, toolsql.NewSQLExecuteTool(deps.QueryExecutor, toolsql.WithViewAccessControl(deps.ViewAccess)))
		case "artifact_reader":
			if deps.ArtifactReaderTool != nil {
				resolved = append(resolved, deps.ArtifactReaderTool)
				continue
			}
			if deps.ChatRepository == nil {
				return nil, fmt.Errorf("tool %q requires chat repository", name)
			}
			if deps.FileStorage == nil {
				return nil, fmt.Errorf("tool %q requires file storage", name)
			}
			resolved = append(resolved, artifacts.NewArtifactReaderTool(deps.ChatRepository, deps.FileStorage))
		case "ask_user_question":
			resolved = append(resolved, hitl.NewAskUserQuestionTool())
		default:
			return nil, fmt.Errorf("unknown tool %q", name)
		}
	}

	return resolved, nil
}
