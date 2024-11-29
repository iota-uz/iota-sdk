package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/service"
	"gorm.io/gorm"
)

func NewUserRepository() user.Repository {
	return &GormUserRepository{}
}

type GormUserRepository struct{}

func (g *GormUserRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Preload("Roles").Preload("Roles.Permissions").Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var rows []*models.User
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*user.User, len(rows))
	for i, row := range rows {
		entities[i] = toDomainUser(row)
	}
	return entities, nil
}

func (g *GormUserRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.User{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormUserRepository) GetAll(ctx context.Context) ([]*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var users []*models.User
	if err := tx.Find(&users).Error; err != nil {
		return nil, err
	}
	entities := make([]*user.User, len(users))
	for i, row := range users {
		entities[i] = toDomainUser(row)
	}
	return entities, nil
}

func (g *GormUserRepository) GetByID(ctx context.Context, id uint) (*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var row models.User
	if err := tx.Preload("Roles").Preload("Avatar").Preload("Roles.Permissions").First(&row, id).Error; err != nil {
		return nil, err
	}
	return toDomainUser(&row), nil
}

func (g *GormUserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var row models.User
	if err := tx.Preload("Roles").Preload("Roles.Permissions").First(&row, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return toDomainUser(&row), nil
}

func (g *GormUserRepository) CreateOrUpdate(ctx context.Context, user *user.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	dbUser, dbRoles := toDBUser(user)
	if err := tx.Save(dbUser).Error; err != nil {
		return err
	}
	return tx.Model(dbUser).Association("Roles").Replace(dbRoles)
}

func (g *GormUserRepository) Create(ctx context.Context, user *user.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	dbUser, dbRoles := toDBUser(user)
	if err := tx.Create(dbUser).Error; err != nil {
		return err
	}
	return tx.Model(dbUser).Association("Roles").Append(dbRoles)
}

func (g *GormUserRepository) Update(ctx context.Context, user *user.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	dbUser, dbRoles := toDBUser(user)
	var q *gorm.DB
	if dbUser.AvatarID == nil {
		q = tx.Updates(dbUser)
	} else {
		q = tx.Updates(dbUser).Preload("Avatar")
	}
	if err := q.Error; err != nil {
		return err
	}
	if err := tx.Model(dbUser).Association("Avatar").Find(dbUser.Avatar); err != nil {
		return err
	}
	user.Avatar = ToDomainUpload(dbUser.Avatar)
	if len(dbRoles) == 0 {
		return nil
	}
	return tx.Model(&models.User{}).Association("Roles").Replace(dbRoles)
}

func (g *GormUserRepository) UpdateLastLogin(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Model(&models.User{}).Where("id = ?", id).Update("last_login", "NOW()").Error //nolint:exhaustruct
}

func (g *GormUserRepository) UpdateLastAction(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Model(&models.User{}).Where("id = ?", id).Update("last_action", "NOW()").Error //nolint:exhaustruct
}

func (g *GormUserRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.User{}, id).Error //nolint:exhaustruct
}
