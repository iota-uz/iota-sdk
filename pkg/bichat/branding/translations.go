package branding

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

// TranslationProvider provides localized strings for the UI.
// Implementations can load translations from files, databases, or other sources.
type TranslationProvider interface {
	// Get returns the translation for a key in the specified locale.
	// Returns the key itself if translation is not found.
	Get(locale, key string) string

	// GetAll returns all translations for a locale as a flat map.
	// Keys use dot notation (e.g., "welcome.title").
	GetAll(locale string) map[string]string

	// Locales returns the list of supported locales.
	Locales() []string
}

// Translations is a map of locale -> key -> value.
// Keys use dot notation: "welcome.title", "chat.newChat", etc.
type Translations map[string]map[string]string

// FileTranslationProvider loads translations from JSON files.
type FileTranslationProvider struct {
	mu           sync.RWMutex
	translations Translations
	locales      []string
	fallback     string
}

// FileTranslationProviderOption is a functional option.
type FileTranslationProviderOption func(*FileTranslationProvider)

// WithFallbackLocale sets the fallback locale when a key is not found.
func WithFallbackLocale(locale string) FileTranslationProviderOption {
	return func(p *FileTranslationProvider) {
		p.fallback = locale
	}
}

// NewFileTranslationProvider creates a provider that loads translations from a filesystem.
// The filesystem should contain JSON files named by locale (e.g., "en.json", "ru.json").
//
// Example:
//
//	//go:embed locales/*.json
//	var localesFS embed.FS
//
//	provider, err := branding.NewFileTranslationProvider(localesFS, "locales")
func NewFileTranslationProvider(filesystem fs.FS, basePath string, opts ...FileTranslationProviderOption) (*FileTranslationProvider, error) {
	p := &FileTranslationProvider{
		translations: make(Translations),
		fallback:     "en",
	}

	for _, opt := range opts {
		opt(p)
	}

	// Read all JSON files in the directory
	entries, err := fs.ReadDir(filesystem, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read translations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		locale := strings.TrimSuffix(entry.Name(), ".json")
		filePath := basePath + "/" + entry.Name()

		content, err := fs.ReadFile(filesystem, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read translation file %s: %w", filePath, err)
		}

		var nested map[string]any
		if err := json.Unmarshal(content, &nested); err != nil {
			return nil, fmt.Errorf("failed to parse translation file %s: %w", filePath, err)
		}

		// Flatten nested structure to dot notation
		p.translations[locale] = flattenMap(nested, "")
		p.locales = append(p.locales, locale)
	}

	return p, nil
}

// NewEmbedTranslationProvider is a convenience constructor for embed.FS.
func NewEmbedTranslationProvider(efs embed.FS, basePath string, opts ...FileTranslationProviderOption) (*FileTranslationProvider, error) {
	return NewFileTranslationProvider(efs, basePath, opts...)
}

// Get returns the translation for a key, falling back to fallback locale if needed.
func (p *FileTranslationProvider) Get(locale, key string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try requested locale
	if localeMap, ok := p.translations[locale]; ok {
		if value, ok := localeMap[key]; ok {
			return value
		}
	}

	// Try fallback locale
	if locale != p.fallback {
		if localeMap, ok := p.translations[p.fallback]; ok {
			if value, ok := localeMap[key]; ok {
				return value
			}
		}
	}

	// Return key as last resort
	return key
}

// GetAll returns all translations for a locale.
func (p *FileTranslationProvider) GetAll(locale string) map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Start with fallback locale
	result := make(map[string]string)
	if fallbackMap, ok := p.translations[p.fallback]; ok {
		for k, v := range fallbackMap {
			result[k] = v
		}
	}

	// Override with requested locale
	if locale != p.fallback {
		if localeMap, ok := p.translations[locale]; ok {
			for k, v := range localeMap {
				result[k] = v
			}
		}
	}

	return result
}

// Locales returns supported locales.
func (p *FileTranslationProvider) Locales() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]string{}, p.locales...)
}

// MapTranslationProvider uses an in-memory map for translations.
// Useful for testing or simple configurations.
type MapTranslationProvider struct {
	translations Translations
	fallback     string
}

// NewMapTranslationProvider creates a provider from a translations map.
func NewMapTranslationProvider(translations Translations, fallback string) *MapTranslationProvider {
	return &MapTranslationProvider{
		translations: translations,
		fallback:     fallback,
	}
}

