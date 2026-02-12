package prompts

import "strings"

// ProjectPromptExtensionProvider provides project-scoped prompt extensions.
// Implementations can load extension text from env vars, embedded files, or external stores.
type ProjectPromptExtensionProvider interface {
	ProjectPromptExtension() (string, error)
}

// ProjectPromptExtensionProviderFunc adapts a function to ProjectPromptExtensionProvider.
type ProjectPromptExtensionProviderFunc func() (string, error)

// ProjectPromptExtension returns extension text from the adapted function.
func (f ProjectPromptExtensionProviderFunc) ProjectPromptExtension() (string, error) {
	if f == nil {
		return "", nil
	}
	return f()
}

// NormalizeProjectPromptExtension trims leading and trailing whitespace.
func NormalizeProjectPromptExtension(text string) string {
	return strings.TrimSpace(text)
}
