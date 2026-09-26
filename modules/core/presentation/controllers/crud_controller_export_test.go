package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The export fixture is a dictionary as the SDK's consumers build them: a
// hidden key, a plain column, a multilingual name, a relation rendered through
// a select, and a field no file may carry.
type exportEntity struct {
	ID        uuid.UUID
	Code      string
	Name      models.MultiLang
	StatusID  int
	Secret    string
	CreatedAt time.Time
}

const (
	exportStatusActive   = 1
	exportStatusArchived = 2
)

func newExportFields() crud.Fields {
	statusField := crud.NewSelectField("status_id").
		SetValueType(crud.IntFieldType).
		SetOptions([]crud.SelectOption{
			{Value: exportStatusActive, Label: "Active"},
			{Value: exportStatusArchived, Label: "Archived"},
		})

	return crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey(), crud.WithReadonly(), crud.WithHidden()),
		crud.NewStringField("code", crud.WithSearchable()),
		crud.NewJSONField(
			"name",
			crud.JSONFieldConfig[models.MultiLang]{Validator: func(models.MultiLang) error { return nil }},
			crud.WithRenderer("multilang"),
		),
		statusField,
		// Hidden from the table, and still part of the record: the export is
		// expected to carry it.
		crud.NewStringField("note", crud.WithHidden()),
		crud.NewStringField("secret"),
		crud.NewTimestampField("created_at", crud.WithReadonly(), crud.WithHidden()),
	})
}

type exportMapper struct {
	fields crud.Fields
}

func (m *exportMapper) ToEntities(_ context.Context, _ ...[]crud.FieldValue) ([]exportEntity, error) {
	return nil, nil
}

func (m *exportMapper) ToFieldValuesList(_ context.Context, entities ...exportEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))
	for i, entity := range entities {
		values, err := m.fields.FieldValues(map[string]any{
			"id":         entity.ID,
			"code":       entity.Code,
			"name":       entity.Name,
			"status_id":  entity.StatusID,
			"note":       "note-" + entity.Code,
			"secret":     entity.Secret,
			"created_at": entity.CreatedAt,
		})
		if err != nil {
			return nil, err
		}
		result[i] = values
	}
	return result, nil
}

// exportService is an ordered in-memory service: the export pages through it,
// so the order has to be stable.
type exportService struct {
	entities  []exportEntity
	listCalls int
	sorts     []crud.SortBy
}

func (s *exportService) GetAll(context.Context) ([]exportEntity, error) { return s.entities, nil }

func (s *exportService) Get(context.Context, crud.FieldValue) (exportEntity, error) {
	return exportEntity{}, nil
}

func (s *exportService) Exists(context.Context, crud.FieldValue) (bool, error) { return false, nil }

func (s *exportService) Count(_ context.Context, params *crud.FindParams) (int64, error) {
	return int64(len(s.filtered(params))), nil
}

func (s *exportService) List(_ context.Context, params *crud.FindParams) ([]exportEntity, error) {
	s.listCalls++
	if params != nil {
		s.sorts = append(s.sorts, params.SortBy)
	}
	matching := s.filtered(params)
	if params == nil {
		return matching, nil
	}
	if params.Offset >= len(matching) {
		return nil, nil
	}
	matching = matching[params.Offset:]
	if params.Limit > 0 && params.Limit < len(matching) {
		matching = matching[:params.Limit]
	}
	return matching, nil
}

func (s *exportService) filtered(params *crud.FindParams) []exportEntity {
	if params == nil || params.Query == "" {
		return s.entities
	}
	matching := make([]exportEntity, 0, len(s.entities))
	for _, entity := range s.entities {
		if strings.Contains(entity.Code, params.Query) {
			matching = append(matching, entity)
		}
	}
	return matching
}

func (s *exportService) Save(_ context.Context, entity exportEntity) (exportEntity, error) {
	return entity, nil
}

func (s *exportService) Delete(context.Context, crud.FieldValue) (exportEntity, error) {
	return exportEntity{}, nil
}

type exportBuilder struct {
	schema  crud.Schema[exportEntity]
	service crud.Service[exportEntity]
}

func (b *exportBuilder) Schema() crud.Schema[exportEntity]         { return b.schema }
func (b *exportBuilder) Service() crud.Service[exportEntity]       { return b.service }
func (b *exportBuilder) Repository() crud.Repository[exportEntity] { return nil }

func newExportBuilder(service crud.Service[exportEntity]) crud.Builder[exportEntity] {
	fields := newExportFields()
	return &exportBuilder{
		schema:  crud.NewSchema("dictionary", fields, &exportMapper{fields: fields}),
		service: service,
	}
}

