package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test user
func createTestUser(t *testing.T, firstName, lastName, email string) user.User {
	emailObj, err := internet.NewEmail(email)
	require.NoError(t, err)
	return user.New(firstName, lastName, emailObj, user.UILanguageEN)
}

// TestGroupRepository provides a simple in-memory implementation for testing
type TestGroupRepository struct {
	groups map[string]group.Group
}

func NewTestGroupRepository() *TestGroupRepository {
	return &TestGroupRepository{
		groups: make(map[string]group.Group),
	}
}

func (r *TestGroupRepository) Count(ctx context.Context, params *group.FindParams) (int64, error) {
	return int64(len(r.groups)), nil
}

func (r *TestGroupRepository) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error) {
	result := make([]group.Group, 0, len(r.groups))
	for _, g := range r.groups {
		result = append(result, g)
	}
	
	// Apply simple pagination
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= len(result) {
		return []group.Group{}, nil
	}
	if end > len(result) {
		end = len(result)
	}
	
	return result[start:end], nil
}

func (r *TestGroupRepository) GetByID(ctx context.Context, id group.GroupID) (group.Group, error) {
	if g, exists := r.groups[uuid.UUID(id).String()]; exists {
		return g, nil
	}
	return nil, persistence.ErrGroupNotFound
}

func (r *TestGroupRepository) Save(ctx context.Context, g group.Group) (group.Group, error) {
	// Generate a new UUID if not provided
	if uuid.UUID(g.ID()) == uuid.Nil {
		g = group.New(g.Name(), group.WithID(group.GroupID(uuid.New())))
	}
	
	r.groups[uuid.UUID(g.ID()).String()] = g
	return g, nil
}

func (r *TestGroupRepository) Delete(ctx context.Context, id group.GroupID) error {
	delete(r.groups, uuid.UUID(id).String())
	return nil
}

func TestGroupService_GetByID(t *testing.T) {
	// Setup
	repo := NewTestGroupRepository()
	bus := eventbus.NewEventPublisher(logrus.New())
	service := services.NewGroupService(repo, bus)
	
	// Create test data
	groupID := group.GroupID(uuid.New())
	testGroup := group.New("Test Group", group.WithID(groupID))
	
	// Add the group to the repository
	_, err := repo.Save(context.Background(), testGroup)
	require.NoError(t, err)
	
	// Execute
	result, err := service.GetByID(context.Background(), groupID)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, testGroup.ID(), result.ID())
	assert.Equal(t, testGroup.Name(), result.Name())
}

func TestGroupService_Count(t *testing.T) {
	// Setup
	repo := NewTestGroupRepository()
	bus := eventbus.NewEventPublisher(logrus.New())
	service := services.NewGroupService(repo, bus)
	
	// Add some test groups
	group1 := group.New("Group 1", group.WithID(group.GroupID(uuid.New())))
	group2 := group.New("Group 2", group.WithID(group.GroupID(uuid.New())))
	group3 := group.New("Group 3", group.WithID(group.GroupID(uuid.New())))
	group4 := group.New("Group 4", group.WithID(group.GroupID(uuid.New())))
	group5 := group.New("Group 5", group.WithID(group.GroupID(uuid.New())))
	
	_, err := repo.Save(context.Background(), group1)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), group2)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), group3)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), group4)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), group5)
	require.NoError(t, err)
	
	// Execute
	result, err := service.Count(context.Background(), &group.FindParams{})
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(5), result)
}

func TestGroupService_GetPaginated(t *testing.T) {
	// Setup
	repo := NewTestGroupRepository()
	bus := eventbus.NewEventPublisher(logrus.New())
	service := services.NewGroupService(repo, bus)
	
	// Add test groups
	group1 := group.New("Group 1", group.WithID(group.GroupID(uuid.New())))
	group2 := group.New("Group 2", group.WithID(group.GroupID(uuid.New())))
	
	_, err := repo.Save(context.Background(), group1)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), group2)
	require.NoError(t, err)
	
	params := &group.FindParams{
		Limit:  10,
		Offset: 0,
	}
	
	// Execute
	result, err := service.GetPaginated(context.Background(), params)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result))
}