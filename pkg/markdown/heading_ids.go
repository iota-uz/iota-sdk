package markdown

import (
	"bytes"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// headingIDs generates readable heading anchors for any script. Goldmark's
// default keeps ASCII letters only, so a Cyrillic heading became "-" or
// "---2"; here letters and digits of every script stay, lowercased.
type headingIDs struct {
	used map[string]struct{}
}

func newHeadingIDs() parser.IDs {
	return &headingIDs{used: map[string]struct{}{}}
}

func (s *headingIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	var id bytes.Buffer
	dash := false
	for len(value) > 0 {
		r, size := utf8.DecodeRune(value)
		value = value[size:]
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if dash && id.Len() > 0 {
				id.WriteByte('-')
			}
			dash = false
			id.WriteRune(unicode.ToLower(r))
		case r == '_':
			id.WriteByte('_')
			dash = false
		case unicode.IsSpace(r) || r == '-':
			dash = true
		}
	}
	base := id.String()
	if base == "" {
		if kind == ast.KindHeading {
			base = "heading"
		} else {
			base = "id"
		}
	}
	candidate := base
	for n := 1; ; n++ {
		if _, taken := s.used[candidate]; !taken {
			break
		}
		candidate = base + "-" + strconv.Itoa(n)
	}
	s.used[candidate] = struct{}{}
	return []byte(candidate)
}

func (s *headingIDs) Put(value []byte) {
	s.used[string(value)] = struct{}{}
}
