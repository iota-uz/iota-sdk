package query

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

// SQL queries
const (
	selectUsersSQL = `SELECT DISTINCT
		u.id, u.tenant_id, u.type, u.first_name, u.last_name, u.middle_name,
		u.email, u.phone, u.ui_language, u.avatar_id, u.last_login, u.last_action,
		u.created_at, u.updated_at
	FROM users u`

	selectUserByIDSQL = `SELECT
		u.id, u.tenant_id, u.type, u.first_name, u.last_name, u.middle_name,
		u.email, u.phone, u.ui_language, u.avatar_id, u.last_login, u.last_action,
		u.created_at, u.updated_at
	FROM users u
	WHERE u.id = $1`

	selectUploadByIDSQL = `SELECT
		id, tenant_id, hash, path, name, size, mimetype, type, created_at, updated_at
	FROM uploads
	WHERE id = $1`

	selectUserRolesSQL = `SELECT r.id, r.type, r.name, r.description, r.created_at, r.updated_at
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1`

	selectUserPermissionsSQL = `SELECT DISTINCT p.id, p.name, p.resource, p.action, p.modifier
		FROM permissions p
		WHERE p.id IN (
			SELECT permission_id FROM user_permissions WHERE user_id = $1
			UNION
			SELECT rp.permission_id FROM role_permissions rp
			JOIN user_roles ur ON rp.role_id = ur.role_id
			WHERE ur.user_id = $1
		)`

	selectUserGroupsSQL = `SELECT group_id FROM group_users WHERE user_id = $1`
)

type Field = string

// Field constants for sorting and filtering
const (
	FieldID        Field = "id"
	FieldFirstName Field = "first_name"
	FieldLastName  Field = "last_name"
	FieldEmail     Field = "email"
	FieldPhone     Field = "phone"
	FieldType      Field = "type"
	FieldCreatedAt Field = "created_at"
	FieldUpdatedAt Field = "updated_at"
	FieldTenantID  Field = "tenant_id"
	FieldGroupID   Field = "group_id"
)

type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Limit   int
	Offset  int
	SortBy  SortBy
	Search  string
	Filters []Filter
}

type UserQueryRepository interface {
	FindUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error)
	FindUserByID(ctx context.Context, userID int) (*viewmodels.User, error)
	SearchUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error)
	FindUsersWithRoles(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error)
}

type pgUserQueryRepository struct{}

func NewPgUserQueryRepository() UserQueryRepository {
	return &pgUserQueryRepository{}
}

func (r *pgUserQueryRepository) fieldMapping() map[Field]string {
	return map[Field]string{
		FieldID:        "u.id",
		FieldFirstName: "u.first_name",
		FieldLastName:  "u.last_name",
		FieldEmail:     "u.email",
		FieldPhone:     "u.phone",
		FieldType:      "u.type",
		FieldCreatedAt: "u.created_at",
		FieldUpdatedAt: "u.updated_at",
		FieldTenantID:  "u.tenant_id",
		FieldGroupID:   "gu.group_id",
	}
}

func (r *pgUserQueryRepository) filtersToSQL(filters []Filter) ([]string, []interface{}) {
	if len(filters) == 0 {
		return []string{}, []interface{}{}
	}

	var conditions []string
	var args []interface{}

	for _, f := range filters {
		fieldName := r.fieldMapping()[f.Column]
		if fieldName == "" {
			continue
		}
		condition := f.Filter.String(fieldName, len(args)+1)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, f.Filter.Value()...)
		}
	}

	return conditions, args
}