// Get returns the translation for a key.
func (p *MapTranslationProvider) Get(locale, key string) string {
	if localeMap, ok := p.translations[locale]; ok {
		if value, ok := localeMap[key]; ok {
			return value
		}
	}
	if locale != p.fallback {
		if localeMap, ok := p.translations[p.fallback]; ok {
			if value, ok := localeMap[key]; ok {
				return value
			}
		}
	}
	return key
}

// GetAll returns all translations for a locale.
func (p *MapTranslationProvider) GetAll(locale string) map[string]string {
	result := make(map[string]string)
	if fallbackMap, ok := p.translations[p.fallback]; ok {
		for k, v := range fallbackMap {
			result[k] = v
		}
	}
	if locale != p.fallback {
		if localeMap, ok := p.translations[locale]; ok {
			for k, v := range localeMap {
				result[k] = v
			}
		}
	}
	return result
}

// Locales returns supported locales.
func (p *MapTranslationProvider) Locales() []string {
	locales := make([]string, 0, len(p.translations))
	for locale := range p.translations {
		locales = append(locales, locale)
	}
	return locales
}

// flattenMap converts a nested map to dot-notation keys.
func flattenMap(m map[string]any, prefix string) map[string]string {
	result := make(map[string]string)

	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case string:
			result[key] = val
		case map[string]any:
			for fk, fv := range flattenMap(val, key) {
				result[fk] = fv
			}
		}
	}

	return result
}

// DefaultTranslations returns the default English translations.
// These serve as fallback when no translations are configured.
func DefaultTranslations() Translations {
	return Translations{
		"en": {
			// Welcome screen
			"welcome.title":       "Welcome to BiChat",
			"welcome.description": "Your intelligent business analytics assistant. Ask questions about your data, generate reports, or explore insights.",
			"welcome.tryAsking":   "Try asking",

			// Chat header
			"chat.newChat":  "New Chat",
			"chat.archived": "Archived",
			"chat.pinned":   "Pinned",
			"chat.goBack":   "Go back",

			// Message input
			"input.placeholder":    "Type a message...",
			"input.attachFiles":    "Attach files",
			"input.attachImages":   "Attach images",
			"input.dropImages":     "Drop images here",
			"input.sendMessage":    "Send message",
			"input.aiThinking":     "AI is thinking...",
			"input.processing":     "Processing...",
			"input.messagesQueued": "{count} message(s) queued",
			"input.dismissError":   "Dismiss error",

			// Message actions
			"message.copy":       "Copy",
			"message.copied":     "Copied!",
			"message.regenerate": "Regenerate",
			"message.edit":       "Edit",
			"message.save":       "Save",
			"message.cancel":     "Cancel",

			// Assistant turn
			"assistant.thinking":   "Thinking...",
			"assistant.toolCall":   "Using tool: {name}",
			"assistant.generating": "Generating response...",

			// Question form
			"question.submit":       "Submit",
			"question.selectOne":    "Select one option",
			"question.selectMulti":  "Select one or more options",
			"question.required":     "This field is required",
			"question.other":        "Other",
			"question.specifyOther": "Please specify",

			// Errors
			"error.generic":        "Something went wrong",
			"error.networkError":   "Network error. Please try again.",
			"error.sessionExpired": "Session expired. Please refresh.",
			"error.fileTooLarge":   "File is too large",
			"error.invalidFile":    "Invalid file type",
			"error.maxFiles":       "Maximum {max} files allowed",

			// Empty states
			"empty.noMessages": "No messages yet",
			"empty.noSessions": "No chat sessions",
			"empty.startChat":  "Start a new chat to begin",

			// Sources panel
			"sources.title":     "Sources",
			"sources.viewMore":  "View more",
			"sources.citations": "{count} citation(s)",

			// Code outputs
			"codeOutputs.title":    "Code Outputs",
			"codeOutputs.download": "Download",
			"codeOutputs.expand":   "Expand",
			"codeOutputs.collapse": "Collapse",

			// Charts
			"chart.download":   "Download chart",
			"chart.fullscreen": "View fullscreen",
			"chart.noData":     "No data available",

			// Example prompt categories
			"category.analysis": "Data Analysis",
			"category.reports":  "Reports",
			"category.insights": "Insights",
		},
	}
}
