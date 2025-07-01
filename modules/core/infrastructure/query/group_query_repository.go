package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

// SQL queries
const (
	selectGroupsSQL = `SELECT DISTINCT
		g.id, g.type, g.tenant_id, g.name, g.description,
		g.created_at, g.updated_at
	FROM user_groups g`

	selectGroupByIDSQL = `SELECT
		g.id, g.type, g.tenant_id, g.name, g.description,
		g.created_at, g.updated_at
	FROM user_groups g
	WHERE g.id = $1 AND g.tenant_id = $2`

	selectGroupUsersSQL = `SELECT
		u.id, u.tenant_id, u.type, u.first_name, u.last_name, u.middle_name,
		u.email, u.phone, u.ui_language, u.avatar_id, u.last_login, u.last_action,
		u.created_at, u.updated_at
	FROM users u
	JOIN group_users gu ON u.id = gu.user_id
	WHERE gu.group_id = $1 AND u.tenant_id = $2`

	selectGroupRolesSQL = `SELECT
		r.id, r.type, r.name, r.description, r.created_at, r.updated_at
	FROM roles r
	JOIN group_roles gr ON r.id = gr.role_id
	WHERE gr.group_id = $1`
)

// Field constants for group sorting and filtering
const (
	GroupFieldID        Field = "id"
	GroupFieldName      Field = "name"
	GroupFieldType      Field = "type"
	GroupFieldCreatedAt Field = "created_at"
	GroupFieldUpdatedAt Field = "updated_at"
	GroupFieldTenantID  Field = "tenant_id"
)

type GroupFilter = repo.FieldFilter[Field]

type GroupFindParams struct {
	Limit   int
	Offset  int
	SortBy  SortBy
	Search  string
	Filters []GroupFilter
}

type GroupQueryRepository interface {
	FindGroups(ctx context.Context, params *GroupFindParams) ([]*viewmodels.Group, int, error)
	FindGroupByID(ctx context.Context, groupID string) (*viewmodels.Group, error)
	SearchGroups(ctx context.Context, params *GroupFindParams) ([]*viewmodels.Group, int, error)
}

type pgGroupQueryRepository struct{}

func NewPgGroupQueryRepository() GroupQueryRepository {
	return &pgGroupQueryRepository{}
}

func (r *pgGroupQueryRepository) fieldMapping() map[Field]string {
	return map[Field]string{
		GroupFieldID:        "g.id",
		GroupFieldName:      "g.name",
		GroupFieldType:      "g.type",
		GroupFieldCreatedAt: "g.created_at",
		GroupFieldUpdatedAt: "g.updated_at",
		GroupFieldTenantID:  "g.tenant_id",
	}
}

func (r *pgGroupQueryRepository) filtersToSQL(ctx context.Context, filters []GroupFilter) ([]string, []interface{}) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return []string{}, []interface{}{}
	}

	// Always include tenant filter as first condition
	conditions := []string{"g.tenant_id = $1"}
	args := []interface{}{tenantID}

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

func (r *pgGroupQueryRepository) FindGroups(ctx context.Context, params *GroupFindParams) ([]*viewmodels.Group, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	conditions, args := r.filtersToSQL(ctx, params.Filters)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := params.SortBy.ToSQL(r.fieldMapping())

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT g.id) FROM user_groups g %s", whereClause)

	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count groups")
	}

	// Main query
	queryParts := []string{selectGroupsSQL}
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
		return nil, 0, errors.Wrap(err, "failed to find groups")
	}
	defer rows.Close()

	groups := make([]*viewmodels.Group, 0)
	for rows.Next() {
		var dbGroup models.Group
		err := rows.Scan(
			&dbGroup.ID, &dbGroup.Type, &dbGroup.TenantID, &dbGroup.Name, &dbGroup.Description,
			&dbGroup.CreatedAt, &dbGroup.UpdatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan group")
		}

		group := mapToGroupViewModel(dbGroup)

		// Load users and roles
		if err := r.loadGroupUsersAndRoles(ctx, &group); err != nil {
			return nil, 0, errors.Wrap(err, "failed to load users and roles")
		}

		groups = append(groups, &group)
	}

	return groups, count, nil
}

func (r *pgGroupQueryRepository) FindGroupByID(ctx context.Context, groupID string) (*viewmodels.Group, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	query := selectGroupByIDSQL
	args := []interface{}{groupID, tenantID}

	var dbGroup models.Group
	err = tx.QueryRow(ctx, query, args...).Scan(
		&dbGroup.ID, &dbGroup.Type, &dbGroup.TenantID, &dbGroup.Name, &dbGroup.Description,
		&dbGroup.CreatedAt, &dbGroup.UpdatedAt,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find group by id")
	}

	group := mapToGroupViewModel(dbGroup)

	// Load users and roles
	if err := r.loadGroupUsersAndRoles(ctx, &group); err != nil {
		return nil, errors.Wrap(err, "failed to load users and roles")
	}

	return &group, nil
}

