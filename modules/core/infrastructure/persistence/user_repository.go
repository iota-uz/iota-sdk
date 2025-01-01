package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
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
            u.employee_id,
            u.created_at,
            u.updated_at,
            up.id,
            up.hash,
            up.path,
            up.size,
            up.mimetype,
            up.created_at,
            up.updated_at,
            r.id,
            r.name,
            r.description,
            r.created_at,
            r.updated_at,
            p.id,
            p.name,
            p.resource,
            p.action,
            p.modifier
        FROM users u
        LEFT JOIN uploads up ON u.avatar_id = up.id
        LEFT JOIN user_roles ur ON u.id = ur.user_id
        LEFT JOIN roles r ON ur.role_id = r.id
        LEFT JOIN role_permissions rp ON r.id = rp.role_id
        LEFT JOIN permissions p ON rp.permission_id = p.id`

	userInsertQuery = `
        INSERT INTO users (
            first_name,
            last_name,
            middle_name,
            email,
            password,
            ui_language,
            avatar_id,
            employee_id,
            created_at,
            updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        RETURNING id`

	userUpdateQuery = `
        UPDATE users SET
            first_name = $1,
            last_name = $2,
            middle_name = $3,
            email = $4,
            password = $5,
            ui_language = $6,
            avatar_id = $7,
            employee_id = $8,
            updated_at = $9
        WHERE id = $10`

	userUpdateLastLoginQuery = `UPDATE users SET last_login = NOW() WHERE id = $2`

	userUpdateLastActionQuery = `UPDATE users SET last_action = NOW() WHERE id = $1`

	userExistsQuery = `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`

	userDeleteQuery     = `DELETE FROM users WHERE id = $1`
	userRoleDeleteQuery = `DELETE FROM user_roles WHERE user_id = $1`
	userRoleInsertQuery = `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`
)

type GormUserRepository struct{}

func NewUserRepository() user.Repository {
	return &GormUserRepository{}
}

func (g *GormUserRepository) GetPaginated(ctx context.Context, params *user.FindParams) ([]*user.User, error) {
	where, args := []string{"1 = 1"}, []interface{}{}

	query := repo.Join(
		userFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryUsers(ctx, query, args...)
}

func (g *GormUserRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (g *GormUserRepository) GetAll(ctx context.Context) ([]*user.User, error) {
	return g.GetPaginated(ctx, &user.FindParams{
		Limit: 100000,
	})
}

func (g *GormUserRepository) GetByID(ctx context.Context, id uint) (*user.User, error) {
	users, err := g.GetPaginated(ctx, &user.FindParams{ID: id})
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, ErrUserNotFound
	}
	return users[0], nil
}

func (g *GormUserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.email = $1", email)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, ErrUserNotFound
	}
	return users[0], nil
}

func (g *GormUserRepository) Create(ctx context.Context, data *user.User) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}

	dbUser, _ := toDBUser(data)

	err = tx.QueryRow(ctx, userInsertQuery,
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName,
		dbUser.Email,
		dbUser.Password,
		dbUser.UiLanguage,
		dbUser.AvatarID,
		dbUser.EmployeeID,
		dbUser.CreatedAt,
		dbUser.UpdatedAt,
	).Scan(&data.ID)

	if err != nil {
		return err
	}

	return g.updateUserRoles(ctx, data.ID, data.Roles)
}

func (g *GormUserRepository) CreateOrUpdate(ctx context.Context, data *user.User) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}

	// Check if the user exists
	var exists bool
	err = tx.QueryRow(ctx, userExistsQuery, data.ID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}

	if exists {
		// Update existing user
		err = g.Update(ctx, data)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
	} else {
		// Create new user
		err = g.Create(ctx, data)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil
}

func (g *GormUserRepository) Update(ctx context.Context, data *user.User) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}

	dbUser, _ := toDBUser(data)

	_, err = tx.Exec(ctx, userUpdateQuery,
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName,
		dbUser.Email,
		dbUser.Password,
		dbUser.UiLanguage,
		dbUser.AvatarID,
		dbUser.EmployeeID,
		dbUser.UpdatedAt,
		dbUser.ID,
	)

	if err != nil {
		return err
	}

	return g.updateUserRoles(ctx, data.ID, data.Roles)
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

func (g *GormUserRepository) queryUsers(ctx context.Context, query string, args ...interface{}) ([]*user.User, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userMap := make(map[uint]*user.User)
	roleMap := make(map[uint]*models.Role)

	for rows.Next() {
		var (
			u models.User
			a models.Upload

			middleName, lastIP, password sql.NullString
			avatarID, employeeID         sql.NullInt32
			lastLogin, lastAction        sql.NullTime

			// Upload fields
			avatarHash, avatarPath, avatarMimeType sql.NullString
			avatarSize                             sql.NullInt32
			avatarCreatedAt, avatarUpdatedAt       sql.NullTime

			// Role fields
			roleID                       sql.NullInt32
			roleName, roleDesc           sql.NullString
			roleCreatedAt, roleUpdatedAt sql.NullTime

			// Permission fields
			permID                                           sql.NullString
			permName, permResource, permAction, permModifier sql.NullString
		)

		if err := rows.Scan(
			&u.ID, &u.FirstName, &u.LastName, &middleName, &u.Email, &password,
			&u.UiLanguage, &avatarID, &lastLogin, &lastIP, &lastAction, &employeeID,
			&u.CreatedAt, &u.UpdatedAt,

			// Upload fields
			&avatarID, &avatarHash, &avatarPath, &avatarSize, &avatarMimeType,
			&avatarCreatedAt, &avatarUpdatedAt,

			// Role fields
			&roleID, &roleName, &roleDesc, &roleCreatedAt, &roleUpdatedAt,

			// Permission fields
			&permID, &permName, &permResource, &permAction, &permModifier,
		); err != nil {
			return nil, err
		}

		// Handle User nullables
		if middleName.Valid {
			u.MiddleName = mapping.Pointer(middleName.String)
		}
		if password.Valid {
			u.Password = mapping.Pointer(password.String)
		}
		if avatarID.Valid {
			u.AvatarID = mapping.Pointer(uint(avatarID.Int32))
		}
		if lastLogin.Valid {
			u.LastLogin = mapping.Pointer(lastLogin.Time)
		}
		if lastIP.Valid {
			u.LastIP = mapping.Pointer(lastIP.String)
		}
		if lastAction.Valid {
			u.LastAction = mapping.Pointer(lastAction.Time)
		}
		if employeeID.Valid {
			u.EmployeeID = mapping.Pointer(uint(employeeID.Int32))
		}

		// Get or create user
		domainUser, exists := userMap[u.ID]
		if !exists {
			var err error
			domainUser, err = ToDomainUser(&u)
			if err != nil {
				return nil, err
			}
			userMap[u.ID] = domainUser

			// Handle Avatar if exists
			if avatarID.Valid && avatarHash.Valid {
				a.ID = uint(avatarID.Int32)
				a.Hash = avatarHash.String
				a.Path = avatarPath.String
				a.Size = int(avatarSize.Int32)
				a.Mimetype = avatarMimeType.String
				a.CreatedAt = avatarCreatedAt.Time
				a.UpdatedAt = avatarUpdatedAt.Time
				domainUser.Avatar = ToDomainUpload(&a)
			}
		}

		// Handle Role and Permissions
		if roleID.Valid {
			r, exists := roleMap[uint(roleID.Int32)]
			if !exists {
				r = &models.Role{
					ID:          uint(roleID.Int32),
					Name:        roleName.String,
					Description: roleDesc.String,
					CreatedAt:   roleCreatedAt.Time,
					UpdatedAt:   roleUpdatedAt.Time,
					Permissions: make([]models.Permission, 0),
				}
				roleMap[r.ID] = r
				domainRole, err := toDomainRole(r)
				if err != nil {
					return nil, err
				}
				domainUser.Roles = append(domainUser.Roles, domainRole)
			}

			if permID.Valid {
				permUUID, err := uuid.Parse(permID.String)
				if err != nil {
					return nil, err
				}

				perm := models.Permission{
					ID:       permUUID,
					Name:     permName.String,
					Resource: permResource.String,
					Action:   permAction.String,
					Modifier: permModifier.String,
				}
				r.Permissions = append(r.Permissions, perm)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert map to slice
	users := make([]*user.User, 0, len(userMap))
	for _, u := range userMap {
		users = append(users, u)
	}

	return users, nil
}

func (g *GormUserRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}

func (g *GormUserRepository) updateUserRoles(ctx context.Context, userID uint, roles []*role.Role) error {
	// Delete existing roles
	if err := g.execQuery(ctx, userRoleDeleteQuery, userID); err != nil {
		return err
	}

	// Insert new roles
	for _, r := range roles {
		if err := g.execQuery(ctx, userRoleInsertQuery, userID, r.ID); err != nil {
			return err
		}
	}

	return nil
}
