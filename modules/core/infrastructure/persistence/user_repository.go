package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/mapping"

	// "github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

func NewUserRepository() user.Repository {
	return &GormUserRepository{
		roleRepo: NewRoleRepository(),
	}
}

type GormUserRepository struct {
	roleRepo role.Repository
}

func (g *GormUserRepository) GetPaginated(
	ctx context.Context, params *user.FindParams,
) ([]*user.User, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}

	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, first_name, last_name, middle_name, email, password, ui_language, avatar_id, last_login, last_ip, last_action, employee_id, created_at, updated_at FROM users
		WHERE `+strings.Join(where, " AND ")+`
	`, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	users := make([]*user.User, 0)

	for rows.Next() {
		var user models.User
		var middleName, lastIp sql.NullString
		var avatarID, employeeID sql.NullInt32
		var lastLogin, lastAction sql.NullTime
		if err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&middleName,
			&user.Email,
			&user.Password,
			&user.UiLanguage,
			&avatarID,
			&lastLogin,
			&lastIp,
			&lastAction,
			&employeeID,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if avatarID.Valid {
			user.AvatarID = mapping.Pointer(uint(avatarID.Int32))
		}

		if lastLogin.Valid {
			user.LastLogin = mapping.Pointer(lastLogin.Time)
		}

		if middleName.Valid {
			user.MiddleName = mapping.Pointer(middleName.String)
		}

		if lastIp.Valid {
			user.LastIP = mapping.Pointer(lastIp.String)
		}

		if lastAction.Valid {
			user.LastAction = mapping.Pointer(lastAction.Time)
		}

		if employeeID.Valid {
			user.EmployeeID = mapping.Pointer(uint(employeeID.Int32))
		}

		domainUser, err := ToDomainUser(&user)
		if err != nil {
			return nil, err
		}

		if domainUser.Roles, err = g.roleRepo.GetPaginated(ctx, &role.FindParams{
			UserID:            user.ID,
			AttachPermissions: true,
		}); err != nil {
			return nil, err
		}

		users = append(users, domainUser)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (g *GormUserRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
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
		return nil, composables.ErrNoTx
	}
	var users []*models.User
	if err := tx.Find(&users).Error; err != nil {
		return nil, err
	}
	entities := make([]*user.User, len(users))
	for i, row := range users {
		entities[i], _ = ToDomainUser(row)
	}
	return entities, nil
}

func (g *GormUserRepository) GetByID(ctx context.Context, id uint) (*user.User, error) {
	users, err := g.GetPaginated(ctx, &user.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, ErrUserNotFound
	} else {
		return users[0], nil
	}
}

func (g *GormUserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var row models.User
	if err := tx.Preload("Roles").Preload("Roles.Permissions").First(&row, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return ToDomainUser(&row)
}

func (g *GormUserRepository) CreateOrUpdate(ctx context.Context, user *user.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
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
		return composables.ErrNoTx
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
		return composables.ErrNoTx
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
		return composables.ErrNoTx
	}
	return tx.Model(&models.User{}).Where("id = ?", id).Update("last_login", "NOW()").Error //nolint:exhaustruct
}

func (g *GormUserRepository) UpdateLastAction(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Model(&models.User{}).Where("id = ?", id).Update("last_action", "NOW()").Error //nolint:exhaustruct
}

func (g *GormUserRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.User{}, id).Error //nolint:exhaustruct
}
