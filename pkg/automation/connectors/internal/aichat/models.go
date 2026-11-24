// Package aichat is what the AI connectors of automation graphs (openai, jev) share: the rules for model
// ids, the strict structured-output schema, an OpenAI-compatible chat completions client that sorts the
// provider's failures into catalog.CallError codes, and the usage accounting of a call.
package aichat

import (
	"fmt"
	"regexp"
	"strings"
)

// maxModelLen bounds a model id.
const maxModelLen = 128

var modelIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+\-]*$`)

// CheckModel reports why s cannot be the model of a saved node: it is not an id, or it is an alias.
// A model is saved as an exact id because changing the model of a node is a new version of the graph; an
// alias follows the provider's newest model and would change the saved graph by itself.
func CheckModel(s string) error {
	switch {
	case s == "":
		return fmt.Errorf("a model is an exact model id")
	case len(s) > maxModelLen || !modelIDRe.MatchString(s):
		return fmt.Errorf("a model is an exact model id: letters, digits and . _ - : / @ +, at most %d characters", maxModelLen)
	case IsAlias(s):
		return fmt.Errorf("%q is an alias: it follows the provider's newest model, so a saved graph would change its model by itself; name an exact model id", s)
	}
	return nil
}

// IsAlias reports whether a model id follows the provider's newest model instead of naming one: a
// "latest" segment (gpt-4o-latest, chatgpt-4o-latest, claude-latest) or one of OpenRouter's routers
// (openrouter/auto), which pick the model per request.
func IsAlias(id string) bool {
	l := strings.ToLower(id)
	if strings.HasPrefix(l, "openrouter/") || strings.HasPrefix(l, "~") {
		return true
	}
	for _, seg := range strings.FieldsFunc(l, func(r rune) bool { return strings.ContainsRune("-_./:@+", r) }) {
		if seg == "latest" {
			return true
		}
	}
	return false
}

// VisionModel reports whether the model takes images, by the id's family. The providers' model lists do
// not carry this for every endpoint, and the check runs when a graph is saved, with no request: an
// application with another list replaces it (the connectors' WithVision option).
func VisionModel(id string) bool {
	l := strings.ToLower(id)
	vendor, name := "", l
	if i := strings.LastIndex(l, "/"); i >= 0 {
		vendor, name = l[:i], l[i+1:]
	}
	name, _, _ = strings.Cut(name, ":") // OpenRouter variants: gpt-4o:extended
	prefixed := func(list ...string) bool {
		for _, p := range list {
			if strings.HasPrefix(name, p) {
				return true
			}
		}
		return false
	}
	switch {
	case prefixed("gpt-4o", "chatgpt-4o", "gpt-4.1", "gpt-4.5", "gpt-4-turbo", "gpt-4-vision", "gpt-5"):
		return true
	case prefixed("o1-mini", "o1-preview", "o3-mini"):
		return false
	case prefixed("o1", "o3", "o4"):
		return true
	case strings.HasPrefix(name, "claude-"):
		return !prefixed("claude-2", "claude-instant", "claude-v1")
	case prefixed("gemini"), prefixed("pixtral"):
		return true
	case strings.Contains(name, "vision"), strings.Contains(name, "-vl"), prefixed("llama-4"):
		return vendor != "" || strings.HasPrefix(name, "llama") || strings.HasPrefix(name, "qwen")
	}
	return false
}

// TakesTemperature reports whether the model accepts a temperature other than the default: the
// reasoning models (o1, o3, o4, gpt-5) refuse an explicit one.
func TakesTemperature(id string) bool {
	name := strings.ToLower(id)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	for _, p := range []string{"o1", "o3", "o4", "gpt-5"} {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return true
}
