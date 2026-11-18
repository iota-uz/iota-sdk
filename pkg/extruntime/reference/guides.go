package reference

import (
	"path"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

// RepositoryURL is the canonical repository the guides' non-guide links point at.
const RepositoryURL = "https://github.com/iota-uz/iota-sdk"

// guideDir is where the embedded guides live in the repository; relative
// links of a guide resolve against it.
const guideDir = "docs/extensions"

// guideDoc is one embedded guide.
type guideDoc struct {
	// Path is the Help Center document path.
	Path string
	// File is the file name in the embedded filesystem.
	File string
}

var guideDocs = []guideDoc{
	{Path: GuidesPath + "/authoring", File: "authoring.md"},
	{Path: GuidesPath + "/consumer", File: "consumer.md"},
}

// SourceRef is the git ref the repository links point at: the release tag of
// this build when the SDK identity names one, otherwise main.
func SourceRef() string {
	if v := strings.TrimSpace(sdkidentity.ReleaseVersion); v != "" {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return "main"
}

var (
	inlineLink = regexp.MustCompile(`\]\(([^)\s]+)((?:\s+"[^"]*")?)\)`)
	refDef     = regexp.MustCompile(`^(\s{0,3}\[[^\]]+\]:\s*)(\S+)(.*)$`)
)

// RewriteGuideLinks makes a repository markdown file readable inside the Help
// Center: links between the embedded guides become document links of the
// mount, and every other repository-relative link becomes an absolute link to
// the file at ref on GitHub. Links in code spans and fenced blocks, anchors
// and absolute URLs are left alone.
func RewriteGuideLinks(md []byte, ref string) []byte {
	lines := strings.Split(string(md), "\n")
	out := make([]string, 0, len(lines))
	fenced := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case fenced != "":
			if strings.HasPrefix(trimmed, fenced) {
				fenced = ""
			}
			out = append(out, line)
			continue
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fenced = trimmed[:3]
			out = append(out, line)
			continue
		}
		if m := refDef.FindStringSubmatch(line); m != nil {
			out = append(out, m[1]+rewriteTarget(m[2], ref)+m[3])
			continue
		}
		// Odd segments between backticks are code spans.
		segments := strings.Split(line, "`")
		for i := 0; i < len(segments); i += 2 {
			segments[i] = inlineLink.ReplaceAllStringFunc(segments[i], func(link string) string {
				m := inlineLink.FindStringSubmatch(link)
				return "](" + rewriteTarget(m[1], ref) + m[2] + ")"
			})
		}
		out = append(out, strings.Join(segments, "`"))
	}
	return []byte(strings.Join(out, "\n"))
}

func rewriteTarget(target, ref string) string {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") || strings.Contains(target, ":") {
		return target
	}
	file, anchor, _ := strings.Cut(target, "#")
	if anchor != "" {
		anchor = "#" + anchor
	}
	resolved := path.Clean(path.Join(guideDir, file))
	for _, g := range guideDocs {
		if resolved == path.Join(guideDir, g.File) {
			return "../" + g.Path + anchor
		}
	}
	kind := "blob"
	if path.Ext(resolved) == "" {
		kind = "tree"
	}
	return RepositoryURL + "/" + kind + "/" + ref + "/" + resolved + anchor
}

// guideTitle is the first level-1 heading of a guide.
func guideTitle(md []byte, fallback string) string {
	for _, line := range strings.Split(string(md), "\n") {
		if t, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(t)
		}
	}
	return fallback
}
