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
	WHERE u.id = $1 AND u.tenant_id = $2`

	selectUploadByIDSQL = `SELECT
		id, tenant_id, hash, path, name, size, mimetype, type, created_at, updated_at
	FROM uploads
	WHERE id = $1 AND tenant_id = $2`

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

func (r *pgUserQueryRepository) buildFilterConditions(filters []Filter) ([]string, []interface{}) {
	return r.buildFilterConditionsWithStartIndex(filters, 1)
}

func (r *pgUserQueryRepository) buildFilterConditionsWithStartIndex(filters []Filter, startIndex int) ([]string, []interface{}) {
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
		condition := f.Filter.String(fieldName, startIndex+len(args))
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, f.Filter.Value()...)
		}
	}

	return conditions, args
}

// buildGroupFilterCondition builds the SQL condition for group filters
func (r *pgUserQueryRepository) buildGroupFilterCondition(filter *Filter, startIndex int) (string, []interface{}) {
	groupValues := filter.Filter.Value()
	if len(groupValues) == 0 {
		return "", nil
	}

	placeholders := make([]string, len(groupValues))
	args := make([]interface{}, 0, len(groupValues))

	for i, val := range groupValues {
		placeholders[i] = fmt.Sprintf("$%d", startIndex+i)
		// Convert string group ID to UUID format
		groupIDStr, ok := val.(string)
		if !ok {
			continue // Skip invalid values
		}
		groupUUID, err := uuid.Parse(groupIDStr)
		if err != nil {
			continue // Skip invalid UUIDs
		}
		args = append(args, groupUUID)
	}

	if len(args) == 0 {
		return "", nil
	}

	return fmt.Sprintf("gu.group_id IN (%s)", strings.Join(placeholders[:len(args)], ", ")), args
}

func (r *pgUserQueryRepository) FindUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get tenant ID")
	}

	// Separate group filters from regular filters
	var regularFilters []Filter
	var groupFilter *Filter

	for _, f := range params.Filters {
		if f.Column == FieldGroupID {
			groupFilter = &f
		} else {
			regularFilters = append(regularFilters, f)
		}
	}

	// Build conditions and args, starting with tenant filter
	conditions := []string{"u.tenant_id = $1"}
	args := []interface{}{tenantID}

	// Add regular filter conditions
	if len(regularFilters) > 0 {
		filterConditions, filterArgs := r.buildFilterConditionsWithStartIndex(regularFilters, len(args)+1)
		conditions = append(conditions, filterConditions...)
		args = append(args, filterArgs...)
	}

	// Handle group filter specially
	joinClause := ""
	if groupFilter != nil {
		joinClause = " JOIN group_users gu ON u.id = gu.user_id"
		groupCondition, groupArgs := r.buildGroupFilterCondition(groupFilter, len(args)+1)
		if groupCondition != "" {
			conditions = append(conditions, groupCondition)
			args = append(args, groupArgs...)
		}
	}

	whereClause := repo.JoinWhere(conditions...)

	// Count query
	countQuery := repo.Join("SELECT COUNT(DISTINCT u.id) FROM users u"+joinClause, whereClause)
	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users")
	}

	// Build main query
	selectQuery := selectUsersSQL
	if joinClause != "" {
		selectQuery = strings.Replace(selectUsersSQL, "FROM users u", "FROM users u"+joinClause, 1)
	}

	query := repo.Join(
		selectQuery,
		whereClause,
		params.SortBy.ToSQL(r.fieldMapping()),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to find users")
	}
	defer rows.Close()

	users, err := r.scanAndLoadUsers(ctx, rows)
	if err != nil {
		return nil, 0, err
	}

	return users, count, nil
}

func (r *pgUserQueryRepository) FindUserByID(ctx context.Context, userID int) (*viewmodels.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	row := tx.QueryRow(ctx, selectUserByIDSQL, userID, tenantID)
	dbUser, err := r.scanUser(row)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user by id")
	}

	return r.loadUserWithRelations(ctx, dbUser)
}

func (r *pgUserQueryRepository) SearchUsers(ctx context.Context, params *FindParams) ([]*viewmodels.User, int, error) {
	if params.Search == "" {
		return r.FindUsers(ctx, params)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get tenant ID")
	}

	// Build search condition
	searchFilter := r.buildSearchFilter(params.Search, 2) // $2 since $1 is tenant_id

	// Build combined conditions and args, starting with tenant filter
	allConditions := []string{"u.tenant_id = $1", searchFilter.condition}
	allArgs := []interface{}{tenantID}
	allArgs = append(allArgs, searchFilter.args...)

	// Add other filter conditions with proper placeholder indexing
	if len(params.Filters) > 0 {
		filterConditions, filterArgs := r.buildFilterConditionsWithStartIndex(params.Filters, len(allArgs)+1)
		allConditions = append(allConditions, filterConditions...)
		allArgs = append(allArgs, filterArgs...)
	}

	// Execute query with combined conditions
	return r.executeUserQuery(ctx, allConditions, allArgs, params)
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

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	var upload models.Upload
	err = tx.QueryRow(ctx, selectUploadByIDSQL, uploadID, tenantID).Scan(
		&upload.ID,
		&upload.TenantID,
		&upload.Hash,
		&upload.Path,
		&upload.Name,
		&upload.Size,
		&upload.Mimetype,
		&upload.Type,
		&upload.CreatedAt,
		&upload.UpdatedAt,
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
		err := rows.Scan(
			&role.ID,
			&role.Type,
			&role.Name,
			&role.Description,
			&role.CreatedAt,
			&role.UpdatedAt,
		)
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
		err := permRows.Scan(
			&perm.ID,
			&perm.Name,
			&perm.Resource,
			&perm.Action,
			&perm.Modifier,
		)
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

// scanAndLoadUsers scans user rows and loads related data (avatar, roles, permissions)
func (r *pgUserQueryRepository) scanAndLoadUsers(ctx context.Context, rows interface {
	Next() bool
	Scan(...interface{}) error
}) ([]*viewmodels.User, error) {
	users := make([]*viewmodels.User, 0)

	for rows.Next() {
		dbUser, err := r.scanUser(rows)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan user")
		}

		user, err := r.loadUserWithRelations(ctx, dbUser)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, nil
}

// scanUser scans a single user row
func (r *pgUserQueryRepository) scanUser(row interface{ Scan(...interface{}) error }) (*models.User, error) {
	var dbUser models.User
	err := row.Scan(
		&dbUser.ID,
		&dbUser.TenantID,
		&dbUser.Type,
		&dbUser.FirstName,
		&dbUser.LastName,
		&dbUser.MiddleName,
		&dbUser.Email,
		&dbUser.Phone,
		&dbUser.UILanguage,
		&dbUser.AvatarID,
		&dbUser.LastLogin,
		&dbUser.LastAction,
		&dbUser.CreatedAt,
		&dbUser.UpdatedAt,
	)
	return &dbUser, err
}

// loadUserWithRelations loads user with all related data (avatar, roles, permissions)
func (r *pgUserQueryRepository) loadUserWithRelations(ctx context.Context, dbUser *models.User) (*viewmodels.User, error) {
	// Load avatar if exists
	var avatar *models.Upload
	if dbUser.AvatarID.Valid {
		var err error
		avatar, err = r.loadUploadByID(ctx, int(dbUser.AvatarID.Int32))
		if err != nil {
			// Log error but don't fail the query
			avatar = nil
		}
	}

	user := mapToUserViewModel(*dbUser, avatar != nil, avatar)

	// Load roles and permissions
	if err := r.loadUserRolesAndPermissions(ctx, &user); err != nil {
		return nil, errors.Wrap(err, "failed to load roles and permissions")
	}

	return &user, nil
}

// buildSearchFilter creates a search condition for user search
func (r *pgUserQueryRepository) buildSearchFilter(search string, startIndex int) struct {
	condition string
	args      []interface{}
} {
	searchQuery := strings.TrimSpace(search)
	placeholder := fmt.Sprintf("$%d", startIndex)
	searchCondition := fmt.Sprintf(`(
		u.email ILIKE %s OR
		u.first_name ILIKE %s OR
		u.last_name ILIKE %s OR
		CONCAT(u.first_name, ' ', u.last_name) ILIKE %s
	)`, placeholder, placeholder, placeholder, placeholder)

	return struct {
		condition string
		args      []interface{}
	}{
		condition: searchCondition,
		args:      []interface{}{"%" + searchQuery + "%"},
	}
}

// executeUserQuery executes a user query with given conditions and args
func (r *pgUserQueryRepository) executeUserQuery(ctx context.Context, conditions []string, args []interface{}, params *FindParams) ([]*viewmodels.User, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = repo.JoinWhere(conditions...)
	}

	// Count query
	countQuery := repo.Join("SELECT COUNT(DISTINCT u.id) FROM users u", whereClause)
	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users")
	}

	// Build main query
	query := repo.Join(
		selectUsersSQL,
		whereClause,
		params.SortBy.ToSQL(r.fieldMapping()),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to execute user query")
	}
	defer rows.Close()

	users, err := r.scanAndLoadUsers(ctx, rows)
	if err != nil {
		return nil, 0, err
	}

	return users, count, nil
}
