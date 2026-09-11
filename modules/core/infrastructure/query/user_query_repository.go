// Package query provides this package.
package query

import (
	"context"
	"database/sql"
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
		u.created_at, u.updated_at, u.is_blocked, u.block_reason, u.blocked_at, u.blocked_by
	FROM users u`

	selectUserByIDSQL = `SELECT
		u.id, u.tenant_id, u.type, u.first_name, u.last_name, u.middle_name,
		u.email, u.phone, u.ui_language, u.avatar_id, u.last_login, u.last_action,
		u.created_at, u.updated_at, u.is_blocked, u.block_reason, u.blocked_at, u.blocked_by
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
	FieldRoleID    Field = "role_id"
	FieldBlocked   Field = "is_blocked"
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
	CanDeleteUser(ctx context.Context, userID int) (bool, error)
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
		FieldRoleID:    "ur.role_id",
		FieldBlocked:   "u.is_blocked",
	}
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

// buildRoleFilterCondition builds the SQL condition for role filters
func (r *pgUserQueryRepository) buildRoleFilterCondition(filter *Filter, startIndex int) (string, []interface{}) {
	roleValues := filter.Filter.Value()
	if len(roleValues) == 0 {
		return "", nil
	}

	placeholders := make([]string, len(roleValues))
	args := make([]interface{}, 0, len(roleValues))

	for i, val := range roleValues {
		placeholders[i] = fmt.Sprintf("$%d", startIndex+i)
		// Convert string role ID to uint
		roleIDStr, ok := val.(string)
		if !ok {
			continue // Skip invalid values
		}
		roleID, err := strconv.ParseUint(roleIDStr, 10, 32)
		if err != nil {
			continue // Skip invalid numbers
		}
		args = append(args, uint(roleID))
	}

	if len(args) == 0 {
		return "", nil
	}

	return fmt.Sprintf("ur.role_id IN (%s)", strings.Join(placeholders[:len(args)], ", ")), args
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

	// Separate group and role filters from regular filters
	var regularFilters []Filter
	var groupFilter *Filter
	var roleFilter *Filter

	for _, f := range params.Filters {
		switch f.Column {
		case FieldGroupID:
			groupFilter = &f
		case FieldRoleID:
			roleFilter = &f
		default:
			regularFilters = append(regularFilters, f)
		}
	}

	// Build conditions and args, starting with tenant filter
	conditions := []string{"u.tenant_id = $1"}
	args := []interface{}{tenantID}

	// Add search condition if provided
	if params.Search != "" {
		searchFilter := r.buildSearchFilter(params.Search, len(args)+1)
		conditions = append(conditions, searchFilter.condition)
		args = append(args, searchFilter.args...)
	}

	// Add regular filter conditions
	if len(regularFilters) > 0 {
		filterConditions, filterArgs := r.buildFilterConditionsWithStartIndex(regularFilters, len(args)+1)
		conditions = append(conditions, filterConditions...)
		args = append(args, filterArgs...)
	}

	// Handle group and role filters specially with JOIN clauses
	joinClause := ""
	if groupFilter != nil {
		joinClause += " JOIN group_users gu ON u.id = gu.user_id"
		groupCondition, groupArgs := r.buildGroupFilterCondition(groupFilter, len(args)+1)
		if groupCondition != "" {
			conditions = append(conditions, groupCondition)
			args = append(args, groupArgs...)
		}
	}

	if roleFilter != nil {
		joinClause += " JOIN user_roles ur ON u.id = ur.user_id"
		roleCondition, roleArgs := r.buildRoleFilterCondition(roleFilter, len(args)+1)
		if roleCondition != "" {
			conditions = append(conditions, roleCondition)
			args = append(args, roleArgs...)
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

func (r *pgUserQueryRepository) CanDeleteUser(ctx context.Context, userID int) (bool, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction")
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get tenant ID")
	}
	var canDelete bool
	err = tx.QueryRow(ctx, `SELECT u.type <> 'system' AND
		(SELECT COUNT(*) FROM users WHERE tenant_id = $2) > 1
		FROM users u WHERE u.id = $1 AND u.tenant_id = $2`, userID, tenantID).Scan(&canDelete)
	return canDelete, errors.Wrap(err, "failed to determine whether user can be deleted")
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

func (r *pgUserQueryRepository) loadUserRelationsBatch(ctx context.Context, dbUsers []*models.User, users []*viewmodels.User) error {
	if len(users) == 0 {
		return nil
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant ID")
	}
	byID := make(map[uint]*viewmodels.User, len(users))
	userIDs := make([]uint, 0, len(users))
	avatarIDs := make([]uint, 0)
	blockedByIDs := make([]uint, 0)
	for i, item := range users {
		id := dbUsers[i].ID
		byID[id] = item
		userIDs = append(userIDs, id)
		item.Roles = make([]*viewmodels.Role, 0)
		item.Permissions = make([]*viewmodels.Permission, 0)
		item.DirectPermissions = make([]*viewmodels.Permission, 0)
		item.EffectivePermissions = make([]*viewmodels.Permission, 0)
		item.GroupIDs = make([]string, 0)
		if dbUsers[i].AvatarID.Valid {
			avatarIDs = append(avatarIDs, uint(dbUsers[i].AvatarID.Int32))
		}
		if dbUsers[i].BlockedBy.Valid {
			blockedByIDs = append(blockedByIDs, uint(dbUsers[i].BlockedBy.Int64))
		}
	}

	if len(avatarIDs) > 0 {
		rows, queryErr := tx.Query(ctx, `SELECT id, tenant_id, hash, path, name, size, mimetype, type, created_at, updated_at
			FROM uploads WHERE id = ANY($1::int[]) AND tenant_id = $2`, avatarIDs, tenantID)
		if queryErr != nil {
			return errors.Wrap(queryErr, "failed to query avatars")
		}
		uploads := make(map[uint]*models.Upload, len(avatarIDs))
		for rows.Next() {
			var upload models.Upload
			if scanErr := rows.Scan(&upload.ID, &upload.TenantID, &upload.Hash, &upload.Path, &upload.Name, &upload.Size, &upload.Mimetype, &upload.Type, &upload.CreatedAt, &upload.UpdatedAt); scanErr != nil {
				rows.Close()
				return errors.Wrap(scanErr, "failed to scan avatar")
			}
			uploads[upload.ID] = &upload
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return errors.Wrap(rowsErr, "failed to iterate avatars")
		}
		rows.Close()
		for i, dbUser := range dbUsers {
			if dbUser.AvatarID.Valid {
				if avatar := uploads[uint(dbUser.AvatarID.Int32)]; avatar != nil {
					mapped := mapToUserViewModel(*dbUser, true, avatar)
					users[i].Avatar = mapped.Avatar
				}
			}
		}
	}

	rows, err := tx.Query(ctx, `SELECT ur.user_id, r.id, r.type, r.name, r.description, r.created_at, r.updated_at
		FROM user_roles ur JOIN users u ON u.id = ur.user_id
		JOIN roles r ON r.id = ur.role_id AND r.tenant_id = u.tenant_id
		WHERE ur.user_id = ANY($1::int[]) AND u.tenant_id = $2 ORDER BY ur.user_id, r.id`, userIDs, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to query roles")
	}
	for rows.Next() {
		var userID uint
		var role models.Role
		err := rows.Scan(
			&userID,
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

		byID[userID].Roles = append(byID[userID].Roles, mapToRoleViewModel(role))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.Wrap(err, "failed to iterate roles")
	}
	rows.Close()

	permRows, err := tx.Query(ctx, `SELECT grants.user_id, p.id, p.name, p.resource, p.action, p.modifier,
		BOOL_OR(grants.is_direct), BOOL_OR(grants.in_legacy_projection)
		FROM (
			SELECT up.user_id, up.permission_id, TRUE AS is_direct, TRUE AS in_legacy_projection FROM user_permissions up
			UNION
			SELECT ur.user_id, rp.permission_id, FALSE, TRUE FROM user_roles ur JOIN role_permissions rp ON rp.role_id = ur.role_id
			UNION
			SELECT gu.user_id, rp.permission_id, FALSE, FALSE FROM group_users gu
			JOIN user_groups g ON g.id = gu.group_id
			JOIN group_roles gr ON gr.group_id = g.id
			JOIN roles r ON r.id = gr.role_id AND r.tenant_id = g.tenant_id
			JOIN role_permissions rp ON rp.role_id = r.id
		) grants
		JOIN users u ON u.id = grants.user_id
		JOIN permissions p ON p.id = grants.permission_id
		WHERE grants.user_id = ANY($1::int[]) AND u.tenant_id = $2
		GROUP BY grants.user_id, p.id, p.name, p.resource, p.action, p.modifier
		ORDER BY grants.user_id, p.name, p.id`, userIDs, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to query permissions")
	}
	for permRows.Next() {
		var userID uint
		var perm models.Permission
		var isDirect, inLegacyProjection bool
		err := permRows.Scan(
			&userID,
			&perm.ID,
			&perm.Name,
			&perm.Resource,
			&perm.Action,
			&perm.Modifier,
			&isDirect,
			&inLegacyProjection,
		)
		if err != nil {
			return errors.Wrap(err, "failed to scan permission")
		}

		mapped := mapToPermissionViewModel(perm)
		byID[userID].EffectivePermissions = append(byID[userID].EffectivePermissions, mapped)
		if inLegacyProjection {
			byID[userID].Permissions = append(byID[userID].Permissions, mapped)
		}
		if isDirect {
			byID[userID].DirectPermissions = append(byID[userID].DirectPermissions, mapped)
		}
	}
	if err := permRows.Err(); err != nil {
		permRows.Close()
		return errors.Wrap(err, "failed to iterate permissions")
	}
	permRows.Close()

	groupRows, err := tx.Query(ctx, `SELECT gu.user_id, gu.group_id FROM group_users gu
		JOIN users u ON u.id = gu.user_id JOIN user_groups g ON g.id = gu.group_id AND g.tenant_id = u.tenant_id
		WHERE gu.user_id = ANY($1::int[]) AND u.tenant_id = $2 ORDER BY gu.user_id, gu.group_id`, userIDs, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to query groups")
	}
	for groupRows.Next() {
		var userID uint
		var groupID uuid.UUID
		if err := groupRows.Scan(&userID, &groupID); err != nil {
			return errors.Wrap(err, "failed to scan group ID")
		}
		byID[userID].GroupIDs = append(byID[userID].GroupIDs, groupID.String())
	}
	if err := groupRows.Err(); err != nil {
		groupRows.Close()
		return errors.Wrap(err, "failed to iterate groups")
	}
	groupRows.Close()

	if len(blockedByIDs) > 0 {
		blockerRows, queryErr := tx.Query(ctx, `SELECT id, first_name, last_name, phone, email FROM users
			WHERE id = ANY($1::int[]) AND tenant_id = $2`, blockedByIDs, tenantID)
		if queryErr != nil {
			return errors.Wrap(queryErr, "failed to query blocker labels")
		}
		labels := make(map[uint]string, len(blockedByIDs))
		for blockerRows.Next() {
			var id uint
			var firstName, lastName, email string
			var phone sql.NullString
			if scanErr := blockerRows.Scan(&id, &firstName, &lastName, &phone, &email); scanErr != nil {
				blockerRows.Close()
				return errors.Wrap(scanErr, "failed to scan blocker label")
			}
			label := strings.TrimSpace(firstName + " " + lastName)
			if label == "" && phone.Valid {
				label = phone.String
			}
			if label == "" {
				label = email
			}
			labels[id] = label
		}
		if rowsErr := blockerRows.Err(); rowsErr != nil {
			blockerRows.Close()
			return errors.Wrap(rowsErr, "failed to iterate blocker labels")
		}
		blockerRows.Close()
		for i, dbUser := range dbUsers {
			if dbUser.BlockedBy.Valid {
				users[i].BlockedByUser = labels[uint(dbUser.BlockedBy.Int64)]
			}
		}
	}

	return nil
}

// scanAndLoadUsers drains the base cursor before loading every relation in batches.
func (r *pgUserQueryRepository) scanAndLoadUsers(ctx context.Context, rows interface {
	Next() bool
	Scan(...interface{}) error
}) ([]*viewmodels.User, error) {
	dbUsers := make([]*models.User, 0)

	for rows.Next() {
		dbUser, err := r.scanUser(rows)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan user")
		}

		dbUsers = append(dbUsers, dbUser)
	}

	users := make([]*viewmodels.User, 0, len(dbUsers))
	for _, dbUser := range dbUsers {
		user := mapToUserViewModel(*dbUser, false, nil)
		users = append(users, &user)
	}
	if err := r.loadUserRelationsBatch(ctx, dbUsers, users); err != nil {
		return nil, err
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
		&dbUser.IsBlocked,
		&dbUser.BlockReason,
		&dbUser.BlockedAt,
		&dbUser.BlockedBy,
	)
	return &dbUser, err
}

// loadUserWithRelations loads user with all related data (avatar, roles, permissions)
func (r *pgUserQueryRepository) loadUserWithRelations(ctx context.Context, dbUser *models.User) (*viewmodels.User, error) {
	user := mapToUserViewModel(*dbUser, false, nil)
	if err := r.loadUserRelationsBatch(ctx, []*models.User{dbUser}, []*viewmodels.User{&user}); err != nil {
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
