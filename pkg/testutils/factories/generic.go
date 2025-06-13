package factories

import (
	"fmt"
	"math/rand"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// Generic provides a fluent API for creating test data
type Generic struct {
	values map[string]interface{}
}

// New creates a new generic factory
func New() *Generic {
	return &Generic{
		values: make(map[string]interface{}),
	}
}

// Set adds a field value
func (g *Generic) Set(field string, value interface{}) *Generic {
	g.values[field] = value
	return g
}

// SetIf conditionally sets a field
func (g *Generic) SetIf(condition bool, field string, value interface{}) *Generic {
	if condition {
		g.values[field] = value
	}
	return g
}

// Merge merges values from another generic
func (g *Generic) Merge(other *Generic) *Generic {
	for k, v := range other.values {
		g.values[k] = v
	}
	return g
}

// ToForm converts to URL values for form submission
func (g *Generic) ToForm() url.Values {
	values := url.Values{}
	for k, v := range g.values {
		values.Set(k, fmt.Sprintf("%v", v))
	}
	return values
}

// ToMap returns the raw map
func (g *Generic) ToMap() map[string]interface{} {
	return g.values
}

// Fill fills a struct with the factory values
func (g *Generic) Fill(target interface{}) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	targetValue = targetValue.Elem()
	targetType := targetValue.Type()

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		if value, ok := g.values[field.Name]; ok {
			fieldValue := targetValue.Field(i)
			if fieldValue.CanSet() {
				fieldValue.Set(reflect.ValueOf(value))
			}
		}
	}

	return nil
}

// Common preset factories

// WithTimestamps adds created/updated timestamps
func WithTimestamps() *Generic {
	now := time.Now()
	return New().
		Set("CreatedAt", now).
		Set("UpdatedAt", now)
}

// WithTenant adds tenant ID
func WithTenant(tenantID uuid.UUID) *Generic {
	return New().Set("TenantID", tenantID)
}

// WithMoney creates money-related fields
func WithMoney(amount float64, currency string) *Generic {
	return New().
		Set("Amount", amount).
		Set("CurrencyCode", currency).
		Set("Balance", money.NewFromFloat(amount, currency))
}

// Random data generators

// RandomString generates a random string
func RandomString(prefix string, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return prefix + string(b)
}

// RandomEmail generates a random email
func RandomEmail() string {
	return fmt.Sprintf("test-%s@example.com", RandomString("", 8))
}

// RandomPhone generates a random phone number
func RandomPhone() string {
	return fmt.Sprintf("+1555%07d", rand.Intn(10000000))
}

// RandomAmount generates a random money amount
func RandomAmount(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

// FormBuilder helps build form data with validation
type FormBuilder struct {
	values url.Values
	errors []string
}

// NewForm creates a new form builder
func NewForm() *FormBuilder {
	return &FormBuilder{
		values: url.Values{},
		errors: []string{},
	}
}

// Add adds a field if value is not empty
func (fb *FormBuilder) Add(field string, value interface{}) *FormBuilder {
	if value != nil && value != "" {
		fb.values.Set(field, fmt.Sprintf("%v", value))
	}
	return fb
}

// AddRequired adds a required field
func (fb *FormBuilder) AddRequired(field string, value interface{}) *FormBuilder {
	if value == nil || value == "" {
		fb.errors = append(fb.errors, fmt.Sprintf("%s is required", field))
	} else {
		fb.values.Set(field, fmt.Sprintf("%v", value))
	}
	return fb
}

// Build returns the form values or error
func (fb *FormBuilder) Build() (url.Values, error) {
	if len(fb.errors) > 0 {
		return nil, fmt.Errorf("form validation failed: %s", strings.Join(fb.errors, ", "))
	}
	return fb.values, nil
}
