package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/components/export"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// crudExportBatchSize is how many records one service call fetches while the
// export walks the whole result set. The page size of the list is irrelevant
// here: an export covers every row the current search and sorting select.
const crudExportBatchSize = 500

// crudExportDateTimeFormat keeps CSV and XLSX showing the same instant the same
// way; excel.DefaultOptions() applies it to XLSX, CSVOptions has to be told.
const crudExportDateTimeFormat = "2006-01-02 15:04:05"

// multilangRendererType is the renderer a schema declares for a field that
// holds one text per language; see WithMultiLangRenderer.
const multilangRendererType = "multilang"

// WithExport adds CSV and XLSX export to the list page.
//
// The file carries the entity's own fields — including the ones hidden from the
// table — because a column set chosen for reading on screen is not the record.
// Rows are those of the current search and sorting, across all pages, and the
// export is behind the same read permission as the list itself.
//
// Field names passed in excludedFields never reach the file. That is the place
// for secrets: a field the schema carries for the application but that no
// export may reveal.
func WithExport[TEntity any](excludedFields ...string) CrudOption[TEntity] {
	return func(c *CrudController[TEntity]) {
		c.enableExport = true
		c.exportExcluded = make(map[string]struct{}, len(excludedFields))
		for _, name := range excludedFields {
			c.exportExcluded[name] = struct{}{}
		}
	}
}

// exportColumn is one column of the export file: where its value comes from and
// how it is read out of a record.
type exportColumn struct {
	field crud.Field
	// header is the localized column title.
	header string
	// locale is set for one language of a multilingual field, empty otherwise.
	locale string
	// rawID marks the column that carries the raw stored value of a relation
	// next to the human-readable one.
	rawID bool
}

