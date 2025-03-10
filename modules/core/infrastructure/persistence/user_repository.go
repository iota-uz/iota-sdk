package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-faster/errors"
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

	userCountQuery = `SELECT COUNT(id) FROM users`

	userUpdateLastLoginQuery = `UPDATE users SET last_login = NOW() WHERE id = $1`

	userUpdateLastActionQuery = `UPDATE users SET last_action = NOW() WHERE id = $1`

	userDeleteQuery     = `DELETE FROM users WHERE id = $1`
	userRoleDeleteQuery = `DELETE FROM user_roles WHERE user_id = $1`
	userRoleInsertQuery = `INSERT INTO user_roles (user_id, role_id) VALUES`

	userRolePermissionsQuery = `
			SELECT p.id, p.name, p.resource, p.action, p.modifier, p.description
			FROM role_permissions rp LEFT JOIN permissions p ON rp.permission_id = p.id WHERE role_id = $1`

	userRolesQuery = `
			SELECT
				r.id,
				r.name,
				r.description,
				r.created_at,
				r.updated_at
			FROM user_roles ur LEFT JOIN roles r ON ur.role_id = r.id WHERE ur.user_id = $1
		`
)

type GormUserRepository struct{}

func NewUserRepository() user.Repository {
	return &GormUserRepository{}
}

// buildFilters creates the where clauses and arguments for filtering user queries
func (g *GormUserRepository) buildFilters(params *user.FindParams) (baseQuery string, where []string, args []interface{}) {
	where = []string{"1 = 1"}
	args = []interface{}{}
	
	// Start with the appropriate base query
	baseQuery = userFindQuery
	
	// Add join for role filter if needed
	if params != nil && params.RoleID > 0 {
		// For Count method we use different base query, so we need to ensure the join part is always added
		if !strings.Contains(baseQuery, "JOIN user_roles ur") {
			baseQuery += " JOIN user_roles ur ON u.id = ur.user_id"
		}
		where = append(where, "ur.role_id = $"+fmt.Sprintf("%d", len(args)+1))
		args = append(args, params.RoleID)
	}

	if params != nil && params.Name != "" {
		where = append(where, "(u.first_name ILIKE $"+fmt.Sprintf("%d", len(args)+1)+" OR u.last_name ILIKE $"+fmt.Sprintf("%d", len(args)+1)+" OR u.middle_name ILIKE $"+fmt.Sprintf("%d", len(args)+1)+")")
		args = append(args, "%"+params.Name+"%")
	}
	
	return baseQuery, where, args
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
			return nil, errors.Wrap(fmt.Errorf("unknown sort field: %v", f), "invalid pagination parameters")
		}
	}
	
	baseQuery, where, args := g.buildFilters(params)

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
		repo.OrderBy(sortFields, params.SortBy.Ascending),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	users, err := g.queryUsers(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get paginated users")
	}
	return users, nil
}

func (g *GormUserRepository) Count(ctx context.Context, params *user.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	// Define a base query for counting users
	countBaseQuery := "SELECT COUNT(u.id) FROM users u"
	
	// If we need to filter by role, add the join to the base query
	if params != nil && params.RoleID > 0 {
		countBaseQuery += " JOIN user_roles ur ON u.id = ur.user_id"
	}
	
	// Get the where clauses and args, but ignore the baseQuery from buildFilters
	_, where, args := g.buildFilters(params)
	
	query := repo.Join(
		countBaseQuery,
		repo.JoinWhere(where...),
	)
	
	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count users")
	}
	return count, nil
}

func (g *GormUserRepository) GetAll(ctx context.Context) ([]user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all users")
	}
	return users, nil
}

func (g *GormUserRepository) GetByID(ctx context.Context, id uint) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.id = $1", id)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user with id: %d", id))
	}
	if len(users) == 0 {
		return nil, errors.Wrap(ErrUserNotFound, fmt.Sprintf("id: %d", id))
	}
	return users[0], nil
}

func (g *GormUserRepository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.email = $1", email)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user with email: %s", email))
	}
	if len(users) == 0 {
		return nil, errors.Wrap(ErrUserNotFound, fmt.Sprintf("email: %s", email))
	}
	return users[0], nil
}

