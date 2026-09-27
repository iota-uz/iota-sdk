package crud

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FieldDescriptor is the renderer-neutral, allow-listed projection of a CRUD
// field. It intentionally excludes values, rules, callbacks, attributes and
// renderer components.
type FieldDescriptor struct {
	ID             string    `json:"id"`
	TranslationKey string    `json:"translationKey"`
	Type           FieldType `json:"type"`
	Key            bool      `json:"key,omitempty"`
	Readonly       bool      `json:"readonly,omitempty"`
	Searchable     bool      `json:"searchable,omitempty"`
	Sortable       bool      `json:"sortable,omitempty"`
	Hidden         bool      `json:"hidden,omitempty"`
	Virtual        bool      `json:"virtual,omitempty"`
	RendererID     string    `json:"rendererId,omitempty"`
}

// ActionDescriptor states a resource capability without prescribing a UI.
type ActionDescriptor struct {
	ID          string `json:"id"`
	Mutation    bool   `json:"mutation,omitempty"`
	Form        bool   `json:"form,omitempty"`
	Destructive bool   `json:"destructive,omitempty"`
}

// SchemaView is an immutable projection of the fields visible in each common
// CRUD surface. Accessors return copies so callers cannot mutate the catalog.
type SchemaView struct {
	list    []string
	details []string
	form    []string
}

func (v SchemaView) ListFields() []string   { return append([]string(nil), v.list...) }
func (v SchemaView) DetailFields() []string { return append([]string(nil), v.details...) }
func (v SchemaView) FormFields() []string   { return append([]string(nil), v.form...) }
func (v SchemaView) MarshalJSON() ([]byte, error) { //nolint:wrapcheck // json is the public projection boundary.
	return json.Marshal(struct {
		List    []string `json:"list"`
		Details []string `json:"details"`
		Form    []string `json:"form"`
	}{v.ListFields(), v.DetailFields(), v.FormFields()})
}

// ResourceDescriptor is the serializable semantic companion to Schema. It is
// deliberately not a replacement for Schema: persistence and server behavior
// remain on the existing generic contract.
type ResourceDescriptor struct {
	id             string
	translationKey string
	fields         []FieldDescriptor
	view           SchemaView
	actions        []ActionDescriptor
	escapeHatches  []string
}

func (d ResourceDescriptor) ID() string             { return d.id }
func (d ResourceDescriptor) TranslationKey() string { return d.translationKey }
func (d ResourceDescriptor) Fields() []FieldDescriptor {
	return append([]FieldDescriptor(nil), d.fields...)
}
func (d ResourceDescriptor) View() SchemaView {
	return SchemaView{list: d.view.ListFields(), details: d.view.DetailFields(), form: d.view.FormFields()}
}
func (d ResourceDescriptor) Actions() []ActionDescriptor {
	return append([]ActionDescriptor(nil), d.actions...)
}
func (d ResourceDescriptor) EscapeHatchIDs() []string {
	return append([]string(nil), d.escapeHatches...)
}
func (d ResourceDescriptor) MarshalJSON() ([]byte, error) { //nolint:wrapcheck // json is the public projection boundary.
	return json.Marshal(struct {
		ID             string             `json:"id"`
		TranslationKey string             `json:"translationKey"`
		Fields         []FieldDescriptor  `json:"fields"`
		Views          SchemaView         `json:"views"`
		Actions        []ActionDescriptor `json:"actions"`
		EscapeHatches  []string           `json:"escapeHatchIds,omitempty"`
	}{d.id, d.translationKey, d.Fields(), d.View(), d.Actions(), d.EscapeHatchIDs()})
}

type DescriptorOption func(*descriptorOptions)

type descriptorOptions struct {
	translationKey string
	actions        []ActionDescriptor
	escapeHatches  []string
}

func WithDescriptorTranslationKey(key string) DescriptorOption {
	return func(options *descriptorOptions) { options.translationKey = strings.TrimSpace(key) }
}

func WithDescriptorActions(actions ...ActionDescriptor) DescriptorOption {
	return func(options *descriptorOptions) { options.actions = append([]ActionDescriptor(nil), actions...) }
}

func WithDescriptorEscapeHatches(ids ...string) DescriptorOption {
	return func(options *descriptorOptions) { options.escapeHatches = cleanUniqueIDs(ids) }
}

// ProjectSchema creates the renderer-neutral view of an existing schema.
func ProjectSchema[TEntity any](schema Schema[TEntity], opts ...DescriptorOption) (ResourceDescriptor, error) {
	if schema == nil {
		return ResourceDescriptor{}, fmt.Errorf("crud descriptor: schema is required")
	}
	id := strings.TrimSpace(schema.Name())
	if id == "" {
		return ResourceDescriptor{}, fmt.Errorf("crud descriptor: schema name is required")
	}
	options := descriptorOptions{translationKey: id}
	for _, option := range opts {
		if option != nil {
			option(&options)
		}
	}
	fields := schema.Fields().Fields()
	result := ResourceDescriptor{
		id: id, translationKey: options.translationKey,
		fields:        make([]FieldDescriptor, 0, len(fields)),
		actions:       append([]ActionDescriptor(nil), options.actions...),
		escapeHatches: append([]string(nil), options.escapeHatches...),
	}
	for _, field := range fields {
		fieldID := strings.TrimSpace(field.Name())
		if fieldID == "" {
			return ResourceDescriptor{}, fmt.Errorf("crud descriptor %s: field id is required", id)
		}
		translationKey := strings.TrimSpace(field.LocalizationKey())
		if translationKey == "" {
			translationKey = id + ".Fields." + fieldID
		}
		result.fields = append(result.fields, FieldDescriptor{
			ID: fieldID, TranslationKey: translationKey, Type: field.Type(), Key: field.Key(),
			Readonly: field.Readonly(), Searchable: field.Searchable(), Sortable: field.Sortable(),
			Hidden: field.Hidden(), Virtual: field.Virtual(), RendererID: strings.TrimSpace(field.RendererType()),
		})
		if !field.Hidden() {
			result.view.list = append(result.view.list, fieldID)
			result.view.details = append(result.view.details, fieldID)
			if !field.Key() || !field.Readonly() {
				result.view.form = append(result.view.form, fieldID)
			}
		}
	}
	return result, nil
}

func cleanUniqueIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
