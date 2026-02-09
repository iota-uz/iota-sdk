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
	// Keys use PascalCase per segment (e.g., "Welcome.Title", "Input.Placeholder").
	GetAll(locale string) map[string]string

	// Locales returns the list of supported locales.
	Locales() []string
}

// Translations is a map of locale -> key -> value.
// Keys use PascalCase per segment: "Welcome.Title", "Chat.NewChat", etc.
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

// pascalSegment returns the segment with first character uppercased (PascalCase).
func pascalSegment(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// flattenMap converts a nested map to dot-notation keys with PascalCase per segment.
// e.g. {"welcome": {"title": "Hi"}} -> {"Welcome.Title": "Hi"}
func flattenMap(m map[string]any, prefix string) map[string]string {
	result := make(map[string]string)

	for k, v := range m {
		segment := pascalSegment(k)
		key := segment
		if prefix != "" {
			key = prefix + "." + segment
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
// Keys use PascalCase per segment (e.g. Input.Placeholder, Welcome.Title).
func DefaultTranslations() Translations {
	return Translations{
		"en": {
			// Welcome screen
			"Welcome.Title":       "Welcome to BiChat",
			"Welcome.Description": "Your intelligent business analytics assistant. Ask questions about your data, generate reports, or explore insights.",
			"Welcome.TryAsking":   "Try asking",

			// Chat header
			"Chat.NewChat":  "New Chat",
			"Chat.Archived": "Archived",
			"Chat.Pinned":   "Pinned",
			"Chat.GoBack":   "Go back",

			// Message input
			"Input.Placeholder":    "Type a message...",
			"Input.AttachFiles":    "Attach files",
			"Input.AttachImages":   "Attach images",
			"Input.DropImages":     "Drop images here",
			"Input.SendMessage":    "Send message",
			"Input.AiThinking":     "AI is thinking...",
			"Input.Processing":     "Processing...",
			"Input.MessagesQueued": "{count} message(s) queued",
			"Input.DismissError":   "Dismiss error",
			"Input.DropFiles":      "Drop files here",
			"Input.FilesAdded":     "Files added",
			"Input.FileInput":      "File input",
			"Input.MessageInput":   "Message input",
			"Input.ShiftEnterHint": "Press Enter to send, Shift+Enter for new line",

			// Slash commands
			"Slash.ClearDescription":              "Clear history",
			"Slash.DebugDescription":               "Debug mode",
			"Slash.CompactDescription":             "Compact history",
			"Slash.CommandsList":                  "Commands",
			"Slash.NoMatches":                     "No matches",
			"Slash.DebugBadge":                    "Debug",
			"Slash.DebugPromptTokens":             "Prompt tokens",
			"Slash.DebugCompletionTokens":         "Completion tokens",
			"Slash.DebugTotalTokens":              "Total tokens",
			"Slash.DebugSessionUsageUnavailable":  "Session usage unavailable",
			"Slash.DebugPolicyMaxContextWindow":   "Policy max context window",
			"Slash.DebugModelMaxContextWindow":    "Model max context window",
			"Slash.DebugEffectiveContextWindow":   "Effective context window",
			"Slash.DebugContextUsage":             "Context usage",
			"Slash.CompactingTitle":               "Compacting",
			"Slash.CompactingSubtitle":             "Compacting history...",
			"Slash.DebugArguments":                  "Arguments",
			"Slash.DebugCopyTrace":                  "Copy trace",
			"Slash.DebugCopied":                     "Copied",
			"Slash.DebugResult":                     "Result",
			"Slash.DebugError":                      "Error",
			"Slash.DebugGeneration":                 "Generation",
			"Slash.DebugTokensPerSecond":            "Tokens/s",
			"Slash.DebugCachedTokens":               "Cached tokens",
			"Slash.DebugPanelTitle":                 "Debug",
			"Slash.DebugToolCalls":                  "Tool calls",
			"Slash.DebugUnavailable":                "Debug data unavailable",

			// Date groups (session grouping)
			"DateGroup.Today":      "Today",
			"DateGroup.Yesterday":  "Yesterday",
			"DateGroup.Last7Days":  "Last 7 Days",
			"DateGroup.Last30Days": "Last 30 Days",
			"DateGroup.Older":      "Older",

			// Attachments
			"Attachment.FileAdded":   "File added: {{size}}",
			"Attachment.InvalidFile":  "Invalid file",
			"Attachment.SelectFiles":  "Select files",

			// Artifacts
			"Artifacts.Title":              "Artifacts",
			"Artifacts.ToggleShow":          "Show artifacts",
			"Artifacts.ToggleHide":          "Hide artifacts",
			"Artifacts.Resize":              "Resize panel",
			"Artifacts.Empty":               "No artifacts yet",
			"Artifacts.EmptySubtitle":       "Artifacts will appear here",
			"Artifacts.GroupCharts":         "Charts",
			"Artifacts.GroupCodeOutputs":    "Code Outputs",
			"Artifacts.GroupExports":        "Exports",
			"Artifacts.GroupAttachments":    "Attachments",
			"Artifacts.GroupOther":          "Other",
			"Artifacts.Rename":              "Rename",
			"Artifacts.Delete":              "Delete",
			"Artifacts.RenameFailed":        "Failed to rename",
			"Artifacts.DeleteConfirm":       "Delete this artifact?",
			"Artifacts.DeleteFailed":        "Failed to delete",
			"Artifacts.FailedToLoad":         "Failed to load artifacts",
			"Artifacts.Loading":              "Loading artifacts...",
			"Artifacts.LoadingMore":          "Loading more...",
			"Artifacts.LoadMore":             "Load more",
			"Artifacts.Unsupported":         "Artifacts panel not available",
			"Artifacts.OpenInNewTab":         "Open in new tab",
			"Artifacts.Download":             "Download",
			"Artifacts.TextPreviewFailed":    "Failed to load preview",
			"Artifacts.PreviewLoading":       "Loading preview...",
			"Artifacts.PreviewUnavailable":   "Preview unavailable",
			"Artifacts.TextPreviewTruncated": "Preview truncated",
			"Artifacts.ChartUnavailable":     "Chart not renderable",
			"Artifacts.ImageUnavailable":     "Image unavailable",
			"Artifacts.DownloadUnavailable":  "Download unavailable",
			"Artifacts.OfficePreviewUnavailable": "Office preview unavailable",
			"Artifacts.PreviewNotSupported":  "Preview not supported",

			// Alert
			"Alert.Retry": "Retry",

			// Chat (extra)
			"Chat.GoBack":            "Go back",
			"Chat.ReadOnly":          "Read only",
			"Chat.Retry":             "Retry",
			"Chat.DismissNotification": "Dismiss",

			// Assistant
			"Assistant.Explanation": "Explanation",

			// Welcome
			"Welcome.QuickStart": "Quick Start",

			// Chart (extra)
			"Chart.Exporting":   "Exporting...",
			"Chart.DownloadPNG": "Download PNG",

			// Question
			"Question.Other": "Other",

			// Error (extra)
			"Error.AllQuestionsRequired": "Please answer all questions",
			"Error.CustomTextRequired":   "Please specify: {{question}}",

			// Common
			"Common.Close":       "Close",
			"Common.Cancel":      "Cancel",
			"Common.Back":        "Back",
			"Common.Clear":       "Clear",
			"Common.Untitled":    "Untitled",
			"Common.Generating":  "Generating...",
			"Common.Pinned":      "Pinned",

			// Sidebar
			"Sidebar.MyChats":                  "My chats",
			"Sidebar.AllChats":                 "All chats",
			"Sidebar.ChatSessions":             "Chat sessions",
			"Sidebar.CloseSidebar":             "Close sidebar",
			"Sidebar.SearchChats":              "Search chats",
			"Sidebar.CreateNewChat":            "New chat",
			"Sidebar.ArchivedChats":            "Archived chats",
			"Sidebar.PinnedChats":              "Pinned chats",
			"Sidebar.ChatOptions":              "Chat options",
			"Sidebar.RenameChat":               "Rename chat",
			"Sidebar.UnpinChat":                "Unpin",
			"Sidebar.PinChat":                 "Pin",
			"Sidebar.RegenerateTitle":          "Regenerate title",
			"Sidebar.ArchiveChat":              "Archive chat",
			"Sidebar.DeleteChat":              "Delete chat",
			"Sidebar.NoChatsYet":               "No chats yet",
			"Sidebar.NoChatsFound":             "No results for \"{{query}}\"",
			"Sidebar.CreateOneToGetStarted":   "Create one to get started",
			"Sidebar.ChatRenamedSuccessfully":  "Chat renamed",
			"Sidebar.FailedToRenameChat":       "Failed to rename",
			"Sidebar.TitleRegenerated":         "Title updated",
			"Sidebar.FailedToRegenerateTitle": "Failed to regenerate title",
			"Sidebar.FailedToLoadSessions":     "Failed to load sessions",
			"Sidebar.FailedToArchiveChat":       "Failed to archive",
			"Sidebar.FailedToTogglePin":        "Failed to update pin",
			"Sidebar.ArchiveChatSession":       "Archive chat",
			"Sidebar.ArchiveChatMessage":       "Archive this chat?",
			"Sidebar.ArchiveButton":            "Archive",

			// Archived
			"Archived.Title":                    "Archived",
			"Archived.BackToChats":             "Back to chats",
			"Archived.SearchArchivedChats":     "Search archived",
			"Archived.NoArchivedChats":         "No archived chats",
			"Archived.NoArchivedChatsDescription": "Archived chats will appear here",
			"Archived.NoResults":               "No results",
			"Archived.NoResultsDescription":    "No chats match \"{{query}}\"",
			"Archived.RestoreChat":             "Restore chat",
			"Archived.RestoreChatMessage":      "Restore this chat?",
			"Archived.RestoreButton":           "Restore",
			"Archived.ChatRestoredSuccessfully": "Chat restored",
			"Archived.FailedToRestoreChat":     "Failed to restore",

			// Welcome
			"Welcome.Disclaimer": "This chat is powered by AI. Mistakes are possible.",

			// Retry
			"Retry.Title":       "Retry",
			"Retry.Description": "Something went wrong. Try again.",
			"Retry.Button":      "Retry",

			// System
			"System.ConversationSummary": "Conversation summary",
			"System.LoadingSummary":      "Loading summary...",
			"System.ShowLess":            "Show less",
			"System.ShowMore":            "Show more",

			// Message actions
			"Message.Copy":       "Copy",
			"Message.Copied":     "Copied!",
			"Message.Regenerate": "Regenerate",
			"Message.Edit":       "Edit",
			"Message.Save":       "Save",
			"Message.Cancel":     "Cancel",

			// Assistant turn
			"Assistant.Thinking":   "Thinking...",
			"Assistant.ToolCall":   "Using tool: {name}",
			"Assistant.Generating": "Generating response...",

			// Question form
			"Question.Submit":       "Submit",
			"Question.SelectOne":    "Select one option",
			"Question.SelectMulti":  "Select one or more options",
			"Question.Required":     "This field is required",
			"Question.Other":        "Other",
			"Question.SpecifyOther": "Please specify",

			// Errors
			"Error.Generic":        "Something went wrong",
			"Error.NetworkError":   "Network error. Please try again.",
			"Error.SessionExpired": "Session expired. Please refresh.",
			"Error.FileTooLarge":   "File is too large",
			"Error.InvalidFile":    "Invalid file type",
			"Error.MaxFiles":       "Maximum {max} files allowed",

			// Empty states
			"Empty.NoMessages": "No messages yet",
			"Empty.NoSessions": "No chat sessions",
			"Empty.StartChat":  "Start a new chat to begin",

			// Sources panel
			"Sources.Title":     "Sources",
			"Sources.ViewMore":  "View more",
			"Sources.Citations": "{count} citation(s)",

			// Code outputs
			"CodeOutputs.Title":    "Code Outputs",
			"CodeOutputs.Download": "Download",
			"CodeOutputs.Expand":   "Expand",
			"CodeOutputs.Collapse": "Collapse",

			// Charts
			"Chart.Download":   "Download chart",
			"Chart.Fullscreen": "View fullscreen",
			"Chart.NoData":     "No data available",

			// Example prompt categories
			"Category.Analysis": "Data Analysis",
			"Category.Reports":  "Reports",
			"Category.Insights": "Insights",
		},
	}
}