func (r *pgUserQueryRepository) FindUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Check if we need to filter by groups
	hasGroupFilter := false
	var groupFilterIndex int
	for i, f := range params.Filters {
		if f.Column == "group_id" {
			hasGroupFilter = true
			groupFilterIndex = i
			break
		}
	}

	// Build conditions and args, excluding group_id since it needs special handling
	var filteredFilters []Filter
	for _, f := range params.Filters {
		if f.Column != "group_id" {
			filteredFilters = append(filteredFilters, f)
		}
	}
	conditions, args := r.filtersToSQL(filteredFilters)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add group filter if specified
	joinClause := ""
	if hasGroupFilter {
		joinClause = " JOIN group_users gu ON u.id = gu.user_id"

		// Get the group filter values
		groupFilter := params.Filters[groupFilterIndex].Filter
		groupValues := groupFilter.Value()

		// Convert string group IDs to UUID format and add to args
		placeholders := make([]string, len(groupValues))
		for i, val := range groupValues {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
			// Convert string group ID to UUID format
			groupIDStr, ok := val.(string)
			if !ok {
				return nil, 0, errors.New("group ID must be a string")
			}
			groupUUID, err := uuid.Parse(groupIDStr)
			if err != nil {
				return nil, 0, errors.Wrapf(err, "invalid group ID: %s", groupIDStr)
			}
			args = append(args, groupUUID)
		}
		groupCondition := fmt.Sprintf("gu.group_id IN (%s)", strings.Join(placeholders, ", "))

		if whereClause == "" {
			whereClause = " WHERE " + groupCondition
		} else {
			whereClause += " AND " + groupCondition
		}
	}

	orderBy := params.SortBy.ToSQL(r.fieldMapping())

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT u.id) FROM users u%s%s", joinClause, whereClause)

	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users")
	}

	// Main query - need to modify selectUsersSQL to include join
	selectQuery := selectUsersSQL
	if hasGroupFilter {
		// Replace "FROM users u" with "FROM users u JOIN group_users gu ON u.id = gu.user_id"
		selectQuery = strings.Replace(selectUsersSQL, "FROM users u", fmt.Sprintf("FROM users u%s", joinClause), 1)
	}

	queryParts := []string{selectQuery}
	if whereClause != "" {
		queryParts = append(queryParts, whereClause)
	}
	if orderBy != "" {
		queryParts = append(queryParts, orderBy)
	}
	if limitOffset := repo.FormatLimitOffset(params.Limit, params.Offset); limitOffset != "" {
		queryParts = append(queryParts, limitOffset)
	}
	query := strings.Join(queryParts, " ")

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to find users")
	}
	defer rows.Close()

	users := make([]*viewmodels.User, 0)
	for rows.Next() {
		var dbUser models.User
		err := rows.Scan(
			&dbUser.ID, &dbUser.TenantID, &dbUser.Type, &dbUser.FirstName, &dbUser.LastName, &dbUser.MiddleName,
			&dbUser.Email, &dbUser.Phone, &dbUser.UILanguage, &dbUser.AvatarID, &dbUser.LastLogin, &dbUser.LastAction,
			&dbUser.CreatedAt, &dbUser.UpdatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan user")
		}

		// Load avatar if exists
		var avatar *models.Upload
		if dbUser.AvatarID.Valid {
			avatar, err = r.loadUploadByID(ctx, int(dbUser.AvatarID.Int32))
			if err != nil {
				// Log error but don't fail the query
				avatar = nil
			}
		}

		user := mapToUserViewModel(dbUser, avatar != nil, avatar)

		// Load roles and permissions separately
		if err := r.loadUserRolesAndPermissions(ctx, &user); err != nil {
			return nil, 0, errors.Wrap(err, "failed to load roles and permissions")
		}

		users = append(users, &user)
	}

	return users, count, nil
}

func (r *pgUserQueryRepository) FindUserByID(ctx context.Context, userID int) (*viewmodels.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	query := selectUserByIDSQL
	args := []interface{}{userID}

	var dbUser models.User
	err = tx.QueryRow(ctx, query, args...).Scan(
		&dbUser.ID, &dbUser.TenantID, &dbUser.Type, &dbUser.FirstName, &dbUser.LastName, &dbUser.MiddleName,
		&dbUser.Email, &dbUser.Phone, &dbUser.UILanguage, &dbUser.AvatarID, &dbUser.LastLogin, &dbUser.LastAction,
		&dbUser.CreatedAt, &dbUser.UpdatedAt,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user by id")
	}

	// Load avatar if exists
	var avatar *models.Upload
	if dbUser.AvatarID.Valid {
		avatar, err = r.loadUploadByID(ctx, int(dbUser.AvatarID.Int32))
		if err != nil {
			// Log error but don't fail the query
			avatar = nil
		}
	}

	user := mapToUserViewModel(dbUser, avatar != nil, avatar)

	// Load roles and permissions
	if err := r.loadUserRolesAndPermissions(ctx, &user); err != nil {
		return nil, errors.Wrap(err, "failed to load roles and permissions")
	}

	return &user, nil
}

