package definition

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type ParseDocumentOptions struct {
	KnownFields bool
	RequireBody bool
}

type ParsedDocument[T any] struct {
	Path           string
	FrontMatterRaw string
	FrontMatter    T
	Body           string
}

func ParseDocument[T any](content []byte, sourcePath string, opts ParseDocumentOptions) (ParsedDocument[T], error) {
	frontMatterRaw, body, err := SplitFrontMatter(string(content))
	if err != nil {
		return ParsedDocument[T]{}, fmt.Errorf("parse %q: %w", sourcePath, err)
	}

	decoder := yaml.NewDecoder(strings.NewReader(frontMatterRaw))
	decoder.KnownFields(opts.KnownFields)

	var frontMatter T
	if err := decoder.Decode(&frontMatter); err != nil {
		return ParsedDocument[T]{}, fmt.Errorf("parse %q front matter: %w", sourcePath, err)
	}

	body = strings.TrimSpace(body)
	if opts.RequireBody && body == "" {
		return ParsedDocument[T]{}, fmt.Errorf("parse %q: markdown body is required", sourcePath)
	}

	return ParsedDocument[T]{
		Path:           sourcePath,
		FrontMatterRaw: frontMatterRaw,
		FrontMatter:    frontMatter,
		Body:           body,
	}, nil
}

func SplitFrontMatter(raw string) (string, string, error) {
	normalized := strings.TrimPrefix(raw, "\ufeff")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")

	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("missing yaml front matter start delimiter")
	}

	closingIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closingIndex = i
			break
		}
	}
	if closingIndex == -1 {
		return "", "", fmt.Errorf("missing yaml front matter end delimiter")
	}

	frontMatter := strings.Join(lines[1:closingIndex], "\n")
	body := strings.Join(lines[closingIndex+1:], "\n")
	return frontMatter, body, nil
}
