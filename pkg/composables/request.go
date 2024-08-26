package composables

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	errLocalizerNotFound = errors.New("localizer not found")
	config               = configuration.Use()
)

type PaginationParams struct {
	Limit  int
	Offset int
}

func UsePaginated(r *http.Request) PaginationParams {
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

func UsePageCtx(r *http.Request, pageData *PageData) (*types.PageContext, error) {
	localizer, found := UseLocalizer(r.Context())
	if !found {
		return nil, errLocalizerNotFound
	}
	locale := composables.UseLocale(r.Context(), language.English)
	return &types.PageContext{
		Pathname:  r.URL.Path,
		Localizer: localizer,
		Title:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: pageData.Title}),
		Locale:    locale.String(),
	}, nil
}
