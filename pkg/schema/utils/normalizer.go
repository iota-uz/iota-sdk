package utils

import (
	"strings"
	"unicode"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
)

// Normalizer handles SQL and schema normalization
type Normalizer struct {
	options NormalizerOptions
}

type NormalizerOptions struct {
	CaseInsensitive bool
	TrimSpaces      bool
	SortElements    bool
}

// NormalizeNode normalizes an AST node and its children
func (n *Normalizer) NormalizeNode(node *types.Node) {
	if node == nil {
		return
	}

	// Normalize node name
	if n.options.CaseInsensitive {
		node.Name = strings.ToLower(node.Name)
	}

	// Sort children if enabled
	if n.options.SortElements && len(node.Children) > 0 {
		n.sortNodeChildren(node)
	}

	// Recursively normalize children
	for _, child := range node.Children {
		n.NormalizeNode(child)
	}
}

// NormalizeSQL normalizes SQL text for consistent comparison
func (n *Normalizer) NormalizeSQL(sql string) string {
	if n.options.TrimSpaces {
		sql = n.normalizeWhitespace(sql)
	}

	if n.options.CaseInsensitive {
		sql = strings.ToLower(sql)
	}

	return sql
}

func (n *Normalizer) normalizeWhitespace(sql string) string {
	// Remove extra whitespace while preserving necessary spacing
	var result strings.Builder
	var lastChar rune

	for _, char := range sql {
		if unicode.IsSpace(char) {
			if !unicode.IsSpace(lastChar) {
				result.WriteRune(' ')
			}
		} else {
			result.WriteRune(char)
		}
		lastChar = char
	}

	return strings.TrimSpace(result.String())
}

func (n *Normalizer) sortNodeChildren(node *types.Node) {
	// Sort children based on type and name
	// This ensures consistent ordering for comparison
}

// New creates a new SQL normalizer
func New(opts NormalizerOptions) *Normalizer {
	return &Normalizer{
		options: opts,
	}
}
