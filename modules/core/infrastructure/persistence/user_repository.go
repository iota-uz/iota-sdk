package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
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
            u.phone,
            u.password,
            u.ui_language,
            u.avatar_id,
            u.last_login,
            u.last_ip,
            u.last_action,
            u.created_at,
            u.updated_at
        FROM users u`

	userCountQuery = `SELECT COUNT(u.id) FROM users u`

	userUpdateLastLoginQuery = `UPDATE users SET last_login = NOW() WHERE id = $1`

	userUpdateLastActionQuery = `UPDATE users SET last_action = NOW() WHERE id = $1`

	userDeleteQuery     = `DELETE FROM users WHERE id = $1`
	userRoleDeleteQuery = `DELETE FROM user_roles WHERE user_id = $1`
	userRoleInsertQuery = `INSERT INTO user_roles (user_id, role_id) VALUES`

	userGroupDeleteQuery = `DELETE FROM group_users WHERE user_id = $1`
	userGroupInsertQuery = `INSERT INTO group_users (user_id, group_id) VALUES`

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

	userGroupsQuery = `
				SELECT
					group_id
				FROM group_users 
				WHERE user_id = $1
			`
)

type PgUserRepository struct {
	uploadRepo upload.Repository
}

func NewUserRepository(uploadRepo upload.Repository) user.Repository {
	return &PgUserRepository{
		uploadRepo: uploadRepo,
	}
}

func BuildUserFilters(params *user.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	// Add join for role filter if needed
	if params.RoleID != nil {
		switch params.RoleID.Expr {
		case repo.Eq:
			where = append(where, fmt.Sprintf("ur.role_id = $%d", len(args)+1))
			args = append(args, params.RoleID.Value)
		case repo.NotEq:
			where = append(where, fmt.Sprintf("ur.role_id != $%d", len(args)+1))
			args = append(args, params.RoleID.Value)
		case repo.In:
			if values, ok := params.RoleID.Value.([]interface{}); ok && len(values) > 0 {
				where = append(where, fmt.Sprintf("ur.role_id = ANY($%d)", len(args)+1))
				args = append(args, values)
			} else {
				return nil, nil, errors.Wrap(fmt.Errorf("invalid value for role ID filter: %v", params.RoleID.Value), "invalid filter")
			}
		default:
			return nil, nil, errors.Wrap(fmt.Errorf("unsupported expression for role ID filter: %v", params.RoleID.Expr), "invalid filter")
		}
	}

	// Add join for group filter if needed
	if params.GroupID != nil {
		switch params.GroupID.Expr {
		case repo.Eq:
			where = append(where, fmt.Sprintf("gu.group_id = $%d", len(args)+1))
			args = append(args, params.GroupID.Value)
		case repo.NotEq:
			where = append(where, fmt.Sprintf("gu.group_id != $%d", len(args)+1))
			args = append(args, params.GroupID.Value)
		case repo.In:
			if values, ok := params.GroupID.Value.([]interface{}); ok && len(values) > 0 {
				where = append(where, fmt.Sprintf("gu.group_id = ANY($%d)", len(args)+1))
				args = append(args, values)
			} else {
				return nil, nil, errors.Wrap(fmt.Errorf("invalid value for group ID filter: %v", params.GroupID.Value), "invalid filter")
			}
		default:
			return nil, nil, errors.Wrap(fmt.Errorf("unsupported expression for group ID filter: %v", params.GroupID.Expr), "invalid filter")
		}
	}

	if params.PermissionID != nil {
		switch params.PermissionID.Expr {
		case repo.Eq:
			where = append(where, fmt.Sprintf("rp.permission_id = $%d", len(args)+1))
			args = append(args, params.PermissionID.Value)
		case repo.NotEq:
			where = append(where, fmt.Sprintf("rp.permission_id != $%d", len(args)+1))
			args = append(args, params.PermissionID.Value)
		case repo.In:
			if values, ok := params.PermissionID.Value.([]interface{}); ok && len(values) > 0 {
				where = append(where, fmt.Sprintf("rp.permission_id = ANY($%d)", len(args)+1))
				args = append(args, values)
			} else {
				return nil, nil, errors.Wrap(fmt.Errorf("invalid value for permission ID filter: %v", params.PermissionID.Value), "invalid filter")
			}
		case repo.NotIn:
			if values, ok := params.PermissionID.Value.([]interface{}); ok && len(values) > 0 {
				where = append(where, fmt.Sprintf("rp.permission_id != ALL($%d)", len(args)+1))
				args = append(args, values)
			} else {
				return nil, nil, errors.Wrap(fmt.Errorf("invalid value for permission ID filter: %v", params.PermissionID.Value), "invalid filter")
			}
		}
	}

	if params.CreatedAt != nil {
		switch params.CreatedAt.Expr {
		case repo.Gt:
			where = append(where, fmt.Sprintf("u.created_at > $%d", len(args)+1))
		case repo.Gte:
			where = append(where, fmt.Sprintf("u.created_at >= $%d", len(args)+1))
		case repo.Lt:
			where = append(where, fmt.Sprintf("u.created_at < $%d", len(args)+1))
		case repo.Lte:
			where = append(where, fmt.Sprintf("u.created_at <= $%d", len(args)+1))
		default:
			return nil, nil, errors.Wrap(fmt.Errorf("unsupported expression for created at filter: %v", params.CreatedAt.Expr), "invalid filter")
		}

		args = append(args, params.CreatedAt.Value)
	}

	if params.Email != nil {
		switch params.Email.Expr {
		case repo.Eq:
			where = append(where, fmt.Sprintf("u.email = $%d", len(args)+1))
			args = append(args, params.Email.Value)
		case repo.NotEq:
			where = append(where, fmt.Sprintf("u.email != $%d", len(args)+1))
			args = append(args, params.Email.Value)
		case repo.Like:
			where = append(where, fmt.Sprintf("u.email ILIKE $%d", len(args)+1))
			args = append(args, params.Email.Value)
		case repo.In:
			if values, ok := params.Email.Value.([]interface{}); ok && len(values) > 0 {
				where = append(where, fmt.Sprintf("u.email = ANY($%d)", len(args)+1))
				args = append(args, values)
			} else {
				return nil, nil, errors.Wrap(fmt.Errorf("invalid value for email filter: %v", params.Email.Value), "invalid filter")
			}
		default:
			return nil, nil, errors.Wrap(fmt.Errorf("unsupported expression for email filter: %v", params.Email.Expr), "invalid filter")
		}
	}

	if params.LastLogin != nil {
		switch params.LastLogin.Expr {
		case repo.Gt:
			where = append(where, fmt.Sprintf("u.last_login > $%d", len(args)+1))
		case repo.Gte:
			where = append(where, fmt.Sprintf("u.last_login >= $%d", len(args)+1))
		case repo.Lt:
			where = append(where, fmt.Sprintf("u.last_login < $%d", len(args)+1))
		case repo.Lte:
			where = append(where, fmt.Sprintf("u.last_login <= $%d", len(args)+1))
		default:
			return nil, nil, errors.Wrap(fmt.Errorf("unsupported expression for last login filter: %v", params.LastLogin.Expr), "invalid filter")
		}
		args = append(args, params.LastLogin.Value)
	}

	if params.Name != "" {
		index := len(args) + 1
		where = append(
			where,
			fmt.Sprintf(
				"(u.first_name ILIKE $%d OR u.last_name ILIKE $%d OR u.middle_name ILIKE $%d)",
				index,
				index,
				index,
			),
		)
		args = append(args, "%"+params.Name+"%")
	}

	return where, args, nil
}

func (g *PgUserRepository) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error) {
	fieldMap := map[user.Field]string{
		user.FirstName:  "u.first_name",
		user.LastName:   "u.last_name",
		user.MiddleName: "u.middle_name",
		user.Email:      "u.email",
		user.LastLogin:  "u.last_login",
		user.CreatedAt:  "u.created_at",
		user.UpdatedAt:  "u.updated_at",
	}

	sortFields := make([]string, 0, len(params.SortBy.Fields))

	for _, f := range params.SortBy.Fields {
		if field, ok := fieldMap[f]; ok {
			sortFields = append(sortFields, field)
		} else {
			return nil, errors.Wrap(fmt.Errorf("unknown sort field: %v", f), "invalid pagination parameters")
		}
	}

	where, args, err := BuildUserFilters(params)
	if err != nil {
		return nil, err
	}

	baseQuery := userFindQuery
	if params.RoleID != nil || params.PermissionID != nil {
		baseQuery += " JOIN user_roles ur ON u.id = ur.user_id"
	}

	if params.GroupID != nil {
		baseQuery += " JOIN group_users gu ON u.id = gu.user_id"
	}

	if params.PermissionID != nil {
		baseQuery += " JOIN role_permissions rp ON ur.role_id = rp.role_id"
	}

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

func (g *PgUserRepository) Count(ctx context.Context, params *user.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	where, args, err := BuildUserFilters(params)
	if err != nil {
		return 0, err
	}

	baseQuery := userCountQuery
	if params.RoleID != nil || params.PermissionID != nil {
		baseQuery += " JOIN user_roles ur ON u.id = ur.user_id"
	}

	if params.GroupID != nil {
		baseQuery += " JOIN group_users gu ON u.id = gu.user_id"
	}

	if params.PermissionID != nil {
		baseQuery += " JOIN role_permissions rp ON ur.role_id = rp.role_id"
	}

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count users")
	}
	return count, nil
}

func (g *PgUserRepository) GetAll(ctx context.Context) ([]user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all users")
	}
	return users, nil
}

func (g *PgUserRepository) GetByID(ctx context.Context, id uint) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.id = $1", id)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user with id: %d", id))
	}
	if len(users) == 0 {
		return nil, errors.Wrap(ErrUserNotFound, fmt.Sprintf("id: %d", id))
	}
	return users[0], nil
}

func (g *PgUserRepository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.email = $1", email)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user with email: %s", email))
	}
	if len(users) == 0 {
		return nil, errors.Wrap(ErrUserNotFound, fmt.Sprintf("email: %s", email))
	}
	return users[0], nil
}

func (g *PgUserRepository) GetByPhone(ctx context.Context, phone string) (user.User, error) {
	users, err := g.queryUsers(ctx, userFindQuery+" WHERE u.phone = $1", phone)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user with phone: %s", phone))
	}
	if len(users) == 0 {
		return nil, errors.Wrap(ErrUserNotFound, fmt.Sprintf("phone: %s", phone))
	}
	return users[0], nil
}

func (g *PgUserRepository) Create(ctx context.Context, data user.User) (user.User, error) {
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
		"phone",
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
		dbUser.Phone,
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

	if err := g.updateUserGroups(ctx, dbUser.ID, data.GroupIDs()); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update group IDs for user ID: %d", dbUser.ID))
	}

	return g.GetByID(ctx, dbUser.ID)
}

func (g *PgUserRepository) Update(ctx context.Context, data user.User) error {
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
		"phone",
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
		dbUser.Phone,
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

	if err := g.updateUserGroups(ctx, data.ID(), data.GroupIDs()); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update group IDs for user ID: %d", data.ID()))
	}

	return nil
}

func (g *PgUserRepository) UpdateLastLogin(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userUpdateLastLoginQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update last login for user ID: %d", id))
	}
	return nil
}

func (g *PgUserRepository) UpdateLastAction(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userUpdateLastActionQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to update last action for user ID: %d", id))
	}
	return nil
}

func (g *PgUserRepository) Delete(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, userRoleDeleteQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete roles for user ID: %d", id))
	}
	if err := g.execQuery(ctx, userGroupDeleteQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete groups for user ID: %d", id))
	}
	if err := g.execQuery(ctx, userDeleteQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete user with ID: %d", id))
	}
	return nil
}

func (g *PgUserRepository) queryUsers(ctx context.Context, query string, args ...interface{}) ([]user.User, error) {
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
	for rows.Next() {
		var u models.User

		if err := rows.Scan(
			&u.ID,
			&u.FirstName,
			&u.LastName,
			&u.MiddleName,
			&u.Email,
			&u.Phone,
			&u.Password,
			&u.UILanguage,
			&u.AvatarID,
			&u.LastLogin,
			&u.LastIP,
			&u.LastAction,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan user row")
		}
		users = append(users, &u)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	entities := make([]user.User, 0, len(users))
	for _, u := range users {
		roles, err := g.userRoles(ctx, u.ID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get roles for user ID: %d", u.ID))
		}

		groupIDs, err := g.userGroupIDs(ctx, u.ID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get group IDs for user ID: %d", u.ID))
		}

		var avatar upload.Upload
		if u.AvatarID.Valid {
			avatar, err = g.uploadRepo.GetByID(ctx, uint(u.AvatarID.Int32))
			if err != nil && !errors.Is(err, ErrUploadNotFound) {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to get avatar for user ID: %d", u.ID))
			}
		}

		var domainUser user.User
		if avatar != nil {
			domainUser, err = ToDomainUser(u, ToDBUpload(avatar), roles, groupIDs)
		} else {
			domainUser, err = ToDomainUser(u, nil, roles, groupIDs)
		}
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert user ID: %d to domain entity", u.ID))
		}
		entities = append(entities, domainUser)
	}

	return entities, nil
}

func (g *PgUserRepository) rolePermissions(ctx context.Context, roleID uint) ([]*models.Permission, error) {
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

func (g *PgUserRepository) userRoles(ctx context.Context, userID uint) ([]role.Role, error) {
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

func (g *PgUserRepository) userGroupIDs(ctx context.Context, userID uint) ([]uuid.UUID, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, userGroupsQuery, userID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query group IDs for user ID: %d", userID))
	}
	defer rows.Close()

	var groupIDs []uuid.UUID
	for rows.Next() {
		var groupIDStr string
		if err := rows.Scan(&groupIDStr); err != nil {
			return nil, errors.Wrap(err, "failed to scan group ID")
		}

		groupID, err := uuid.Parse(groupIDStr)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to parse group ID: %s", groupIDStr))
		}

		groupIDs = append(groupIDs, groupID)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	return groupIDs, nil
}

func (g *PgUserRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
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

func (g *PgUserRepository) updateUserRoles(ctx context.Context, userID uint, roles []role.Role) error {
	if len(roles) == 0 {
		return nil
	}

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

func (g *PgUserRepository) updateUserGroups(ctx context.Context, userID uint, groupIDs []uuid.UUID) error {
	if err := g.execQuery(ctx, userGroupDeleteQuery, userID); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete existing groups for user ID: %d", userID))
	}

	if len(groupIDs) == 0 {
		return nil
	}

	values := make([][]interface{}, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		values = append(values, []interface{}{userID, groupID.String()})
	}
	q, args := repo.BatchInsertQueryN(userGroupInsertQuery, values)
	if err := g.execQuery(ctx, q, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to insert groups for user ID: %d", userID))
	}
	return nil
}