func newExportSuite(t *testing.T) *itf.Suite {
	t.Helper()
	testUser := user.New(
		"Test",
		"User",
		internet.MustParseEmail("export@example.com"),
		user.UILanguageEN,
	)
	return itf.NewSuiteBuilder(t).
		WithComponents(core.NewComponent(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		})).
		Build().
		AsUser(testUser)
}

func seededExportService(count int) *exportService {
	entities := make([]exportEntity, 0, count)
	for i := 0; i < count; i++ {
		entities = append(entities, exportEntity{
			ID:        uuid.New(),
			Code:      fmt.Sprintf("CODE-%03d", i),
			Name:      models.NewMultiLang(fmt.Sprintf("Nomi %d", i), fmt.Sprintf("Имя %d", i), fmt.Sprintf("Name %d", i)),
			StatusID:  exportStatusActive,
			Secret:    fmt.Sprintf("token-%d", i),
			CreatedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
		})
	}
	return &exportService{entities: entities}
}

// csvRecords splits an exported CSV body, dropping the UTF-8 BOM.
func csvRecords(t *testing.T, body string) [][]string {
	t.Helper()
	body = strings.TrimPrefix(body, string([]byte{0xEF, 0xBB, 0xBF}))
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(body), "\r\n", "\n"), "\n")
	records := make([][]string, 0, len(lines))
	for _, line := range lines {
		records = append(records, strings.Split(line, ","))
	}
	return records
}

// TestCrudControllerExportWritesEveryRowOfTheFilteredSet pins what an export
// is: the whole result set of the current search, not the page on screen, and
// the record's own fields rather than the table's columns.
//
// Falsely green if the fixture held one page worth of rows, or if every field
// were visible: then "all pages" and "hidden fields too" would be free.
func TestCrudControllerExportWritesEveryRowOfTheFilteredSet(t *testing.T) {
	suite := newExportSuite(t)
	service := seededExportService(120)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(service),
		controllers.WithExport[exportEntity]("secret"),
	)
	suite.Register(controller)

	body := suite.GET("/dictionary/export?format=csv").
		Expect(t).
		Status(http.StatusOK).
		Body()

	records := csvRecords(t, body)
	require.Len(t, records, 121, "header plus every seeded row, not one page")

	header := records[0]
	assert.Contains(t, header, "note", "a field hidden from the table still belongs to the record")
	assert.NotContains(t, header, "secret", "an excluded field never reaches the file")
	for _, column := range header {
		assert.NotEqual(t, "", column)
	}
	assert.NotContains(t, body, "token-0", "excluded values are gone with their column")
}

// TestCrudControllerExportKeepsEveryLanguageAndBothSidesOfARelation covers the
// two shapes a single column cannot hold: a multilingual value and a relation.
//
// Falsely green if the fixture were filled in one language only.
func TestCrudControllerExportKeepsEveryLanguageAndBothSidesOfARelation(t *testing.T) {
	suite := newExportSuite(t)
	service := seededExportService(1)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(service),
		controllers.WithExport[exportEntity](),
	)
	suite.Register(controller)

	body := suite.GET("/dictionary/export?format=csv").
		Expect(t).
		Status(http.StatusOK).
		Body()

	records := csvRecords(t, body)
	require.Len(t, records, 2)
	header, row := records[0], records[1]

	for _, locale := range []string{"ru", "en", "uz"} {
		assert.Contains(t, header, fmt.Sprintf("name (%s)", locale))
	}
	assert.Contains(t, row, "Имя 0")
	assert.Contains(t, row, "Name 0")
	assert.Contains(t, row, "Nomi 0")

	assert.Contains(t, header, "status_id")
	assert.Contains(t, header, "status_id ID")
	assert.Contains(t, row, "Active", "the relation is written as its label")
	assert.Contains(t, row, "1", "and as the stored value beside it")
}

// TestCrudControllerExportNeutralizesFormulasAndHonoursSearch pins the two
// things the file shares with the screen: the search that produced it, and
// values that stay values when a spreadsheet opens them.
func TestCrudControllerExportNeutralizesFormulasAndHonoursSearch(t *testing.T) {
	suite := newExportSuite(t)
	service := seededExportService(3)
	service.entities[0].Code = "=cmd|'/c calc'!A1"
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(service),
		controllers.WithExport[exportEntity](),
	)
	suite.Register(controller)

	body := suite.GET("/dictionary/export?format=csv").
		Expect(t).
		Status(http.StatusOK).
		Body()
	assert.NotContains(t, body, "\"=cmd", "a value that looks like a formula must not open as one")
	assert.Contains(t, body, "cmd|'/c calc'!A1")

	filtered := suite.GET("/dictionary/export?format=csv&Search=CODE-002").
		Expect(t).
		Status(http.StatusOK).
		Body()
	records := csvRecords(t, filtered)
	assert.Len(t, records, 2, "the export carries the searched rows and nothing else")
	assert.Contains(t, filtered, "CODE-002")
	assert.NotContains(t, filtered, "CODE-001")
}

