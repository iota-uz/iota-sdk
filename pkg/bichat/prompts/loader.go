// Package prompts provides interfaces and implementations for loading agent prompts.
// This enables domain-specific prompt customization by allowing consumers to inject
// their own prompt templates.
package prompts

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

// PromptLoader loads prompt templates for agents.
// Implementations can load prompts from embedded filesystems, databases, or other sources.
type PromptLoader interface {
	// Load retrieves a prompt template for the given agent type and renders it with variables.
	// The agentType identifies which prompt to load (e.g., "parent", "sql", "analyst").
	// Variables are passed to the template for rendering.
	Load(ctx context.Context, agentType string, vars map[string]any) (string, error)
}

// FilePromptLoader loads prompts from an embedded or real filesystem.
// Prompt files are expected to be in the format "{agentType}.md" or "{agentType}.txt".
type FilePromptLoader struct {
	fs        fs.FS
	basePath  string
	extension string
	cache     map[string]*template.Template
	mu        sync.RWMutex
}

// FilePromptLoaderOption is a functional option for FilePromptLoader.
type FilePromptLoaderOption func(*FilePromptLoader)

// WithBasePath sets the base path within the filesystem where prompts are located.
func WithBasePath(path string) FilePromptLoaderOption {
	return func(l *FilePromptLoader) {
		l.basePath = path
	}
}

// WithExtension sets the file extension for prompt files (default: ".md").
func WithExtension(ext string) FilePromptLoaderOption {
	return func(l *FilePromptLoader) {
		l.extension = ext
	}
}

// NewFilePromptLoader creates a new FilePromptLoader with the given filesystem.
// The filesystem can be an embed.FS or any fs.FS implementation.
//
// Example with embedded filesystem:
//
//	//go:embed prompts/*.md
//	var promptFS embed.FS
//
//	loader := prompts.NewFilePromptLoader(promptFS, prompts.WithBasePath("prompts"))
func NewFilePromptLoader(filesystem fs.FS, opts ...FilePromptLoaderOption) *FilePromptLoader {
	loader := &FilePromptLoader{
		fs:        filesystem,
		basePath:  "",
		extension: ".md",
		cache:     make(map[string]*template.Template),
	}

	for _, opt := range opts {
		opt(loader)
	}

	return loader
}

// NewEmbedPromptLoader is a convenience constructor for creating a loader from embed.FS.
func NewEmbedPromptLoader(efs embed.FS, opts ...FilePromptLoaderOption) *FilePromptLoader {
	return NewFilePromptLoader(efs, opts...)
}

// Load retrieves and renders a prompt template.
// The agentType is used to construct the filename: {basePath}/{agentType}.{extension}
//
// Variables are available in the template using Go's text/template syntax:
//
//	{{ .Timezone }}
//	{{ .BusinessRules }}
//	{{ range .DataSources }}...{{ end }}
func (l *FilePromptLoader) Load(ctx context.Context, agentType string, vars map[string]any) (string, error) {
	// Check cache first (read lock)
	l.mu.RLock()
	tmpl, ok := l.cache[agentType]
	l.mu.RUnlock()

	if !ok {
		// Build file path
		filename := agentType + l.extension
		if l.basePath != "" {
			filename = filepath.Join(l.basePath, filename)
		}

		// Read file
		content, err := fs.ReadFile(l.fs, filename)
		if err != nil {
			return "", fmt.Errorf("failed to load prompt %q: %w", agentType, err)
		}

		// Parse template
		parsedTmpl, err := template.New(agentType).Parse(string(content))
		if err != nil {
			return "", fmt.Errorf("failed to parse prompt template %q: %w", agentType, err)
		}

		// Double-checked locking: re-check cache and store if still missing
		l.mu.Lock()
		if existingTmpl, exists := l.cache[agentType]; exists {
			// Another goroutine already parsed it, use existing
			tmpl = existingTmpl
		} else {
			// Store parsed template
			l.cache[agentType] = parsedTmpl
			tmpl = parsedTmpl
		}
		l.mu.Unlock()
	}

	// Render template
	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to render prompt %q: %w", agentType, err)
	}

	return buf.String(), nil
}

// StaticPromptLoader returns a fixed prompt regardless of agent type.
// Useful for testing or simple single-agent configurations.
type StaticPromptLoader struct {
	prompt string
}

// NewStaticPromptLoader creates a loader that always returns the same prompt.
func NewStaticPromptLoader(prompt string) *StaticPromptLoader {
	return &StaticPromptLoader{prompt: prompt}
}

// Load returns the static prompt, ignoring agent type and variables.
func (l *StaticPromptLoader) Load(ctx context.Context, agentType string, vars map[string]any) (string, error) {
	return l.prompt, nil
}

// MapPromptLoader stores prompts in an in-memory map.
// Useful for testing or programmatic prompt configuration.
type MapPromptLoader struct {
	prompts map[string]string
}

// NewMapPromptLoader creates a loader with prompts stored in a map.
// Keys are agent types, values are prompt templates (Go text/template syntax supported).
func NewMapPromptLoader(prompts map[string]string) *MapPromptLoader {
	return &MapPromptLoader{prompts: prompts}
}

// Load retrieves and renders a prompt from the map.
func (l *MapPromptLoader) Load(ctx context.Context, agentType string, vars map[string]any) (string, error) {
	promptTemplate, ok := l.prompts[agentType]
	if !ok {
		return "", fmt.Errorf("prompt not found for agent type %q", agentType)
	}

	// Parse and render template
	tmpl, err := template.New(agentType).Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template %q: %w", agentType, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to render prompt %q: %w", agentType, err)
	}

	return buf.String(), nil
}
