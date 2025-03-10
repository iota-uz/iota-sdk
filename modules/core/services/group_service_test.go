package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test user
func createTestUser(t *testing.T, firstName, lastName, email string) user.User {
	emailObj, err := internet.NewEmail(email)
	require.NoError(t, err)
	return user.New(firstName, lastName, emailObj, user.UILanguageEN)
}

// Mock repositories
type mockGroupRepo struct {
	mock.Mock
}

func (m *mockGroupRepo) Count(ctx context.Context, params *group.FindParams) (int64, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockGroupRepo) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]group.Group), args.Error(1)
}

func (m *mockGroupRepo) GetByID(ctx context.Context, id group.GroupID) (group.Group, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(group.Group), args.Error(1)
}

func (m *mockGroupRepo) Save(ctx context.Context, g group.Group) (group.Group, error) {
	args := m.Called(ctx, g)
	return args.Get(0).(group.Group), args.Error(1)
}

func (m *mockGroupRepo) Delete(ctx context.Context, id group.GroupID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Mock EventBus
type mockEventBus struct {
	mock.Mock
}

func (m *mockEventBus) Publish(args ...interface{}) {
	m.Called(args...)
}

func (m *mockEventBus) Subscribe(handler interface{}) {
	m.Called(handler)
}

func (m *mockEventBus) Unsubscribe(handler interface{}) {
	m.Called(handler)
}

func (m *mockEventBus) Clear() {
	m.Called()
}

func (m *mockEventBus) SubscribersCount() int {
	args := m.Called()
	return args.Int(0)
}

func TestGroupService_GetByID(t *testing.T) {
	// Setup
	repo := new(mockGroupRepo)
	bus := new(mockEventBus)
	bus.On("Clear").Maybe()
	service := services.NewGroupService(repo, bus)
	
	// Create test data
	groupID := group.GroupID(uuid.New())
	testGroup := group.New("Test Group", group.WithID(groupID))
	
	// Configure mocks
	repo.On("GetByID", mock.Anything, groupID).Return(testGroup, nil)
	
	// Execute
	result, err := service.GetByID(context.Background(), groupID)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, testGroup, result)
	repo.AssertExpectations(t)
}

func TestGroupService_Count(t *testing.T) {
	// Setup
	repo := new(mockGroupRepo)
	bus := new(mockEventBus)
	bus.On("Clear").Maybe()
	service := services.NewGroupService(repo, bus)
	
	params := &group.FindParams{}
	expected := int64(5)
	
	// Configure mocks
	repo.On("Count", mock.Anything, params).Return(expected, nil)
	
	// Execute
	result, err := service.Count(context.Background(), params)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestGroupService_GetPaginated(t *testing.T) {
	// Setup
	repo := new(mockGroupRepo)
	bus := new(mockEventBus)
	bus.On("Clear").Maybe()
	service := services.NewGroupService(repo, bus)
	
	params := &group.FindParams{
		Limit:  10,
		Offset: 0,
	}
	
	group1 := group.New("Group 1", group.WithID(group.GroupID(uuid.New())))
	group2 := group.New("Group 2", group.WithID(group.GroupID(uuid.New())))
	expected := []group.Group{group1, group2}
	
	// Configure mocks
	repo.On("GetPaginated", mock.Anything, params).Return(expected, nil)
	
	// Execute
	result, err := service.GetPaginated(context.Background(), params)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}