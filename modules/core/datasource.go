package core

import (
	"context"
	"fmt"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

var _ spotlight.DataSource = &dataSource{}

type dataSource struct {
	userRepo user.Repository
}

func (d *dataSource) Find(ctx context.Context, q string) []spotlight.Item {
	users, err := d.userRepo.GetPaginated(ctx, &user.FindParams{
		Search: q,
	})
	if err != nil {
		return nil
	}

	items := make([]spotlight.Item, len(users))
	for i, user := range users {
		items[i] = spotlight.NewItem(
			icons.UserCircle(icons.Props{Size: "20"}),
			user.FirstName()+" "+user.LastName(),
			fmt.Sprintf("/users/%d", user.ID()),
		)
	}
	return items
}
