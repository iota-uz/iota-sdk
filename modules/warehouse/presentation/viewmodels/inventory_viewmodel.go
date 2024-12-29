package viewmodels

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/domain/aggregates/user"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Check struct {
	ID         string
	Type       string
	Status     string
	Name       string
	Results    []*CheckResult
	CreatedAt  string
	FinishedAt string
	CreatedBy  *user.User
	FinishedBy *user.User
}

type CheckResult struct {
	ID               string
	PositionID       string
	Position         *Position
	ExpectedQuantity string
	ActualQuantity   string
	Difference       string
	CreatedAt        string
}

func (c *Check) LocalizedStatus(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("WarehouseInventory.Single.Statuses.%s", c.Status),
		},
	})
}

func (c *Check) LocalizedType(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("WarehouseInventory.Single.Types.%s", c.Type),
		},
	})
}
