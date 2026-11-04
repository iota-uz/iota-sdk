package help

import (
	"context"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// splitAfterLead splits rendered article HTML after its title and lead
// paragraph, where the article meta line belongs. Without a leading H1 the
// meta line goes first.
func splitAfterLead(articleHTML string) (string, string) {
	end := strings.Index(articleHTML, "</h1>")
	if end < 0 {
		return "", articleHTML
	}
	end += len("</h1>")
	rest := articleHTML[end:]
	trimmed := strings.TrimLeft(rest, " \t\r\n")
	if !strings.HasPrefix(trimmed, "<p>") {
		return articleHTML[:end], rest
	}
	leadEnd := strings.Index(trimmed, "</p>")
	if leadEnd < 0 {
		return articleHTML[:end], rest
	}
	cut := end + len(rest) - len(trimmed) + leadEnd + len("</p>")
	return articleHTML[:cut], articleHTML[cut:]
}

var partEmphasis = regexp.MustCompile(`\*\*(.+?)\*\*`)

// partHTML escapes a front-matter part label and renders its **emphasis**.
func partHTML(part string) string {
	return partEmphasis.ReplaceAllString(html.EscapeString(part), "<b>$1</b>")
}

func localize(ctx context.Context, messageID string, data map[string]interface{}, pluralCount interface{}) (string, bool) {
	localizer, ok := intl.UseLocalizer(ctx)
	if !ok {
		return "", false
	}
	label, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
		PluralCount:  pluralCount,
	})
	if err != nil {
		return "", false
	}
	return label, true
}

func pluralLabel(ctx context.Context, messageID string, count int) string {
	if label, ok := localize(ctx, messageID, map[string]interface{}{"Count": count}, count); ok {
		return label
	}
	return strconv.Itoa(count)
}

func readingTimeLabel(ctx context.Context, minutes int) string {
	return pluralLabel(ctx, "HelpCenter.ArticleMeta.ReadingTime", max(1, minutes))
}

func guideCountLabel(ctx context.Context, count int) string {
	return pluralLabel(ctx, "HelpCenter.Tasks.GuideCount", count)
}

func minutesLabel(ctx context.Context, minutes int) string {
	return pluralLabel(ctx, "HelpCenter.Tasks.Minutes", minutes)
}

func updatedLabel(ctx context.Context, updated time.Time) string {
	month, ok := localize(ctx, "HelpCenter.ArticleMeta.Months."+updated.Month().String(), nil, nil)
	if !ok {
		month = updated.Month().String()
	}
	label, ok := localize(ctx, "HelpCenter.ArticleMeta.Updated", map[string]interface{}{
		"Day":   updated.Day(),
		"Month": month,
		"Year":  updated.Year(),
	}, nil)
	if !ok {
		return updated.Format(time.DateOnly)
	}
	return label
}
