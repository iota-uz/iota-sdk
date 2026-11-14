package controllers

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
)

// anchorHref matches the href of an anchor in sanitized article HTML, where
// attribute values are always double-quoted and escaped.
var anchorHref = regexp.MustCompile(`(<a\b[^>]*?\shref=")([^"]*)(")`)

// resolveRelativeLinks makes the relative links of a contributed source's
// article absolute against the article's own URL (docURL), the way a browser
// resolves them. A source document has no file extension and no media, so its
// relative links point at sibling documents of the same source ("../reference/crm"
// from <mount>/guides/intro is <mount>/reference/crm); resolving them here keeps
// them valid wherever the rendered fragment ends up. Absolute paths, links with
// a scheme or host and fragment-only links are left as they are.
func resolveRelativeLinks(rendered template.HTML, docURL string) template.HTML {
	base := &url.URL{Path: docURL}
	out := anchorHref.ReplaceAllStringFunc(string(rendered), func(match string) string {
		parts := anchorHref.FindStringSubmatch(match)
		ref, err := url.Parse(html.UnescapeString(parts[2]))
		if err != nil || ref.IsAbs() || ref.Host != "" || ref.Path == "" || ref.Path[0] == '/' {
			return match
		}
		return parts[1] + template.HTMLEscapeString(base.ResolveReference(ref).String()) + parts[3]
	})
	return template.HTML(out)
}
