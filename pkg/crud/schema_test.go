package crud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
)

// Mock DataStore for testing
type mockDataStore struct {
	items  map[int]*TestEntity
	nextID int
}

func newMockDataStore() *mockDataStore {
	return &mockDataStore{
		items:  make(map[int]*TestEntity),
		nextID: 1,
	}
}

func (m *mockDataStore) List(ctx context.Context, params FindParams) ([]*TestEntity, error) {
	result := make([]*TestEntity, 0, len(m.items))
	for _, item := range m.items {
		// Basic search implementation
		if params.Search != "" && !strings.Contains(item.Name, params.Search) {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (m *mockDataStore) Get(ctx context.Context, id int) (*TestEntity, error) {
	if item, ok := m.items[id]; ok {
		return item, nil
	}
	return &TestEntity{}, ErrNotFound
}

func (m *mockDataStore) Create(ctx context.Context, entity *TestEntity) (int, error) {
	id := m.nextID
	m.nextID++
	entity.ID = id
	m.items[id] = entity
	return id, nil
}

func (m *mockDataStore) Update(ctx context.Context, id int, entity *TestEntity) error {
	if _, ok := m.items[id]; !ok {
		return ErrNotFound
	}
	entity.ID = id
	m.items[id] = entity
	return nil
}

func (m *mockDataStore) Delete(ctx context.Context, id int) error {
	if _, ok := m.items[id]; !ok {
		return ErrNotFound
	}
	delete(m.items, id)
	return nil
}

// TestEntity for testing
type TestEntity struct {
	ID        int
	Name      string
	Email     string
	Age       int
	Active    bool
	CreatedAt time.Time
}

// Mock model validator
type mockValidator struct {
	shouldFail bool
	errMsg     string
}

func (v *mockValidator) ValidateModel(ctx context.Context, model *TestEntity) error {
	if v.shouldFail {
		return errors.New(v.errMsg)
	}
	return nil
}

// mockRenderer struct removed as unused

func TestParseID(t *testing.T) {
	tests := []struct {
		name      string
		idStr     string
		idType    interface{}
		want      interface{}
		wantError bool
	}{
		{
			name:      "string ID",
			idStr:     "abc123",
			idType:    "",
			want:      "abc123",
			wantError: false,
		},
		{
			name:      "int ID",
			idStr:     "123",
			idType:    int(0),
			want:      int(123),
			wantError: false,
		},
		{
			name:      "int64 ID",
			idStr:     "123456789",
			idType:    int64(0),
			want:      int64(123456789),
			wantError: false,
		},
		{
			name:      "uint ID",
			idStr:     "123",
			idType:    uint(0),
			want:      uint(123),
			wantError: false,
		},
		{
			name:      "invalid int",
			idStr:     "abc",
			idType:    int(0),
			want:      int(0),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.idType.(type) {
			case string:
				result, err := parseID[string](tt.idStr)
				if (err != nil) != tt.wantError {
					t.Errorf("parseID() error = %v, wantError %v", err, tt.wantError)
					return
				}
				if !tt.wantError && result != tt.want {
					t.Errorf("parseID() = %v, want %v", result, tt.want)
				}
			case int:
				result, err := parseID[int](tt.idStr)
				if (err != nil) != tt.wantError {
					t.Errorf("parseID() error = %v, wantError %v", err, tt.wantError)
					return
				}
				if !tt.wantError && result != tt.want {
					t.Errorf("parseID() = %v, want %v", result, tt.want)
				}
			case int64:
				result, err := parseID[int64](tt.idStr)
				if (err != nil) != tt.wantError {
					t.Errorf("parseID() error = %v, wantError %v", err, tt.wantError)
					return
				}
				if !tt.wantError && result != tt.want {
					t.Errorf("parseID() = %v, want %v", result, tt.want)
				}
			case uint:
				result, err := parseID[uint](tt.idStr)
				if (err != nil) != tt.wantError {
					t.Errorf("parseID() error = %v, wantError %v", err, tt.wantError)
					return
				}
				if !tt.wantError && result != tt.want {
					t.Errorf("parseID() = %v, want %v", result, tt.want)
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name   string
		errors []FieldError
		want   string
	}{
		{
			name:   "no errors",
			errors: []FieldError{},
			want:   "no validation errors",
		},
		{
			name: "single error",
			errors: []FieldError{
				{Field: "name", Message: "required"},
			},
			want: "validation errors: name: required",
		},
		{
			name: "multiple errors",
			errors: []FieldError{
				{Field: "name", Message: "required"},
				{Field: "email", Message: "invalid format"},
			},
			want: "validation errors: name: required, email: invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ve := ValidationError{Errors: tt.errors}
			if got := ve.Error(); got != tt.want {
				t.Errorf("ValidationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultEntityFactory_Create(t *testing.T) {
	factory := DefaultEntityFactory[*TestEntity]{}
	entity := factory.Create()

	// Test that we get a valid, zero-initialized TestEntity
	if reflect.TypeOf(entity) != reflect.TypeOf(&TestEntity{}) {
		t.Errorf("Expected type *TestEntity, got %T", entity)
	}
}

func TestDefaultEntityPatcher_Patch(t *testing.T) {
	fields := []formui.Field{
		formui.Text("Name", "Name").Build(),
		formui.Text("Email", "Email").Build(),
		formui.Text("Age", "Age").Build(),
		formui.Text("Active", "Active").Build(),
	}

	tests := []struct {
		name          string
		formData      map[string]string
		wantEntity    *TestEntity
		wantFieldErrs int
	}{
		{
			name: "valid data",
			formData: map[string]string{
				"Name":   "John Doe",
				"Email":  "john@example.com",
				"Age":    "30",
				"Active": "true",
			},
			wantEntity: &TestEntity{
				Name:   "John Doe",
				Email:  "john@example.com",
				Age:    30,
				Active: true,
			},
			wantFieldErrs: 0,
		},
		{
			name: "invalid age",
			formData: map[string]string{
				"Name":   "John Doe",
				"Email":  "john@example.com",
				"Age":    "not-a-number",
				"Active": "true",
			},
			wantEntity: &TestEntity{
				Name:   "John Doe",
				Email:  "john@example.com",
				Active: true,
			},
			wantFieldErrs: 1,
		},
		{
			name: "empty values",
			formData: map[string]string{
				"Name":  "",
				"Email": "",
			},
			wantEntity:    &TestEntity{},
			wantFieldErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &TestEntity{}
			patcher := DefaultEntityPatcher[*TestEntity]{}
			result, valErrs := patcher.Patch(entity, tt.formData, fields)

			if len(valErrs.Errors) != tt.wantFieldErrs {
				t.Errorf("Expected %d validation errors, got %d", tt.wantFieldErrs, len(valErrs.Errors))
			}

			// Check that the expected fields were set correctly
			if tt.wantFieldErrs == 0 {
				if result.Name != tt.wantEntity.Name {
					t.Errorf("Expected Name=%s, got %s", tt.wantEntity.Name, result.Name)
				}
				if result.Email != tt.wantEntity.Email {
					t.Errorf("Expected Email=%s, got %s", tt.wantEntity.Email, result.Email)
				}
				if tt.formData["Age"] != "" && result.Age != tt.wantEntity.Age {
					t.Errorf("Expected Age=%d, got %d", tt.wantEntity.Age, result.Age)
				}
				if tt.formData["Active"] != "" && result.Active != tt.wantEntity.Active {
					t.Errorf("Expected Active=%v, got %v", tt.wantEntity.Active, result.Active)
				}
			}
		})
	}
}

func TestSetFieldValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		field     interface{}
		expected  interface{}
		wantError bool
	}{
		{
			name:      "string value",
			value:     "test",
			field:     "",
			expected:  "test",
			wantError: false,
		},
		{
			name:      "int value",
			value:     "123",
			field:     int(0),
			expected:  int(123),
			wantError: false,
		},
		{
			name:      "invalid int",
			value:     "abc",
			field:     int(0),
			expected:  int(0),
			wantError: true,
		},
		{
			name:      "bool value true",
			value:     "true",
			field:     false,
			expected:  true,
			wantError: false,
		},
		{
			name:      "bool value false",
			value:     "false",
			field:     true,
			expected:  false,
			wantError: false,
		},
		{
			name:      "invalid bool",
			value:     "not-a-bool",
			field:     false,
			expected:  false,
			wantError: true,
		},
		{
			name:      "float value",
			value:     "123.45",
			field:     float64(0),
			expected:  float64(123.45),
			wantError: false,
		},
		{
			name:      "empty value",
			value:     "",
			field:     "original",
			expected:  "original",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := reflect.ValueOf(tt.field)
			if rv.Kind() != reflect.Ptr {
				// Create a settable reflect.Value
				v := reflect.New(reflect.TypeOf(tt.field)).Elem()
				v.Set(reflect.ValueOf(tt.field))
				err := setFieldValue(v, tt.value)

				if (err != nil) != tt.wantError {
					t.Errorf("setFieldValue() error = %v, wantError %v", err, tt.wantError)
					return
				}

				if !tt.wantError {
					got := v.Interface()
					if !reflect.DeepEqual(got, tt.expected) {
						t.Errorf("setFieldValue() = %v, want %v", got, tt.expected)
					}
				}
			}
		})
	}
}

func TestServiceCRUD(t *testing.T) {
	store := newMockDataStore()
	fields := []formui.Field{
		formui.Text("Name", "Name").Build(),
		formui.Text("Email", "Email").Build(),
	}
	service := NewService[*TestEntity, int](
		"Test",
		"/test",
		"ID",
		store,
		fields,
	)

	ctx := context.Background()

	t.Run("CreateEntity", func(t *testing.T) {
		formData := map[string]string{
			"Name":  "John Doe",
			"Email": "john@example.com",
		}

		id, err := service.CreateEntity(ctx, formData)
		if err != nil {
			t.Fatalf("CreateEntity() error = %v", err)
		}

		if id != 1 {
			t.Errorf("Expected ID=1, got %d", id)
		}

		// Verify entity was created
		entity, err := service.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if entity.Name != "John Doe" || entity.Email != "john@example.com" {
			t.Errorf("Entity values not set correctly: %+v", entity)
		}
	})

	t.Run("UpdateEntity", func(t *testing.T) {
		formData := map[string]string{
			"Name":  "Jane Doe",
			"Email": "jane@example.com",
		}

		err := service.UpdateEntity(ctx, 1, formData)
		if err != nil {
			t.Fatalf("UpdateEntity() error = %v", err)
		}

		// Verify entity was updated
		entity, err := service.Get(ctx, 1)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if entity.Name != "Jane Doe" || entity.Email != "jane@example.com" {
			t.Errorf("Entity values not updated correctly: %+v", entity)
		}
	})

	t.Run("List", func(t *testing.T) {
		// Add another entity
		formData := map[string]string{
			"Name":  "Bob Smith",
			"Email": "bob@example.com",
		}
		_, err := service.CreateEntity(ctx, formData)
		if err != nil {
			t.Fatalf("CreateEntity() error = %v", err)
		}

		entities, err := service.List(ctx, FindParams{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(entities) != 2 {
			t.Errorf("Expected 2 entities, got %d", len(entities))
		}
	})

	t.Run("DeleteEntity", func(t *testing.T) {
		err := service.DeleteEntity(ctx, 1)
		if err != nil {
			t.Fatalf("DeleteEntity() error = %v", err)
		}

		// Verify entity was deleted
		_, err = service.Get(ctx, 1)
		if err == nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}

		// List should return only one entity now
		entities, err := service.List(ctx, FindParams{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(entities) != 1 {
			t.Errorf("Expected 1 entity, got %d", len(entities))
		}
	})

	t.Run("ValidationError", func(t *testing.T) {
		// Add a validator that always fails
		service.ModelValidators = []ModelLevelValidator[*TestEntity]{
			&mockValidator{shouldFail: true, errMsg: "validation failed"},
		}

		formData := map[string]string{
			"Name":  "Will Fail",
			"Email": "fail@example.com",
		}

		_, err := service.CreateEntity(ctx, formData)
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Errorf("Expected ErrValidation, got %v", err)
		}

		// Reset validators
		service.ModelValidators = nil
	})
}

func TestSchemaOptions(t *testing.T) {
	store := newMockDataStore()
	fields := []formui.Field{
		formui.Text("Name", "Name").Build(),
		formui.Text("Email", "Email").Build(),
	}

	t.Run("WithFields", func(t *testing.T) {
		schema := NewSchema[*TestEntity, int](
			"Test", "/test", store,
			WithFields[*TestEntity, int](fields...),
		)

		if len(schema.Service.Fields) != len(fields) {
			t.Errorf("Expected %d fields, got %d", len(fields), len(schema.Service.Fields))
		}
	})

	t.Run("WithModelValidators", func(t *testing.T) {
		validator := &mockValidator{}
		schema := NewSchema[*TestEntity, int](
			"Test", "/test", store,
			WithModelValidators[*TestEntity, int](validator),
		)

		if len(schema.Service.ModelValidators) != 1 {
			t.Errorf("Expected 1 validator, got %d", len(schema.Service.ModelValidators))
		}
	})

	t.Run("WithMiddlewares", func(t *testing.T) {
		middleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}

		schema := NewSchema[*TestEntity, int](
			"Test", "/test", store,
			WithMiddlewares[*TestEntity, int](middleware),
		)

		if len(schema.middlewares) != 1 {
			t.Errorf("Expected 1 middleware, got %d", len(schema.middlewares))
		}
	})

	t.Run("WithGetPrimaryKey", func(t *testing.T) {
		schema := NewSchema[*TestEntity, int](
			"Test", "/test", store,
			WithGetPrimaryKey[*TestEntity, int](func() string {
				return "CustomID"
			}),
		)

		if schema.Service.IDField != "CustomID" {
			t.Errorf("Expected IDField='CustomID', got '%s'", schema.Service.IDField)
		}
	})
}

func TestExtract(t *testing.T) {
	store := newMockDataStore()
	fields := []formui.Field{
		formui.Text("Name", "Name").Build(),
		formui.Text("Email", "Email").Build(),
		formui.Text("Age", "Age").Build(),
	}
	service := NewService[*TestEntity, int](
		"Test",
		"/test",
		"ID",
		store,
		fields,
	)

	entity := &TestEntity{
		ID:    1,
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	result := service.Extract(entity)

	if result["Name"] != "John Doe" {
		t.Errorf("Expected Name='John Doe', got '%s'", result["Name"])
	}
	if result["Email"] != "john@example.com" {
		t.Errorf("Expected Email='john@example.com', got '%s'", result["Email"])
	}
	if result["Age"] != "30" {
		t.Errorf("Expected Age='30', got '%s'", result["Age"])
	}
}

func TestParseFormToMap(t *testing.T) {
	formValues := url.Values{}
	formValues.Add("field1", "value1")
	formValues.Add("field2", "value2")
	formValues.Add("multi", "value3")
	formValues.Add("multi", "value4") // This should be ignored

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(formValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	result, err := parseFormToMap(req)
	if err != nil {
		t.Fatalf("parseFormToMap() error = %v", err)
	}

	if result["field1"] != "value1" {
		t.Errorf("Expected field1='value1', got '%s'", result["field1"])
	}
	if result["field2"] != "value2" {
		t.Errorf("Expected field2='value2', got '%s'", result["field2"])
	}
	if result["multi"] != "value3" {
		t.Errorf("Expected multi='value3', got '%s'", result["multi"])
	}
}

func TestHTTPHandlers(t *testing.T) {
	store := newMockDataStore()
	fields := []formui.Field{
		formui.Text("Name", "Name").Build(),
		formui.Text("Email", "Email").Build(),
	}

	// Pre-populate the store
	ctx := context.Background()
	_, _ = store.Create(ctx, &TestEntity{Name: "John Doe", Email: "john@example.com"})

	// Create custom renderer for testing
	renderer := func(w http.ResponseWriter, r *http.Request, c templ.Component, options ...func(*templ.ComponentHandler)) {
		w.WriteHeader(http.StatusOK)
	}

	schema := NewSchema[*TestEntity, int](
		"Test", "/test", store,
		WithFields[*TestEntity, int](fields...),
		WithRenderer[*TestEntity, int](renderer),
	)

	// Create router and register schema
	router := mux.NewRouter()
	schema.Register(router)

	t.Run("listHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("newHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test/new", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("createHandler", func(t *testing.T) {
		formValues := url.Values{}
		formValues.Add("Name", "Jane Doe")
		formValues.Add("Email", "jane@example.com")

		req := httptest.NewRequest(http.MethodPost, "/test/", strings.NewReader(formValues.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should redirect after successful creation
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status code %d, got %d", http.StatusSeeOther, w.Code)
		}

		// Verify entity was created in store
		entities, _ := store.List(ctx, FindParams{})
		if len(entities) != 2 {
			t.Errorf("Expected 2 entities in store, got %d", len(entities))
		}
	})

	t.Run("editHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test/1/edit", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("updateHandler", func(t *testing.T) {
		formValues := url.Values{}
		formValues.Add("Name", "John Updated")
		formValues.Add("Email", "john.updated@example.com")

		req := httptest.NewRequest(http.MethodPut, "/test/1", strings.NewReader(formValues.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should redirect after successful update
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status code %d, got %d", http.StatusSeeOther, w.Code)
		}

		// Verify entity was updated
		entity, _ := store.Get(ctx, 1)
		if entity.Name != "John Updated" || entity.Email != "john.updated@example.com" {
			t.Errorf("Entity not updated correctly: %+v", entity)
		}
	})

	t.Run("deleteHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/test/1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should redirect after successful deletion
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status code %d, got %d", http.StatusSeeOther, w.Code)
		}

		// Verify entity was deleted
		entities, _ := store.List(ctx, FindParams{})
		if len(entities) != 1 {
			t.Errorf("Expected 1 entity in store, got %d", len(entities))
		}
	})

	t.Run("nonexistent entity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test/999/edit", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}
