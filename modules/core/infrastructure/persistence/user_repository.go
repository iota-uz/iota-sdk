package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

const (
	userFindQuery = `
        SELECT
            u.id,
            u.first_name,
            u.last_name,
            u.middle_name,
            u.email,
            u.password,
            u.ui_language,
            u.avatar_id,
            u.last_login,
            u.last_ip,
            u.last_action,
            u.created_at,
            u.updated_at,
            up.id,
            up.hash,
            up.path,
            up.size,
            up.mimetype,
            up.created_at,
            up.updated_at
        FROM users u LEFT JOIN uploads up ON u.avatar_id = up.id`

	userInsertQuery = `
        INSERT INTO users (
            first_name,
            last_name,
            middle_name,
            email,
            password,
            ui_language,
            avatar_id,
            created_at,
            updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        RETURNING id`

	userUpdateQuery = `
        UPDATE users SET
            first_name = $1,
            last_name = $2,
            middle_name = $3,
            email = $4,
            password = COALESCE(NULLIF($5, ''), users.password),
            ui_language = $6,
            avatar_id = $7,
            updated_at = $8
        WHERE id = $9`

	userCountQuery = `SELECT COUNT(id) FROM users`

	userUpdateLastLoginQuery = `UPDATE users SET last_login = NOW() WHERE id = $1`

	userUpdateLastActionQuery = `UPDATE users SET last_action = NOW() WHERE id = $1`

	userDeleteQuery     = `DELETE FROM users WHERE id = $1`
	userRoleDeleteQuery = `DELETE FROM user_roles WHERE user_id = $1`
	userRoleInsertQuery = `INSERT INTO user_roles (user_id, role_id) VALUES`
)

type GormUserRepository struct{}

func NewUserRepository() user.Repository {
	return &GormUserRepository{}
}

func (g *GormUserRepository) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		switch f {
		case user.FirstName:
			sortFields = append(sortFields, "u.first_name")
		case user.LastName:
			sortFields = append(sortFields, "u.last_name")
		case user.MiddleName:
			sortFields = append(sortFields, "u.middle_name")
		case user.Email:
			sortFields = append(sortFields, "u.email")
		case user.LastLogin:
			sortFields = append(sortFields, "u.last_login")
		case user.CreatedAt:
			sortFields = append(sortFields, "u.created_at")
		default:
			return nil, fmt.Errorf("unknown sort field: %v", f)
		}
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	query := repo.Join(
		userFindQuery,
		repo.JoinWhere(where...),
		repo.OrderBy(sortFields, params.SortBy.Ascending),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryUsers(ctx, query, args...)
}

func (g *GormUserRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	err = tx.QueryRow(ctx, userCountQuery).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUserRepository) GetAll(ctx context.Context) ([]user.User, error) {
	return g.queryUsers(ctx, userFindQuery)
}

func (g *GormUserRepository) GetByID(ctx context.Context, id uint) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, ErrUserNotFound
	}
	return users[0], nil
}

func (g *GormUserRepository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.email = $1", email)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, ErrUserNotFound
	}
	return users[0], nil
}

func (g *GormUserRepository) Create(ctx context.Context, data user.User) (user.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	dbUser, _ := toDBUser(data)

	err = tx.QueryRow(
		ctx,
		userInsertQuery,
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName,
		dbUser.Email,
		dbUser.Password,
		dbUser.UILanguage,
		dbUser.AvatarID,
		dbUser.CreatedAt,
		dbUser.UpdatedAt,
	).Scan(&dbUser.ID)
	if err != nil {
		return nil, err
	}
	if err := g.updateUserRoles(ctx, dbUser.ID, data.Roles()); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, dbUser.ID)
}

func (g *GormUserRepository) Update(ctx context.Context, data user.User) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	dbUser, _ := toDBUser(data)

	_, err = tx.Exec(
		ctx,
		userUpdateQuery,
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName,
		dbUser.Email,
		dbUser.Password,
		dbUser.UILanguage,
		dbUser.AvatarID,
		dbUser.UpdatedAt,
		dbUser.ID,
	)

	if err != nil {
		return err
	}

	return g.updateUserRoles(ctx, data.ID(), data.Roles())
}