// Export streams the whole filtered result set as CSV or XLSX.
func (c *CrudController[TEntity]) Export(w http.ResponseWriter, r *http.Request) {
	if c.accessDenied(w, r, c.readPerm) {
		return
	}
	ctx := r.Context()

	format, ok := export.GetExportFormat(r)
	if !ok || (format != export.ExportFormatExcel && format != export.ExportFormatCSV) {
		// An unsupported format is a bad request, not a silent fallback to one
		// of the two: the caller would save a file it did not ask for.
		errorMsg, _ := c.localize(ctx, "Errors.InvalidExportFormat", "Unsupported export format")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	entities, err := c.exportEntities(ctx, r)
	if err != nil {
		log.Printf("[CrudController.Export] Failed to list entities: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRetrieve, "Failed to retrieve data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	rowValues, err := c.exportFieldValues(ctx, entities)
	if err != nil {
		log.Printf("[CrudController.Export] Failed to map entities: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRetrieve, "Failed to retrieve data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	columns := c.exportColumns(ctx, rowValues)
	headers := make([]string, 0, len(columns))
	for _, col := range columns {
		headers = append(headers, col.header)
	}
	rows := make([][]interface{}, 0, len(rowValues))
	for _, values := range rowValues {
		rows = append(rows, c.exportRow(ctx, columns, values))
	}

	filename := c.exportFilename(format)
	switch format {
	case export.ExportFormatCSV:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)

		opts := excel.DefaultCSVOptions()
		opts.DateTimeFormat = crudExportDateTimeFormat
		// Dictionary values are user-entered: the SDK writer neutralizes every
		// string cell so none of them opens as a spreadsheet formula.
		writer, err := excel.NewCSVWriter(w, opts)
		if err != nil {
			log.Printf("[CrudController.Export] Failed to start CSV: %v", err)
			return
		}
		if err := writer.WriteHeader(headers); err != nil {
			log.Printf("[CrudController.Export] Failed to write CSV header: %v", err)
			return
		}
		for _, row := range rows {
			if err := writer.WriteRow(row); err != nil {
				log.Printf("[CrudController.Export] Failed to write CSV row: %v", err)
				return
			}
		}
		if err := writer.Flush(); err != nil {
			log.Printf("[CrudController.Export] Failed to flush CSV: %v", err)
		}
	default:
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)

		opts := excel.DefaultOptions()
		opts.DateTimeFormat = crudExportDateTimeFormat
		exporter := excel.NewExcelExporter(opts, excel.DefaultStyleOptions())
		datasource := excel.NewFunctionDataSource(headers, func(context.Context) ([][]interface{}, error) {
			return rows, nil
		}).WithSheetName(c.exportSheetName())
		if err := exporter.ExportToWriter(ctx, w, datasource); err != nil {
			log.Printf("[CrudController.Export] Failed to write XLSX: %v", err)
		}
	}
}

// exportEntities walks the full result set of the current search and sorting.
func (c *CrudController[TEntity]) exportEntities(ctx context.Context, r *http.Request) ([]TEntity, error) {
	params := &crud.FindParams{Limit: crudExportBatchSize}
	if searchQuery := r.URL.Query().Get("Search"); searchQuery != "" {
		params.Query = searchQuery
	}
	if sortField := table.UseSortQuery(r); sortField != "" {
		params.SortBy = crud.SortBy{
			Fields: []repo.SortByField[string]{
				{Field: sortField, Ascending: table.UseOrderQuery(r) == "asc"},
			},
		}
	}

	entities := make([]TEntity, 0)
	for {
		batch, err := c.service.List(ctx, params)
		if err != nil {
			return nil, err
		}
		entities = append(entities, batch...)
		if len(batch) < crudExportBatchSize {
			return entities, nil
		}
		params.Offset += len(batch)
	}
}

func (c *CrudController[TEntity]) exportFieldValues(ctx context.Context, entities []TEntity) ([][]crud.FieldValue, error) {
	rows := make([][]crud.FieldValue, 0, len(entities))
	for _, entity := range entities {
		fieldValues, err := c.schema.Mapper().ToFieldValues(ctx, entity)
		if err != nil {
			return nil, err
		}
		rows = append(rows, fieldValues)
	}
	return rows, nil
}

// exportColumns lays out the file: every schema field that is not excluded, a
// column per language of a multilingual field, and the stored value of a
// relation beside its label.
func (c *CrudController[TEntity]) exportColumns(ctx context.Context, rows [][]crud.FieldValue) []exportColumn {
	columns := make([]exportColumn, 0, len(c.schema.Fields().Fields()))
	for _, field := range c.schema.Fields().Fields() {
		if _, excluded := c.exportExcluded[field.Name()]; excluded {
			continue
		}
		label := c.exportFieldLabel(ctx, field)

		if locales := exportFieldLocales(field, rows); len(locales) > 0 {
			for _, locale := range locales {
				columns = append(columns, exportColumn{
					field:  field,
					header: fmt.Sprintf("%s (%s)", label, locale),
					locale: locale,
				})
			}
			continue
		}

		columns = append(columns, exportColumn{field: field, header: label})
		if _, isSelect := field.(crud.SelectField); isSelect {
			// A relation is worth nothing to the reader as a bare id and worth
			// nothing to a machine as a bare label, so both are written.
			columns = append(columns, exportColumn{
				field:  field,
				header: label + " ID",
				rawID:  true,
			})
		}
	}
	return columns
}

func (c *CrudController[TEntity]) exportRow(ctx context.Context, columns []exportColumn, values []crud.FieldValue) []interface{} {
	byName := make(map[string]crud.FieldValue, len(values))
	for _, value := range values {
		byName[value.Field().Name()] = value
	}

	row := make([]interface{}, 0, len(columns))
	for _, col := range columns {
		value, ok := byName[col.field.Name()]
		if !ok || value.Value() == nil {
			row = append(row, "")
			continue
		}
		row = append(row, c.exportCell(ctx, col, value))
	}
	return row
}

func (c *CrudController[TEntity]) exportCell(ctx context.Context, col exportColumn, value crud.FieldValue) interface{} {
	if col.locale != "" {
		ml, ok := exportMultiLang(value)
		if !ok {
			return ""
		}
		// Get, not GetWithFallback: a column of one language must stay empty
		// rather than repeat another language's text.
		translation, err := ml.Get(col.locale)
		if err != nil {
			return ""
		}
		return translation
	}

	if selectField, ok := col.field.(crud.SelectField); ok && !col.rawID {
		return c.exportSelectLabel(ctx, selectField, value)
	}

	// A time keeps its type so the spreadsheet stores a date, not text.
	if t, ok := value.Value().(time.Time); ok {
		return t
	}
	return c.convertValueToString(value.Value(), col.field.Type())
}

// exportSelectLabel is getSelectFieldLabel without the templ wrapper.
func (c *CrudController[TEntity]) exportSelectLabel(ctx context.Context, selectField crud.SelectField, value crud.FieldValue) string {
	options := selectField.Options()
	if options == nil && selectField.OptionsLoader() != nil {
		options = selectField.OptionsLoader()(ctx)
	}
	for _, opt := range options {
		if c.compareSelectValues(opt.Value, value.Value(), selectField.ValueType()) {
			return opt.Label
		}
	}
	return c.convertValueToString(value.Value(), selectField.ValueType())
}

func (c *CrudController[TEntity]) exportFieldLabel(ctx context.Context, field crud.Field) string {
	localizationKey := field.LocalizationKey()
	if localizationKey == "" {
		localizationKey = fmt.Sprintf("%s.Fields.%s", c.schema.Name(), field.Name())
	}
	label, err := c.localize(ctx, localizationKey, field.Name())
	if err != nil {
		return field.Name()
	}
	return label
}

func (c *CrudController[TEntity]) exportFilename(format export.ExportFormat) string {
	extension := "xlsx"
	if format == export.ExportFormatCSV {
		extension = "csv"
	}
	return fmt.Sprintf("%s_%s.%s", exportSlug(c.schema.Name()), time.Now().Format("20060102_150405"), extension)
}

func (c *CrudController[TEntity]) exportSheetName() string {
	name := c.schema.Name()
	if len(name) > 31 {
		return name[:31]
	}
	return name
}

// exportMultiLang reads a multilingual value. A JSON field stores it as the
// JSON text of its translations, which is why a type assertion alone is not
// enough — the same two steps components/multilang takes when rendering.
func exportMultiLang(value crud.FieldValue) (models.MultiLang, bool) {
	switch v := value.Value().(type) {
	case models.MultiLang:
		return v, v != nil
	case string:
		if v == "" {
			return nil, false
		}
		ml, err := models.MultiLangFromJSON([]byte(v))
		if err != nil {
			return nil, false
		}
		return ml, true
	default:
		return nil, false
	}
}

// exportFieldLocales returns the languages a multilingual field is written in:
// every language its stored translations carry, ordered by the application's
// own language list so the file reads the same way every time, with unexpected
// locale codes kept at the end rather than dropped.
//
// A field the schema does not render as multilingual returns nothing and gets a
// single column, and so does a multilingual field whose rows hold no
// translation at all — there is no language to name a column after.
func exportFieldLocales(field crud.Field, rows [][]crud.FieldValue) []string {
	if field.RendererType() != multilangRendererType {
		return nil
	}

	present := make(map[string]struct{})
	for _, values := range rows {
		for _, value := range values {
			if value.Field().Name() != field.Name() {
				continue
			}
			ml, ok := exportMultiLang(value)
			if !ok {
				continue
			}
			for locale := range ml.GetAll() {
				present[locale] = struct{}{}
			}
		}
	}
	if len(present) == 0 {
		return nil
	}

	locales := make([]string, 0, len(present)+len(intl.SupportedLanguages))
	seen := make(map[string]struct{}, len(present))
	for _, lang := range intl.SupportedLanguages {
		if _, ok := present[lang.Code]; !ok {
			continue
		}
		locales = append(locales, lang.Code)
		seen[lang.Code] = struct{}{}
	}
	extra := make([]string, 0, len(present))
	for locale := range present {
		if _, ok := seen[locale]; !ok {
			extra = append(extra, locale)
		}
	}
	sort.Strings(extra)
	return append(locales, extra...)
}

// exportSlug turns a schema name into a file-name-safe token.
func exportSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "export"
	}
	return slug
}
