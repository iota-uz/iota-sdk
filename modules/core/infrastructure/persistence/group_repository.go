package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrGroupNotFound = errors.New("group not found")
)

const (
	groupFindQuery = `
		SELECT DISTINCT
			g.id,
			g.name,
			g.description,
			g.created_at,
			g.updated_at,
			g.tenant_id
		FROM user_groups g`

	groupCountQuery = `SELECT COUNT(DISTINCT g.id) FROM user_groups g`

	groupDeleteQuery     = `DELETE FROM user_groups WHERE id = $1 AND tenant_id = $2`
	groupUserDeleteQuery = `DELETE FROM group_users WHERE group_id = $1`
	groupRoleDeleteQuery = `DELETE FROM group_roles WHERE group_id = $1`
	groupUserInsertQuery = `INSERT INTO group_users (group_id, user_id) VALUES`
	groupRoleInsertQuery = `INSERT INTO group_roles (group_id, role_id) VALUES`
)

type PgGroupRepository struct {
	userRepository user.Repository
	roleRepository role.Repository
	fieldMap       map[group.Field]string
}

func NewGroupRepository(userRepo user.Repository, roleRepo role.Repository) group.Repository {
	return &PgGroupRepository{
		userRepository: userRepo,
		roleRepository: roleRepo,
		fieldMap: map[group.Field]string{
			group.CreatedAt: "g.created_at",
			group.UpdatedAt: "g.updated_at",
			group.TenantID:  "g.tenant_id",
		},
	}
}

func (g *PgGroupRepository) buildGroupFilters(params *group.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	if params.Search != "" {
		index := len(args) + 1
		where = append(where, fmt.Sprintf("(g.name ILIKE $%d OR g.description ILIKE $%d)", index, index))
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (g *PgGroupRepository) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error) {
	sortFields := make([]string, 0, len(params.SortBy.Fields))

	for _, f := range params.SortBy.Fields {
		if field, ok := g.fieldMap[f]; ok {
			sortFields = append(sortFields, field)
		} else {
			return nil, errors.Wrap(fmt.Errorf("unknown sort field: %v", f), "invalid pagination parameters")
		}
	}

	where, args, err := g.buildGroupFilters(params)
	if err != nil {
		return nil, err
	}

	baseQuery := groupFindQuery

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
		repo.OrderBy(sortFields, params.SortBy.Ascending),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	groups, err := g.queryGroups(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get paginated groups")
	}
	return groups, nil
}

func (g *PgGroupRepository) Count(ctx context.Context, params *group.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	where, args, err := g.buildGroupFilters(params)
	if err != nil {
		return 0, err
	}

	baseQuery := groupCountQuery

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count groups")
	}
	return count, nil
}

func (g *PgGroupRepository) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	groups, err := g.queryGroups(ctx, groupFindQuery+" WHERE g.id = $1 AND g.tenant_id = $2", id.String(), tenant.ID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query group with id: %s", id.String()))
	}
	if len(groups) == 0 {
		return nil, errors.Wrap(ErrGroupNotFound, fmt.Sprintf("id: %s", id.String()))
	}
	return groups[0], nil
}

func (g *PgGroupRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction")
	}

	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get tenant from context")
	}

	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM user_groups WHERE id = $1 AND tenant_id = $2)",
		id.String(), tenant.ID).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if group exists")
	}
	return exists, nil
}

func (g *PgGroupRepository) Save(ctx context.Context, group group.Group) (group.Group, error) {
	exists, err := g.Exists(ctx, group.ID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if group exists")
	}

	if exists {
		return g.update(ctx, group)
	}
	return g.create(ctx, group)
}

func (g *PgGroupRepository) create(ctx context.Context, entity group.Group) (group.Group, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	// Generate a new UUID if not provided
	var groupID uuid.UUID
	if entity.ID() == uuid.Nil {
		groupID = uuid.New()
	} else {
		groupID = entity.ID()
	}

	dbGroup := ToDBGroup(entity)
	dbGroup.ID = groupID.String()
	dbGroup.TenantID = tenant.ID.String()

	fields := []string{
		"id",
		"name",
		"description",
		"created_at",
		"updated_at",
		"tenant_id",
	}

	values := []interface{}{
		dbGroup.ID,
		dbGroup.Name,
		dbGroup.Description,
		dbGroup.CreatedAt,
		dbGroup.UpdatedAt,
		dbGroup.TenantID,
	}

	_, err = tx.Exec(ctx, repo.Insert("user_groups", fields), values...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert group")
	}

	if err := g.updateGroupUsers(ctx, dbGroup.ID, entity.Users()); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update users for group ID: %s", dbGroup.ID))
	}

	if err := g.updateGroupRoles(ctx, dbGroup.ID, entity.Roles()); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update roles for group ID: %s", dbGroup.ID))
	}

	id, err := uuid.Parse(dbGroup.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse UUID")
	}

	return g.GetByID(ctx, id)
}