func (g *GormUserRepository) UpdateLastLogin(ctx context.Context, id uint) error {
	return g.execQuery(ctx, userUpdateLastLoginQuery, id)
}

func (g *GormUserRepository) UpdateLastAction(ctx context.Context, id uint) error {
	return g.execQuery(ctx, userUpdateLastActionQuery, id)
}

func (g *GormUserRepository) Delete(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userRoleDeleteQuery, id); err != nil {
		return err
	}
	return g.execQuery(ctx, userDeleteQuery, id)
}

func (g *GormUserRepository) queryUsers(ctx context.Context, query string, args ...interface{}) ([]user.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	var uploads []*models.Upload
	for rows.Next() {
		var u models.User

		var (
			avatarId        sql.NullInt32
			avatarHash      sql.NullString
			avatarPath      sql.NullString
			avatarSize      sql.NullInt32
			avatarMimetype  sql.NullString
			avatarCreatedAt sql.NullTime
			avatarUpdatedAt sql.NullTime
		)

		if err := rows.Scan(
			&u.ID,
			&u.FirstName,
			&u.LastName,
			&u.MiddleName,
			&u.Email,
			&u.Password,
			&u.UILanguage,
			&u.AvatarID,
			&u.LastLogin,
			&u.LastIP,
			&u.LastAction,
			&u.CreatedAt,
			&u.UpdatedAt,
			&avatarId,
			&avatarHash,
			&avatarPath,
			&avatarSize,
			&avatarMimetype,
			&avatarCreatedAt,
			&avatarUpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, &u)
		if avatarId.Valid {
			uploads = append(uploads, &models.Upload{
				ID:        uint(avatarId.Int32),
				Hash:      avatarHash.String,
				Path:      avatarPath.String,
				Size:      int(avatarSize.Int32),
				Mimetype:  avatarMimetype.String,
				CreatedAt: avatarCreatedAt.Time,
				UpdatedAt: avatarUpdatedAt.Time,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	uploadMap := make(map[uint]*models.Upload)
	for _, u := range uploads {
		uploadMap[u.ID] = u
	}

	entities := make([]user.User, 0, len(users))
	for _, u := range users {
		roles, err := g.userRoles(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		avatar, _ := uploadMap[uint(u.AvatarID.Int32)]
		entity, err := ToDomainUser(u, avatar, roles)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func (g *GormUserRepository) rolePermissions(ctx context.Context, roleID uint) ([]*models.Permission, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(
		ctx,
		`
		SELECT p.id, p.name, p.resource, p.action, p.modifier, p.description
		FROM role_permissions rp LEFT JOIN permissions p ON rp.permission_id = p.id WHERE role_id = $1`,
		roleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []*models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Resource,
			&p.Action,
			&p.Modifier,
			&p.Description,
		); err != nil {
			return nil, err
		}
		permissions = append(permissions, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (g *GormUserRepository) userRoles(ctx context.Context, userID uint) ([]role.Role, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
		SELECT
			r.id,
			r.name,
			r.description,
			r.created_at,
			r.updated_at
		FROM user_roles ur LEFT JOIN roles r ON ur.role_id = r.id WHERE ur.user_id = $1
	`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*models.Role
	for rows.Next() {
		var r models.Role
		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.Description,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		roles = append(roles, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	entities := make([]role.Role, 0, len(roles))
	for _, r := range roles {
		permissions, err := g.rolePermissions(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		entity, err := toDomainRole(r, permissions)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func (g *GormUserRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}

func (g *GormUserRepository) updateUserRoles(ctx context.Context, userID uint, roles []role.Role) error {
	// Delete existing roles
	if err := g.execQuery(ctx, userRoleDeleteQuery, userID); err != nil {
		return err
	}

	values := make([][]interface{}, 0, len(roles)*2)
	for _, r := range roles {
		values = append(values, []interface{}{userID, r.ID()})
	}
	q, args := repo.BuildBatchInsertQueryN(userRoleInsertQuery, values)
	if err := g.execQuery(ctx, q, args...); err != nil {
		return err
	}
	return nil
}