func (r *pgGroupQueryRepository) SearchGroups(ctx context.Context, params *GroupFindParams) ([]*viewmodels.Group, int, error) {
	if params.Search == "" {
		return r.FindGroups(ctx, params)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get tenant ID")
	}

	searchQuery := strings.TrimSpace(params.Search)
	baseQuery := selectGroupsSQL + ` WHERE g.tenant_id = $1 AND (
		g.name ILIKE $2 OR
		g.description ILIKE $2
	)`

	args := []interface{}{tenantID, "%" + searchQuery + "%"}
	argIndex := 3

	// Add additional filters if any
	conditions, filterArgs := r.filtersToSQL(ctx, params.Filters)
	// Skip the first condition since it's the tenant filter we already added
	if len(conditions) > 1 {
		additionalConditions := conditions[1:]
		// Update arg positions in conditions
		for i, cond := range additionalConditions {
			for j := 2; j <= len(filterArgs); j++ {
				oldPlaceholder := fmt.Sprintf("$%d", j)
				newPlaceholder := fmt.Sprintf("$%d", argIndex)
				additionalConditions[i] = strings.Replace(cond, oldPlaceholder, newPlaceholder, 1)
				if strings.Contains(additionalConditions[i], newPlaceholder) {
					argIndex++
				}
			}
		}
		baseQuery += " AND " + strings.Join(additionalConditions, " AND ")
		// Skip the first arg (tenant ID) since we already added it
		args = append(args, filterArgs[1:]...)
	}

	// Count query - extract the WHERE clause from baseQuery
	whereClauseStart := strings.Index(baseQuery, "WHERE")
	countQuery := "SELECT COUNT(DISTINCT g.id) FROM user_groups g " + baseQuery[whereClauseStart:]

	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count groups")
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
		return nil, 0, errors.Wrap(err, "failed to search groups")
	}
	defer rows.Close()

	groups := make([]*viewmodels.Group, 0)
	for rows.Next() {
		var dbGroup models.Group
		err := rows.Scan(
			&dbGroup.ID, &dbGroup.Type, &dbGroup.TenantID, &dbGroup.Name, &dbGroup.Description,
			&dbGroup.CreatedAt, &dbGroup.UpdatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan group")
		}

		group := mapToGroupViewModel(dbGroup)

		// Load users and roles
		if err := r.loadGroupUsersAndRoles(ctx, &group); err != nil {
			return nil, 0, errors.Wrap(err, "failed to load users and roles")
		}

		groups = append(groups, &group)
	}

	return groups, count, nil
}

func (r *pgGroupQueryRepository) loadGroupUsersAndRoles(ctx context.Context, group *viewmodels.Group) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant ID")
	}

	// Load users
	userRows, err := tx.Query(ctx, selectGroupUsersSQL, group.ID, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to query group users")
	}
	defer userRows.Close()

	group.Users = make([]*viewmodels.User, 0)
	for userRows.Next() {
		var dbUser models.User
		err := userRows.Scan(
			&dbUser.ID, &dbUser.TenantID, &dbUser.Type, &dbUser.FirstName, &dbUser.LastName, &dbUser.MiddleName,
			&dbUser.Email, &dbUser.Phone, &dbUser.UILanguage, &dbUser.AvatarID, &dbUser.LastLogin, &dbUser.LastAction,
			&dbUser.CreatedAt, &dbUser.UpdatedAt,
		)
		if err != nil {
			return errors.Wrap(err, "failed to scan user")
		}

		// For group users, we don't need to load avatars and full details
		user := mapToUserViewModel(dbUser, false, nil)
		group.Users = append(group.Users, &user)
	}

	// Load roles
	roleRows, err := tx.Query(ctx, selectGroupRolesSQL, group.ID)
	if err != nil {
		return errors.Wrap(err, "failed to query group roles")
	}
	defer roleRows.Close()

	group.Roles = make([]*viewmodels.Role, 0)
	for roleRows.Next() {
		var dbRole models.Role
		err := roleRows.Scan(
			&dbRole.ID, &dbRole.Type, &dbRole.Name, &dbRole.Description,
			&dbRole.CreatedAt, &dbRole.UpdatedAt,
		)
		if err != nil {
			return errors.Wrap(err, "failed to scan role")
		}

		group.Roles = append(group.Roles, mapToRoleViewModel(dbRole))
	}

	return nil
}

func mapToGroupViewModel(dbGroup models.Group) viewmodels.Group {
	group := viewmodels.Group{
		ID:          dbGroup.ID,
		Type:        dbGroup.Type,
		Name:        dbGroup.Name,
		Description: dbGroup.Description.String,
		CreatedAt:   dbGroup.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   dbGroup.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CanUpdate:   dbGroup.Type == "user",
		CanDelete:   dbGroup.Type == "user",
	}

	return group
}
