package help

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var homeGroups = []viewmodels.TaskGroup{
	{Title: "Деньги", Tasks: []viewmodels.HelpTask{{Title: "Клиент оплатил, а полиса нет", ArticlePath: "pay.md"}}},
	{Title: "Претензии", Tasks: []viewmodels.HelpTask{{Title: "Претензия не уходит", ArticlePath: "claim.md"}, {Title: "Резервы", ArticlePath: "reserve.md"}}},
}

// False green: checking only that the configured title appears, which also
// passes when the step path and the default kicker still render beside it.
func TestHome_ConfiguredIntroReplacesDefaultsAndHidesPath(t *testing.T) {
	t.Parallel()

	body := renderHome(t, HomeProps{
		BasePath: "/help",
		Groups:   homeGroups,
		Intro:    viewmodels.HomeIntro{Title: "Что-то пошло не так?", Subtitle: "Найдите свою ситуацию.", HidePath: true},
	})

	require.Contains(t, body, "<h1>Что-то пошло не так?</h1>")
	require.Contains(t, body, "Найдите свою ситуацию.")
	require.Contains(t, body, "Навигатор задач", "an empty field keeps the component text")
	require.NotContains(t, body, "Что вы хотите сделать?")
	require.NotContains(t, body, "help-home-path")
}

// False green: a single group, where the jump links are rightly omitted and
// nothing ties a link to its section.
func TestHome_DefaultIntroAndGroupJumpLinks(t *testing.T) {
	t.Parallel()

	body := renderHome(t, HomeProps{BasePath: "/help", Groups: homeGroups})

	require.Contains(t, body, "<h1>Что вы хотите сделать?</h1>")
	require.Contains(t, body, `class="help-home-path"`)
	require.Contains(t, body, `href="#help-task-group-2"`)
	require.Contains(t, body, `<section id="help-task-group-2"`)
	require.Equal(t, 1, strings.Count(body, `class="help-task-jump"`))

	single := renderHome(t, HomeProps{BasePath: "/help", Groups: homeGroups[:1]})
	require.NotContains(t, single, `class="help-task-jump"`)
}

func renderHome(t *testing.T, props HomeProps) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, Home(props).Render(homeContext(t), &out))
	return out.String()
}

func homeContext(t *testing.T) context.Context {
	t.Helper()
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_, err := bundle.LoadMessageFile("../../../locales/ru.json")
	require.NoError(t, err)
	localizer := i18n.NewLocalizer(bundle, "ru")
	ctx := intl.WithLocalizer(intl.WithLocale(context.Background(), language.Russian), localizer)
	return composables.WithPageCtx(ctx, types.NewPageContext(language.Russian, &url.URL{Path: "/help"}, localizer))
}

// False green: a flat category, where a nested category never gets the
// chance to repeat its parent's filter field.
func TestCategoryNode_LongCategoryGetsOneFilterAndCounts(t *testing.T) {
	t.Parallel()

	nested := viewmodels.CategoryNode{Title: "Портфель"}
	for i := range categoryFilterThreshold {
		nested.Children = append(nested.Children, viewmodels.CategoryNode{Title: "Статья", Path: "p/" + strconv.Itoa(i) + ".md"})
	}
	long := viewmodels.CategoryNode{Title: "01 ERP", Children: []viewmodels.CategoryNode{nested, {Title: "Обзор", Path: "erp.md"}}}
	short := viewmodels.CategoryNode{Title: "CRM", Children: []viewmodels.CategoryNode{{Title: "Клиенты", Path: "crm.md"}}}

	var out bytes.Buffer
	ctx := homeContext(t)
	require.NoError(t, categoryNode("/help", long, nil, false).Render(ctx, &out))
	require.NoError(t, categoryNode("/help", short, nil, false).Render(ctx, &out))
	body := out.String()

	require.Equal(t, 1, strings.Count(body, "data-help-category-filter"))
	require.Contains(t, body, `<span class="help-category-title">ERP</span>`)
	require.Contains(t, body, `aria-label="13 статей">13</span>`)
	require.Contains(t, body, `aria-label="1 статья">1</span>`)
}