// TestCrudControllerExportRefusesAnUnsupportedFormat: an unknown format is a
// bad request, never a file in a format nobody asked for.
func TestCrudControllerExportRefusesAnUnsupportedFormat(t *testing.T) {
	suite := newExportSuite(t)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(seededExportService(1)),
		controllers.WithExport[exportEntity](),
	)
	suite.Register(controller)

	suite.GET("/dictionary/export?format=json").Expect(t).Status(http.StatusBadRequest)
	suite.GET("/dictionary/export").Expect(t).Status(http.StatusBadRequest)
}

// TestCrudControllerWithoutExportHasNoRoute keeps the option an opt-in: a
// controller that was never asked to export does not answer /export at all.
func TestCrudControllerWithoutExportHasNoRoute(t *testing.T) {
	suite := newExportSuite(t)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(seededExportService(1)),
	)
	suite.Register(controller)

	// 405, not 404: the path is taken by the update and delete routes of
	// /{id}, so a GET there is a method that route does not answer.
	suite.GET("/dictionary/export?format=csv").Expect(t).Status(http.StatusMethodNotAllowed)
}

// TestCrudControllerExportOffersBothFormatsOnTheList proves the control is on
// the page, so the route is reachable without knowing the URL.
func TestCrudControllerExportOffersBothFormatsOnTheList(t *testing.T) {
	suite := newExportSuite(t)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(seededExportService(2)),
		controllers.WithExport[exportEntity](),
	)
	suite.Register(controller)

	body := suite.GET("/dictionary").Expect(t).Status(http.StatusOK).Body()
	assert.Contains(t, body, "/dictionary/export")
	formats := []string{"excel", "csv"}
	sort.Strings(formats)
	for _, format := range formats {
		assert.Contains(t, body, format)
	}
}

// TestCrudControllerExportProducesAnExcelWorkbook: the XLSX branch answers with
// a real workbook, not an empty body with spreadsheet headers.
func TestCrudControllerExportProducesAnExcelWorkbook(t *testing.T) {
	suite := newExportSuite(t)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(seededExportService(3)),
		controllers.WithExport[exportEntity](),
	)
	suite.Register(controller)

	resp := suite.GET("/dictionary/export?format=excel").Expect(t).Status(http.StatusOK)
	body := resp.Body()
	assert.True(t, strings.HasPrefix(body, "PK"), "a xlsx file is a zip container")
	assert.Greater(t, len(body), 1024)
}

// TestCrudControllerExportPagesOverATotalOrder pins what makes batched OFFSET
// reads sound: every batch is ordered, and the primary key breaks ties, so no
// row can move between batches and be written twice or not at all.
//
// Falsely green if the fixture fit in one batch — then no second read would
// ever depend on the order of the first. 1200 rows are three batches.
func TestCrudControllerExportPagesOverATotalOrder(t *testing.T) {
	suite := newExportSuite(t)
	service := seededExportService(1200)
	controller := controllers.NewCrudController[exportEntity](
		"/dictionary",
		newExportBuilder(service),
		controllers.WithExport[exportEntity](),
	)
	suite.Register(controller)

	suite.GET("/dictionary/export?format=csv").Expect(t).Status(http.StatusOK)
	require.Len(t, service.sorts, 3, "1200 rows are read in three batches")
	for _, sortBy := range service.sorts {
		require.NotEmpty(t, sortBy.Fields)
		assert.Equal(t, "id", sortBy.Fields[len(sortBy.Fields)-1].Field, "the key breaks every tie")
	}

	service.sorts = nil
	suite.GET("/dictionary/export?format=csv&sort=code&order=desc").Expect(t).Status(http.StatusOK)
	require.NotEmpty(t, service.sorts)
	fields := service.sorts[0].Fields
	require.Len(t, fields, 2)
	assert.Equal(t, "code", fields[0].Field, "the order the reader chose comes first")
	assert.False(t, fields[0].Ascending)
	assert.Equal(t, "id", fields[1].Field)
}
