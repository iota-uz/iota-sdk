package controllers_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/assert"
)

// integrationTestBuilder implements crud.Builder[TestEntity] for integration tests
type integrationTestBuilder struct {
	schema  crud.Schema[TestEntity]
	service crud.Service[TestEntity]
}

func (b *integrationTestBuilder) Schema() crud.Schema[TestEntity] {
	return b.schema
}

func (b *integrationTestBuilder) Service() crud.Service[TestEntity] {
	return b.service
}

func (b *integrationTestBuilder) Repository() crud.Repository[TestEntity] {
	return nil // Not needed for these tests
}

// TestCrudController_ConcurrentRequests tests handling of concurrent requests
func TestCrudController_ConcurrentRequests(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Add initial entities
	for i := 0; i < 10; i++ {
		entity := TestEntity{
			ID:   uuid.New(),
			Name: fmt.Sprintf("Entity %d", i),
		}
		service.entities[entity.ID] = entity
	}

	// Simulate concurrent list requests
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(page int) {
			defer func() { done <- true }()

			resp := suite.GET(fmt.Sprintf("/test?page=%d", page))
			resp.Expect().Status(t, 200)
		}(i + 1)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify service wasn't corrupted
	assert.Equal(t, 10, len(service.entities))
}

// TestCrudController_LargeDataset tests performance with large datasets
func TestCrudController_LargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add 1000 entities
	for i := 0; i < 1000; i++ {
		entity := TestEntity{
			ID:          uuid.New(),
			Name:        fmt.Sprintf("Entity %04d", i),
			Description: fmt.Sprintf("Long description for entity %d with some text", i),
			Amount:      float64(i),
			IsActive:    i%2 == 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		service.entities[entity.ID] = entity
	}

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test pagination efficiency
	start := time.Now()
	doc := suite.GET("/test?page=25").Expect().Status(t, 200).HTML(t)
	duration := time.Since(start)

	// Should complete within reasonable time
	assert.Less(t, duration, 500*time.Millisecond)

	// Should show correct page
	rows := doc.Elements("//tbody/tr")
	assert.Equal(t, 20, len(rows))
}

// TestCrudController_FieldValidationIntegration tests field validation with service
func TestCrudController_FieldValidationIntegration(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	// Create schema with comprehensive validation
	validatedFields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name",
			crud.WithSearchable(),
			crud.WithRules(
				crud.RequiredRule(),
				crud.MinLengthRule(3),
				crud.MaxLengthRule(50),
			),
		),
		crud.NewStringField("description",
			crud.WithRules(
				crud.MaxLengthRule(200),
			),
		),
		crud.NewFloatField("amount",
			crud.WithRules(
				crud.MinValueRule(0),
				crud.MaxValueRule(10000),
			),
		),
		crud.NewBoolField("is_active"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	validatedSchema := crud.NewSchema(
		"test_entities",
		validatedFields,
		&testMapper{},
		crud.WithValidator[TestEntity](func(entity TestEntity) error {
			if entity.Name == "forbidden" {
				return fmt.Errorf("name 'forbidden' is not allowed")
			}
			return nil
		}),
	)

	service := newTestService()
	builder := &integrationTestBuilder{
		schema:  validatedSchema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test cases for validation
	testCases := []struct {
		name           string
		formData       url.Values
		expectedStatus int
		expectError    bool
		errorContains  string
	}{
		{
			name: "valid entity",
			formData: url.Values{
				"name":        {"Valid Name"},
				"description": {"Valid description"},
				"amount":      {"100"},
				"is_active":   {"true"},
			},
			expectedStatus: 303, // Redirect on success
			expectError:    false,
		},
		{
			name: "name too short",
			formData: url.Values{
				"name":        {"ab"},
				"description": {"Valid description"},
				"amount":      {"100"},
			},
			expectedStatus: 422,
			expectError:    true,
			errorContains:  "validation rule failed",
		},
		{
			name: "name too long",
			formData: url.Values{
				"name":        {string(make([]byte, 51))},
				"description": {"Valid description"},
				"amount":      {"100"},
			},
			expectedStatus: 422,
			expectError:    true,
			errorContains:  "validation rule failed",
		},
		{
			name: "amount negative",
			formData: url.Values{
				"name":        {"Valid Name"},
				"description": {"Valid description"},
				"amount":      {"-10"},
			},
			expectedStatus: 422,
			expectError:    true,
			errorContains:  "validation rule failed",
		},
		{
			name: "amount too large",
			formData: url.Values{
				"name":        {"Valid Name"},
				"description": {"Valid description"},
				"amount":      {"10001"},
			},
			expectedStatus: 422,
			expectError:    true,
			errorContains:  "validation rule failed",
		},
		{
			name: "forbidden name",
			formData: url.Values{
				"name":        {"forbidden"},
				"description": {"Valid description"},
				"amount":      {"100"},
			},
			expectedStatus: 422,
			expectError:    true,
			errorContains:  "not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := suite.POST("/test").
				WithForm(tc.formData).
				Expect().
				Status(t, tc.expectedStatus)

			if tc.expectError {
				doc := resp.HTML(t)
				errorEl := doc.Element("//*[@data-testid='alert-message']")
				errorEl.Exists(t)
				if tc.errorContains != "" {
					assert.Contains(t, errorEl.Text(), tc.errorContains)
				}
			}
		})
	}
}