func (r *pgUserQueryRepository) SearchUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error) {
	if params.Search == "" {
		return r.FindUsers(ctx, params)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	searchQuery := strings.TrimSpace(params.Search)
	baseQuery := selectUsersSQL + ` WHERE (
		u.email ILIKE $1 OR
		u.first_name ILIKE $1 OR
		u.last_name ILIKE $1 OR
		CONCAT(u.first_name, ' ', u.last_name) ILIKE $1
	)`

	args := []interface{}{"%" + searchQuery + "%"}
	argIndex := 2

	// Add additional filters if any
	conditions, filterArgs := r.filtersToSQL(params.Filters)
	if len(conditions) > 0 {
		// Update arg positions in conditions
		for i, cond := range conditions {
			for j := 1; j <= len(filterArgs); j++ {
				oldPlaceholder := fmt.Sprintf("$%d", j)
				newPlaceholder := fmt.Sprintf("$%d", argIndex)
				conditions[i] = strings.Replace(cond, oldPlaceholder, newPlaceholder, 1)
				if strings.Contains(conditions[i], newPlaceholder) {
					argIndex++
				}
			}
		}
		baseQuery += " AND " + strings.Join(conditions, " AND ")
		args = append(args, filterArgs...)
	}

	// Count query - extract the WHERE clause from baseQuery
	whereClauseStart := strings.Index(baseQuery, "WHERE")
	countQuery := "SELECT COUNT(DISTINCT u.id) FROM users u " + baseQuery[whereClauseStart:]

	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users")
	}

	orderBy := params.SortBy.ToSQL(r.fieldMapping())

	queryParts := []string{baseQuery}
	if orderBy != "" {
		queryParts = append(queryParts, orderBy)
	}
	if limitOffset := repo.FormatLimitOffset(params.Limit, params.Offset); limitOffset != "" {
		queryParts = append(queryParts, limitOffset)
	}
	query := strings.Join(queryParts, " ")

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to search users")
	}
	defer rows.Close()

	users := make([]*viewmodels.User, 0)
	for rows.Next() {
		var dbUser models.User
		err := rows.Scan(
			&dbUser.ID, &dbUser.TenantID, &dbUser.Type, &dbUser.FirstName, &dbUser.LastName, &dbUser.MiddleName,
			&dbUser.Email, &dbUser.Phone, &dbUser.UILanguage, &dbUser.AvatarID, &dbUser.LastLogin, &dbUser.LastAction,
			&dbUser.CreatedAt, &dbUser.UpdatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan user")
		}

		// Load avatar if exists
		var avatar *models.Upload
		if dbUser.AvatarID.Valid {
			avatar, err = r.loadUploadByID(ctx, int(dbUser.AvatarID.Int32))
			if err != nil {
				// Log error but don't fail the query
				avatar = nil
			}
		}

		user := mapToUserViewModel(dbUser, avatar != nil, avatar)

		// Load roles and permissions
		if err := r.loadUserRolesAndPermissions(ctx, &user); err != nil {
			return nil, 0, errors.Wrap(err, "failed to load roles and permissions")
		}

		users = append(users, &user)
	}

	return users, count, nil
}

func (r *pgUserQueryRepository) FindUsersWithRoles(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error) {
	// This is the same as FindUsers since we load roles for all users anyway
	return r.FindUsers(ctx, params)
}

func (r *pgUserQueryRepository) loadUploadByID(ctx context.Context, uploadID int) (*models.Upload, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	var upload models.Upload
	err = tx.QueryRow(ctx, selectUploadByIDSQL, uploadID).Scan(
		&upload.ID, &upload.TenantID, &upload.Hash, &upload.Path, &upload.Name,
		&upload.Size, &upload.Mimetype, &upload.Type, &upload.CreatedAt, &upload.UpdatedAt,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load upload")
	}

	return &upload, nil
}

func (r *pgUserQueryRepository) loadUserRolesAndPermissions(ctx context.Context, user *viewmodels.User) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	userID, err := strconv.Atoi(user.ID)
	if err != nil {
		return errors.Wrap(err, "invalid user ID")
	}

	// Load roles
	rows, err := tx.Query(ctx, selectUserRolesSQL, userID)
	if err != nil {
		return errors.Wrap(err, "failed to query roles")
	}
	defer rows.Close()

	user.Roles = make([]*viewmodels.Role, 0)
	for rows.Next() {
		var role models.Role
		err := rows.Scan(&role.ID, &role.Type, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			return errors.Wrap(err, "failed to scan role")
		}

		user.Roles = append(user.Roles, mapToRoleViewModel(role))
	}

	// Load permissions (both direct and through roles)
	permRows, err := tx.Query(ctx, selectUserPermissionsSQL, userID)
	if err != nil {
		return errors.Wrap(err, "failed to query permissions")
	}
	defer permRows.Close()

	user.Permissions = make([]*viewmodels.Permission, 0)
	for permRows.Next() {
		var perm models.Permission
		err := permRows.Scan(&perm.ID, &perm.Name, &perm.Resource, &perm.Action, &perm.Modifier)
		if err != nil {
			return errors.Wrap(err, "failed to scan permission")
		}

		user.Permissions = append(user.Permissions, mapToPermissionViewModel(perm))
	}

	// Load group IDs
	groupRows, err := tx.Query(ctx, selectUserGroupsSQL, userID)
	if err != nil {
		return errors.Wrap(err, "failed to query groups")
	}
	defer groupRows.Close()

	user.GroupIDs = make([]string, 0)
	for groupRows.Next() {
		var groupID string
		if err := groupRows.Scan(&groupID); err != nil {
			return errors.Wrap(err, "failed to scan group ID")
		}
		user.GroupIDs = append(user.GroupIDs, groupID)
	}

	return nil
}
