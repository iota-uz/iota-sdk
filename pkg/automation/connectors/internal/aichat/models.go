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

// CheckModel reports why s cannot be the model of a saved node: it is not an id, it is an alias, or it
// is a model of a provider that rolls its ids (see Rolling). A model is saved as an exact id because
// changing the model of a node is a new version of the graph; an alias follows the provider's newest
// model and would change the saved graph by itself.
func CheckModel(s string) error {
	switch {
	case s == "":
		return fmt.Errorf("a model is an exact model id")
	case len(s) > maxModelLen || !modelIDRe.MatchString(s):
		return fmt.Errorf("a model is an exact model id: letters, digits and . _ - : / @ +, at most %d characters", maxModelLen)
	case IsAlias(s):
		return fmt.Errorf("%q is an alias: it follows the provider's newest model, so a saved graph would change its model by itself; name an exact model id", s)
	case Rolling(s):
		return fmt.Errorf("%q is a rolling id: the provider moves it to newer snapshots, so a saved graph would change its model by itself; name a dated snapshot such as %s-YYYY-MM-DD", s, s)
	}
	return nil
}

// IsAlias reports whether a model id follows the provider's newest model instead of naming one: a
// "latest" segment (gpt-4o-latest, chatgpt-4o-latest, claude-latest, typesafe/jev-latest), a "~" alias
// of OpenRouter or one of its routers (openrouter/auto).
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

var (
	snapshotRe = regexp.MustCompile(`-(\d{4}-\d{2}-\d{2}|\d{4})$`) // a dated snapshot, or OpenAI's 0613
	// openAIFamily is the ids OpenAI serves: its own, or OpenRouter's openai/<id>.
	openAIFamily = regexp.MustCompile(`^(openai/)?(gpt-|chatgpt-|o[0-9]|text-|omni-|davinci|babbage)`)
	// snapshotlessOpenAI is OpenAI models published without dated snapshots (e.g. gpt-6-luna), accepted
	// as exact ids despite lacking a -YYYY-MM-DD suffix.
	snapshotlessOpenAI = regexp.MustCompile(`^(?:openai/)?gpt-6(?:\.[0-9]+)?-(?:luna|terra|sol|astra)$`)
)

// Rolling reports whether id is an id of OpenAI that the provider moves to newer snapshots (gpt-4o,
// gpt-4o-mini, o3): OpenAI pins a model with its dated snapshot (gpt-4o-mini-2024-07-18), so an id of its
// family without a date is not pinned, except models OpenAI ships without dated snapshots (such as
// gpt-6-luna). An id of any other family is taken as the provider's own: a compatible endpoint names its
// models as it pleases.
func Rolling(id string) bool {
	l := strings.ToLower(id)
	if !openAIFamily.MatchString(l) || snapshotlessOpenAI.MatchString(l) {
		return false
	}
	// A model of the family OpenAI ships with its own fixed number (a preview, a 0613) is pinned too.
	return !snapshotRe.MatchString(l)
}

// vision lists the models known to take images, by their exact base id; a snapshot adds -YYYY-MM-DD. The
// providers' own model lists do not carry the modality for every endpoint, and the check runs when a graph
// is saved, with no request: an application with metadata of its own replaces it (WithVision).
var vision = regexp.MustCompile(`^(?:openai/)?(?:` +
	`gpt-4o(?:-mini)?|chatgpt-4o|gpt-4\.1(?:-mini|-nano)?|gpt-4\.5-preview|gpt-4-turbo|gpt-4-vision-preview|` +
	`gpt-5(?:\.[0-9]+)?(?:-mini|-nano|-pro)?|gpt-6(?:\.[0-9]+)?-(?:luna|terra|sol|astra)|o1|o3(?:-pro)?|o4-mini)` +
	`(?:-\d{4}-\d{2}-\d{2})?$` +
	`|^anthropic/claude-(?:3|3\.5|3\.7|opus-4|sonnet-4|haiku-4|opus-4\.[0-9]|sonnet-4\.[0-9]|haiku-4\.[0-9])(?:-(?:opus|sonnet|haiku))?(?:-\d{8}|-\d{4}-\d{2}-\d{2})?$` +
	`|^google/gemini-[0-9][0-9a-z.\-]*$`)

// VisionModel reports whether the model takes images: a model of the verified list above. A model that only
// shares a prefix with one (gpt-4o-mini-transcribe, gpt-4o-audio-preview, gpt-4o-realtime-preview, whose
// input is audio or text) is not on it.
func VisionModel(id string) bool {
	l := strings.ToLower(id)
	for _, no := range []string{"embedding", "transcribe", "tts", "audio", "realtime", "search", "moderation"} {
		if strings.Contains(l, no) {
			return false
		}
	}
	return vision.MatchString(l)
}

// gpt5DefaultNone is GPT-5.1–5.4 non-pro, non-codex models that default to reasoning effort "none"
// and therefore accept an explicit temperature when reasoning_effort is omitted. GPT-5.0 and GPT-5.5+
// default to reasoning enabled ("medium") and are excluded.
var gpt5DefaultNone = regexp.MustCompile(`^gpt-5\.[1-4](?:-(?:mini|nano))?(?:-\d{4}-\d{2}-\d{2})?$`)

// TakesTemperature reports whether the model accepts a temperature other than the default when
// reasoning_effort is omitted: reasoning models with reasoning enabled by default (o1, o3, o4, gpt-5
// 5.0/5.5+/pro/codex, gpt-6) refuse an explicit one, whereas GPT-5.1–5.4 chat models default to
// reasoning effort "none" and accept it.
func TakesTemperature(id string) bool {
	name := strings.ToLower(id)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if gpt5DefaultNone.MatchString(name) {
		return true
	}
	for _, p := range []string{"o1", "o3", "o4", "gpt-5", "gpt-6"} {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return true
}

var structuredOutputs = regexp.MustCompile(`^(?:openai/)?(?:` +
	`gpt-4o-mini|gpt-4\.1(?:-mini|-nano)?|gpt-5(?:\.[0-9]+)?(?:-mini|-nano|-pro)?|gpt-6(?:\.[0-9]+)?-(?:luna|terra|sol|astra)|o1|o3(?:-mini|-pro)?|o4-mini)` +
	`(?:-\d{4}-\d{2}-\d{2})?$`)

// gpt4oSnapshot is a dated gpt-4o snapshot; Structured Outputs came with 2024-08-06.
var gpt4oSnapshot = regexp.MustCompile(`^(?:openai/)?gpt-4o-(\d{4}-\d{2}-\d{2})$`)

// NoStructuredOutput reports whether id is a model of OpenAI that cannot answer in strict structured output
// (json_schema with strict): gpt-4-turbo, gpt-3.5-turbo, o1-mini, gpt-4o before 2024-08-06 and the like. A
// model of another family is not judged: a compatible endpoint names and serves its models as it pleases.
func NoStructuredOutput(id string) bool {
	l := strings.ToLower(id)
	if !openAIFamily.MatchString(l) {
		return false
	}
	if m := gpt4oSnapshot.FindStringSubmatch(l); m != nil {
		return m[1] < "2024-08-06"
	}
	return !structuredOutputs.MatchString(l)
}
