package pkgdocs

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Links in the markdown of a package are written against the source tree:
//
//   - [text](other.md#section) or ../admin/setup.md — another document of the
//     package, by its file name. The Help Center serves documents without an
//     extension, so the rewrite drops ".md"; the browser then resolves the
//     relative address against the document's own address, which mirrors the
//     file layout.
//   - [text](route:acme.loyalty.cards) — a page or route of the package, by
//     its resource ID. It becomes the route's path.
//
// Everything else is left as written (the Help Center's sanitizer decides what
// survives). Only markdown links are rewritten, not raw HTML.
var (
	inlineLink = regexp.MustCompile(`(!?)(\[(?:[^\[\]\\]|\\.)*\]\(\s*)(<[^>]*>|[^\s)]*)`)
	linkDef    = regexp.MustCompile(`^( {0,3}\[[^\]]+\]:\s*)(<[^>]*>|\S+)`)
	codeSpan   = regexp.MustCompile("(`+)[^`]*?(`+)")
	fenceOpen  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	scheme     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	safePath   = regexp.MustCompile(`^/[A-Za-z0-9._~/%-]*$`)
)

// RewriteLinks resolves the package-relative links of a document (see above)
// against its manifest.
func RewriteLinks(body []byte, m manifest.Manifest) []byte {
	routes := map[string]string{}
	for _, r := range m.Resources {
		if (r.Kind == manifest.ResRoute || r.Kind == manifest.ResPage) && safePath.MatchString(r.Path) {
			routes[r.ID] = r.Path
		}
	}
	rewrite := func(dest string) string {
		wrapped := strings.HasPrefix(dest, "<") && strings.HasSuffix(dest, ">") && len(dest) >= 2
		raw := dest
		if wrapped {
			raw = dest[1 : len(dest)-1]
		}
		out := rewriteDest(raw, routes)
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}
	lines := bytes.Split(body, []byte("\n"))
	var fence string
	for i, line := range lines {
		text := string(line)
		if open := fenceOpen.FindStringSubmatch(text); open != nil {
			switch {
			case fence == "":
				fence = open[1]
				continue
			case open[1][0] == fence[0] && len(open[1]) >= len(fence) && strings.TrimSpace(text) == open[1]:
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		// Blank the code spans so that their content is not rewritten, then
		// splice the rewritten destinations back into the original line.
		masked := codeSpan.ReplaceAllStringFunc(text, func(s string) string { return strings.Repeat(" ", len(s)) })
		var b strings.Builder
		last := 0
		for _, loc := range inlineLink.FindAllStringSubmatchIndex(masked, -1) {
			if masked[loc[2]:loc[3]] == "!" {
				continue
			}
			b.WriteString(text[last:loc[6]])
			b.WriteString(rewrite(text[loc[6]:loc[7]]))
			last = loc[7]
		}
		if def := linkDef.FindStringSubmatchIndex(masked); def != nil && last == 0 {
			b.WriteString(text[:def[4]])
			b.WriteString(rewrite(text[def[4]:def[5]]))
			last = def[5]
		}
		b.WriteString(text[last:])
		lines[i] = []byte(b.String())
	}
	return bytes.Join(lines, []byte("\n"))
}

func rewriteDest(dest string, routes map[string]string) string {
	switch {
	case dest == "" || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "/"):
		return dest
	case strings.HasPrefix(dest, "route:"):
		id, frag, _ := strings.Cut(strings.TrimPrefix(dest, "route:"), "#")
		path, ok := routes[id]
		if !ok || strings.ContainsAny(path, ":{*") {
			// Unknown to this version or parameterised: no address to offer.
			return "#"
		}
		if frag != "" {
			return path + "#" + frag
		}
		return path
	case scheme.MatchString(dest):
		return dest
	}
	target, frag, hasFrag := strings.Cut(dest, "#")
	if strings.HasSuffix(target, ".md") {
		target = strings.TrimSuffix(target, ".md")
		if hasFrag {
			return target + "#" + frag
		}
		return target
	}
	return dest
}