func (g *PgGroupRepository) update(ctx context.Context, entity group.Group) (group.Group, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbGroup := ToDBGroup(entity)

	fields := []string{
		"name",
		"description",
		"updated_at",
	}

	values := []interface{}{
		dbGroup.Name,
		dbGroup.Description,
		dbGroup.UpdatedAt,
	}

	values = append(values, dbGroup.ID)

	_, err = tx.Exec(ctx, repo.Update("user_groups", fields, fmt.Sprintf("id = $%d", len(values))), values...)

	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update group with ID: %s", dbGroup.ID))
	}

	if err := g.updateGroupUsers(ctx, dbGroup.ID, entity.Users()); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update users for group ID: %s", dbGroup.ID))
	}

	if err := g.updateGroupRoles(ctx, dbGroup.ID, entity.Roles()); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update roles for group ID: %s", dbGroup.ID))
	}

	id, err := uuid.Parse(dbGroup.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse UUID")
	}

	return g.GetByID(ctx, id)
}

func (g *PgGroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	uuidStr := id.String()

	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	if err := g.execQuery(ctx, groupUserDeleteQuery, uuidStr); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete users for group ID: %s", uuidStr))
	}

	if err := g.execQuery(ctx, groupRoleDeleteQuery, uuidStr); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete roles for group ID: %s", uuidStr))
	}

	if err := g.execQuery(ctx, groupDeleteQuery, uuidStr, tenant.ID); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete group with ID: %s", uuidStr))
	}

	return nil
}

func (g *PgGroupRepository) queryGroups(ctx context.Context, query string, args ...interface{}) ([]group.Group, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var dbGroups []*models.Group
	for rows.Next() {
		var dbGroup models.Group

		if err := rows.Scan(
			&dbGroup.ID,
			&dbGroup.Name,
			&dbGroup.Description,
			&dbGroup.CreatedAt,
			&dbGroup.UpdatedAt,
			&dbGroup.TenantID,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan group row")
		}
		dbGroups = append(dbGroups, &dbGroup)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	entities := make([]group.Group, 0, len(dbGroups))
	for _, dbGroup := range dbGroups {
		roles, err := g.groupRoles(ctx, dbGroup.ID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get roles for group ID: %s", dbGroup.ID))
		}

		users, err := g.groupUsers(ctx, dbGroup.ID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get users for group ID: %s", dbGroup.ID))
		}

		domainGroup, err := ToDomainGroup(dbGroup, users, roles)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert group ID: %s to domain entity", dbGroup.ID))
		}

		entities = append(entities, domainGroup)
	}

	return entities, nil
}

func (g *PgGroupRepository) groupRoles(ctx context.Context, groupID string) ([]role.Role, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	// Get role IDs associated with this group
	rows, err := tx.Query(ctx, "SELECT role_id FROM group_roles WHERE group_id = $1", groupID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query role IDs for group ID: %s", groupID))
	}
	defer rows.Close()

	var roleIDs []uint
	for rows.Next() {
		var roleID uint
		if err := rows.Scan(&roleID); err != nil {
			return nil, errors.Wrap(err, "failed to scan role ID")
		}
		roleIDs = append(roleIDs, roleID)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	// Get roles by their IDs using the role repository
	roles := make([]role.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role, err := g.roleRepository.GetByID(ctx, roleID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get role with ID: %d", roleID))
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (g *PgGroupRepository) groupUsers(ctx context.Context, groupID string) ([]user.User, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	// Get user IDs associated with this group
	rows, err := tx.Query(ctx, "SELECT user_id FROM group_users WHERE group_id = $1", groupID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user IDs for group ID: %s", groupID))
	}
	defer rows.Close()

	var userIDs []uint
	for rows.Next() {
		var userID uint
		if err := rows.Scan(&userID); err != nil {
			return nil, errors.Wrap(err, "failed to scan user ID")
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	// Get users by their IDs using the user repository
	users := make([]user.User, 0, len(userIDs))
	for _, userID := range userIDs {
		user, err := g.userRepository.GetByID(ctx, userID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get user with ID: %d", userID))
		}
		users = append(users, user)
	}

	return users, nil
}

func (g *PgGroupRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
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

func (g *PgGroupRepository) updateGroupUsers(ctx context.Context, groupID string, users []user.User) error {
	if err := g.execQuery(ctx, groupUserDeleteQuery, groupID); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete existing users for group ID: %s", groupID))
	}

	if len(users) == 0 {
		return nil
	}

	values := make([][]interface{}, 0, len(users))
	for _, u := range users {
		values = append(values, []interface{}{groupID, u.ID()})
	}
	q, args := repo.BatchInsertQueryN(groupUserInsertQuery, values)
	if err := g.execQuery(ctx, q, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to insert users for group ID: %s", groupID))
	}
	return nil
}

func (g *PgGroupRepository) updateGroupRoles(ctx context.Context, groupID string, roles []role.Role) error {
	if err := g.execQuery(ctx, groupRoleDeleteQuery, groupID); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete existing roles for group ID: %s", groupID))
	}

	if len(roles) == 0 {
		return nil
	}

	values := make([][]interface{}, 0, len(roles))
	for _, r := range roles {
		values = append(values, []interface{}{groupID, r.ID()})
	}
	q, args := repo.BatchInsertQueryN(groupRoleInsertQuery, values)
	if err := g.execQuery(ctx, q, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to insert roles for group ID: %s", groupID))
	}
	return nil
}