func (g *GormUserRepository) Create(ctx context.Context, data user.User) (user.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbUser, _ := toDBUser(data)

	fields := []string{
		"first_name",
		"last_name",
		"middle_name",
		"email",
		"password",
		"ui_language",
		"avatar_id",
		"created_at",
		"updated_at",
	}

	values := []interface{}{
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName,
		dbUser.Email,
		dbUser.Password,
		dbUser.UILanguage,
		dbUser.AvatarID,
		dbUser.CreatedAt,
		dbUser.UpdatedAt,
	}

	if efs, ok := data.(repo.ExtendedFieldSet); ok {
		fields = append(fields, efs.Fields()...)
		for _, f := range efs.Fields() {
			values = append(values, efs.Value(f))
		}
	}

	err = tx.QueryRow(ctx, repo.Insert("users", fields, "id"), values...).Scan(&dbUser.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert user")
	}
	if err := g.updateUserRoles(ctx, dbUser.ID, data.Roles()); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update roles for user ID: %d", dbUser.ID))
	}
	return g.GetByID(ctx, dbUser.ID)
}

func (g *GormUserRepository) Update(ctx context.Context, data user.User) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	dbUser, _ := toDBUser(data)

	fields := []string{
		"first_name",
		"last_name",
		"middle_name",
		"email",
		"password",
		"ui_language",
		"avatar_id",
		"updated_at",
	}

	values := []interface{}{
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName,
		dbUser.Email,
		dbUser.Password,
		dbUser.UILanguage,
		dbUser.AvatarID,
		dbUser.UpdatedAt,
	}

	if efs, ok := data.(repo.ExtendedFieldSet); ok {
		fields = append(fields, efs.Fields()...)
		for _, f := range efs.Fields() {
			values = append(values, efs.Value(f))
		}
	}

	values = append(values, dbUser.ID)

	_, err = tx.Exec(ctx, repo.Update("users", fields, fmt.Sprintf("id = $%d", len(values))), values...)

	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update user with ID: %d", dbUser.ID))
	}

	if err := g.updateUserRoles(ctx, data.ID(), data.Roles()); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update roles for user ID: %d", data.ID()))
	}

	return nil
}

func (g *GormUserRepository) UpdateLastLogin(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userUpdateLastLoginQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update last login for user ID: %d", id))
	}
	return nil
}

func (g *GormUserRepository) UpdateLastAction(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userUpdateLastActionQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update last action for user ID: %d", id))
	}
	return nil
}

func (g *GormUserRepository) Delete(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userRoleDeleteQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete roles for user ID: %d", id))
	}
	if err := g.execQuery(ctx, userDeleteQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete user with ID: %d", id))
	}
	return nil
}

func (g *GormUserRepository) queryUsers(ctx context.Context, query string, args ...interface{}) ([]user.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
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
			return nil, errors.Wrap(err, "failed to scan user row")
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
		return nil, errors.Wrap(err, "row iteration error")
	}

	uploadMap := make(map[uint]*models.Upload)
	for _, u := range uploads {
		uploadMap[u.ID] = u
	}

	entities := make([]user.User, 0, len(users))
	for _, u := range users {
		roles, err := g.userRoles(ctx, u.ID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get roles for user ID: %d", u.ID))
		}
		avatar, _ := uploadMap[uint(u.AvatarID.Int32)]
		entity, err := ToDomainUser(u, avatar, roles)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert user ID: %d to domain entity", u.ID))
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func (g *GormUserRepository) rolePermissions(ctx context.Context, roleID uint) ([]*models.Permission, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, userRolePermissionsQuery, roleID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query permissions for role ID: %d", roleID))
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
			return nil, errors.Wrap(err, "failed to scan permission row")
		}
		permissions = append(permissions, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	return permissions, nil
}

func (g *GormUserRepository) userRoles(ctx context.Context, userID uint) ([]role.Role, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, userRolesQuery, userID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query roles for user ID: %d", userID))
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
			return nil, errors.Wrap(err, "failed to scan role row")
		}
		roles = append(roles, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	entities := make([]role.Role, 0, len(roles))
	for _, r := range roles {
		permissions, err := g.rolePermissions(ctx, r.ID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get permissions for role ID: %d", r.ID))
		}
		entity, err := toDomainRole(r, permissions)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert role ID: %d to domain entity", r.ID))
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func (g *GormUserRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}
	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to execute query")
	}
	return nil
}

func (g *GormUserRepository) updateUserRoles(ctx context.Context, userID uint, roles []role.Role) error {
	if err := g.execQuery(ctx, userRoleDeleteQuery, userID); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete existing roles for user ID: %d", userID))
	}

	if len(roles) == 0 {
		return nil
	}

	values := make([][]interface{}, 0, len(roles)*2)
	for _, r := range roles {
		values = append(values, []interface{}{userID, r.ID()})
	}
	q, args := repo.BatchInsertQueryN(userRoleInsertQuery, values)
	if err := g.execQuery(ctx, q, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to insert roles for user ID: %d", userID))
	}
	return nil
}