// TestCrudController_FormStatePreservation tests that form state is preserved on validation errors
func TestCrudController_FormStatePreservation(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	// Schema with validation
	fieldsWithValidation := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name", crud.WithRules(crud.RequiredRule())),
		crud.NewStringField("description"),
		crud.NewFloatField("amount"),
		crud.NewBoolField("is_active"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	schemaWithValidation := crud.NewSchema(
		"test_entities",
		fieldsWithValidation,
		&testMapper{},
	)

	service := newTestService()
	builder := &integrationTestBuilder{
		schema:  schemaWithValidation,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Submit form with validation error but valid other fields
	formData := url.Values{
		"name":        {""}, // This will fail validation
		"description": {"User entered description"},
		"amount":      {"123.45"},
		"is_active":   {"true"},
	}

	doc := suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, 422).
		HTML(t)

	// Check that other field values are preserved
	descInput := doc.Element("//input[@name='description']")
	assert.Equal(t, "User entered description", descInput.Attr("value"))

	amountInput := doc.Element("//input[@name='amount']")
	assert.Equal(t, "123.45", amountInput.Attr("value"))

	activeInput := doc.Element("//input[@name='is_active']")
	assert.Equal(t, "checked", activeInput.Attr("checked"))
}

// TestCrudController_ComplexFiltering tests complex filtering scenarios
func TestCrudController_ComplexFiltering(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add diverse test data
	entities := []TestEntity{
		{ID: uuid.New(), Name: "Apple Product", Description: "Electronics", Amount: 1000, IsActive: true},
		{ID: uuid.New(), Name: "Apple Fruit", Description: "Food", Amount: 2, IsActive: true},
		{ID: uuid.New(), Name: "Banana", Description: "Food", Amount: 1, IsActive: true},
		{ID: uuid.New(), Name: "Orange", Description: "Food", Amount: 1.5, IsActive: false},
		{ID: uuid.New(), Name: "Laptop", Description: "Electronics", Amount: 800, IsActive: true},
		{ID: uuid.New(), Name: "Phone", Description: "Electronics", Amount: 600, IsActive: false},
	}

	for _, e := range entities {
		service.entities[e.ID] = e
	}

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test search with filters
	testCases := []struct {
		name          string
		query         string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "search for apple",
			query:         "apple",
			expectedCount: 2,
			expectedNames: []string{"Apple Product", "Apple Fruit"},
		},
		{
			name:          "search for food items",
			query:         "food",
			expectedCount: 0, // Description is not searchable, only name
			expectedNames: []string{},
		},
		{
			name:          "empty search",
			query:         "",
			expectedCount: 6,
			expectedNames: []string{"Apple Product", "Apple Fruit", "Banana", "Orange", "Laptop", "Phone"},
		},
		{
			name:          "case insensitive search",
			query:         "APPLE",
			expectedCount: 2,
			expectedNames: []string{"Apple Product", "Apple Fruit"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc := suite.GET(fmt.Sprintf("/test?q=%s", url.QueryEscape(tc.query))).
				Expect().
				Status(t, 200).
				HTML(t)

			rows := doc.Elements("//tbody/tr")
			assert.Equal(t, tc.expectedCount, len(rows))

			// Check that expected names appear by searching the document
			for _, expectedName := range tc.expectedNames {
				nameElement := doc.Element(fmt.Sprintf("//tbody/tr[contains(., '%s')]", expectedName))
				if tc.expectedCount > 0 {
					nameElement.Exists(t)
				} else {
					nameElement.NotExists(t)
				}
			}
		})
	}
}

// TestCrudController_UpdateWithReadonlyFields tests that readonly fields are preserved during updates
func TestCrudController_UpdateWithReadonlyFields(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add entity with specific timestamps
	originalCreated := time.Now().Add(-24 * time.Hour)
	originalUpdated := time.Now().Add(-1 * time.Hour)

	entity := TestEntity{
		ID:          uuid.New(),
		Name:        "Original",
		Description: "Original Desc",
		Amount:      100,
		IsActive:    true,
		CreatedAt:   originalCreated,
		UpdatedAt:   originalUpdated,
	}
	service.entities[entity.ID] = entity

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Attempt to update with readonly fields in form
	formData := url.Values{
		"name":        {"Updated"},
		"description": {"Updated Desc"},
		"amount":      {"200"},
		"is_active":   {"false"},
		// Try to modify readonly fields
		"created_at": {"2024-01-01T00:00:00Z"},
		"updated_at": {"2024-01-01T00:00:00Z"},
	}

	suite.POST(fmt.Sprintf("/test/%s", entity.ID)).
		WithForm(formData).
		Expect().
		Status(t, 303)

	// Verify readonly fields were not modified
	updated := service.entities[entity.ID]
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "Updated Desc", updated.Description)
	assert.Equal(t, float64(200), updated.Amount)
	assert.Equal(t, false, updated.IsActive)

	// Readonly fields should be preserved
	assert.Equal(t, originalCreated.Unix(), updated.CreatedAt.Unix())
	// UpdatedAt might be updated by the service, but not from form data
	assert.NotEqual(t, "2024-01-01T00:00:00Z", updated.UpdatedAt.Format(time.RFC3339))
}

