package composables

import (
	"errors"
	ut "github.com/go-playground/universal-translator"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"net/http"
	"strconv"

	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type PageContext struct {
	Title         string
	Locale        string
	Localizer     *i18n.Localizer
	UniTranslator ut.Translator
	User          *user.User
	Pathname      string
}

func (p *PageContext) T(k string) string {
	return p.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: k})
}

var errLocalizerNotFound = errors.New("localizer not found")

type PaginationParams struct {
	Limit  int
	Offset int
}

func UsePaginated(r *http.Request) PaginationParams {
	config := configuration.Use()
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit > config.MaxPageSize {
		limit = config.PageSize
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	return PaginationParams{
		Limit:  limit,
		Offset: page * limit,
	}
}

type PageData struct {
	Title       string
	Description string
}

func NewPageData(title string, description string) *PageData {
	return &PageData{
		Title:       title,
		Description: description,
	}
}

func UsePageCtx(r *http.Request, pageData *PageData) (*PageContext, error) {
	localizer, found := UseLocalizer(r.Context())
	if !found {
		return nil, errLocalizerNotFound
	}
	uniTranlator, found := UseUniLocalizer(r.Context())
	if !found {
		return nil, errLocalizerNotFound
	}
	locale := composables.UseLocale(r.Context(), language.English)
	return &PageContext{
		Pathname:      r.URL.Path,
		Localizer:     localizer,
		Title:         localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: pageData.Title}), //nolint:exhaustruct
		Locale:        locale.String(),
		UniTranslator: uniTranlator,
	}, nil
}