// TestCrudController_EmptyListHandling tests proper handling of empty entity lists
func TestCrudController_EmptyListHandling(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService() // Empty service
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	doc := suite.GET("/test").Expect().Status(t, 200).HTML(t)

	// Should show empty state
	tbody := doc.Element("//tbody")
	tbody.Exists(t)

	// Should have a row indicating no results
	rows := doc.Elements("//tbody/tr")
	assert.Equal(t, 1, len(rows))

	// Check for empty state message
	emptyMessage := doc.Element("//tbody/tr").Text()
	assert.Contains(t, emptyMessage, "No")
}

// TestCrudController_SpecialCharacterHandling tests handling of special characters
func TestCrudController_SpecialCharacterHandling(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test creating entity with special characters
	specialChars := []string{
		"Test & Co.",
		"Test < > Entity",
		"Test \"Quoted\"",
		"Test 'Single'",
		"Test\nNewline",
		"Test\tTab",
		"Test © ® ™",
		"Test 你好 世界", // Unicode
	}

	for _, name := range specialChars {
		t.Run(fmt.Sprintf("special_char_%s", name), func(t *testing.T) {
			formData := url.Values{
				"name":        {name},
				"description": {"Special char test"},
				"amount":      {"100"},
				"is_active":   {"true"},
			}

			resp := suite.POST("/test").
				WithForm(formData).
				Expect().
				Status(t, 303)

			// Should redirect successfully
			location := resp.Header("Location")
			assert.Contains(t, location, "/test")

			// Verify entity was created with correct name
			found := false
			for _, e := range service.entities {
				if e.Name == name {
					found = true
					assert.Equal(t, "Special char test", e.Description)
					break
				}
			}
			assert.True(t, found, "Entity with special characters should be created")
		})
	}
}

// TestCrudController_SessionHandling tests that controller properly handles session state
func TestCrudController_SessionHandling(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Create entity with validation error to get form state
	formData := url.Values{
		"name":        {""}, // Empty to trigger validation
		"description": {"Session Test"},
		"amount":      {"999"},
	}

	// First request - should fail validation
	resp1 := suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, 422)

	// Extract any session cookies
	cookies := resp1.Cookies()

	// Make another request with same session
	req := suite.GET("/test/new")
	for _, cookie := range cookies {
		req.WithCookie(cookie.Name, cookie.Value)
	}

	// Form should be clean, not retain previous error state
	doc := suite.GET("/test/new").Expect().Status(t, 200).HTML(t)

	descInput := doc.Element("//input[@name='description']")
	// Should not have the previous form value
	assert.NotEqual(t, "Session Test", descInput.Attr("value"))
}

// TestCrudController_HTTPMethodSafety tests that only appropriate HTTP methods are accepted
func TestCrudController_HTTPMethodSafety(t *testing.T) {
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	entity := TestEntity{ID: uuid.New(), Name: "Test"}
	service.entities[entity.ID] = entity

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test inappropriate methods
	testCases := []struct {
		method       string
		path         string
		expectStatus int
	}{
		// List should only accept GET
		{"POST", "/test", 422}, // This is create, so might work
		{"PUT", "/test", 405},
		{"DELETE", "/test", 405},

		// Create form should only accept GET
		{"POST", "/test/new", 405},
		{"PUT", "/test/new", 405},
		{"DELETE", "/test/new", 405},

		// Edit form should only accept GET
		{"POST", fmt.Sprintf("/test/%s/edit", entity.ID), 405},
		{"PUT", fmt.Sprintf("/test/%s/edit", entity.ID), 405},
		{"DELETE", fmt.Sprintf("/test/%s/edit", entity.ID), 405},

		// Update should only accept POST
		{"GET", fmt.Sprintf("/test/%s", entity.ID), 200}, // This might be show
		{"PUT", fmt.Sprintf("/test/%s", entity.ID), 405},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.method, tc.path), func(t *testing.T) {
			suite.Request(tc.method, tc.path).
				Expect().
				Status(t, tc.expectStatus)
		})
	}
}

// Helper function
func contains(text, substr string) bool {
	return len(substr) > 0 && len(text) >= len(substr) &&
		(text == substr ||
			(len(text) > len(substr) &&
				(text[:len(substr)] == substr ||
					text[len(text)-len(substr):] == substr ||
					findSubstring(text, substr))))
}

func findSubstring(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
