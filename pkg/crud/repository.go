package crud

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"strings"
)

type SortBy = repo.SortBy[string]
type Filter = repo.FieldFilter[string]

type FindParams struct {
	Query   string
	Filters []Filter
	Limit   int
	Offset  int
	SortBy  SortBy
}

type Repository[TEntity any] interface {
	GetAll(ctx context.Context) ([]TEntity, error)
	Get(ctx context.Context, value FieldValue) (TEntity, error)
	Exists(ctx context.Context, value FieldValue) (bool, error)
	Count(ctx context.Context, filters *FindParams) (int64, error)
	List(ctx context.Context, params *FindParams) ([]TEntity, error)
	Create(ctx context.Context, values FieldValues) (TEntity, error)
	Update(ctx context.Context, values FieldValues) (TEntity, error)
	Delete(ctx context.Context, value FieldValue) (TEntity, error)
}

func DefaultRepository[TEntity any](
	schema Schema[TEntity],
) Repository[TEntity] {
	// Initialize fieldMap correctly for generic repository
	fieldMap := make(map[string]string)
	for _, f := range schema.Fields().Fields() {
		fieldMap[f.Name()] = f.Name() // Map field name to itself for generic columns
	}
	return &repository[TEntity]{
		schema:   schema,
		fieldMap: fieldMap,
	}
}

type repository[TEntity any] struct {
	schema   Schema[TEntity]
	fieldMap map[string]string
}

func (r *repository[TEntity]) GetAll(ctx context.Context) ([]TEntity, error) {
	query := fmt.Sprintf("SELECT * FROM %s", r.schema.Name())
	return r.queryEntities(ctx, query)
}

func (r *repository[TEntity]) Get(ctx context.Context, value FieldValue) (TEntity, error) {
	var zero TEntity

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = $1",
		r.schema.Name(),
		value.Field().Name(),
	)

	entities, err := r.queryEntities(ctx, query, value.Value())
	if err != nil {
		return zero, errors.Wrap(err, fmt.Sprintf("failed to get entity by %s", value.Field().Name()))
	}
	if len(entities) == 0 {
		return zero, errors.New("entity not found")
	}

	return entities[0], nil
}

func (r *repository[TEntity]) Exists(ctx context.Context, value FieldValue) (bool, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction")
	}

	base := fmt.Sprintf(
		"SELECT 1 FROM %s WHERE %s = $1",
		r.schema.Name(),
		value.Field().Name(),
	)
	query := repo.Exists(base)

	exists := false
	if err := tx.QueryRow(ctx, query, value.Value()).Scan(&exists); err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to check if %s exists", value.Field().Name()))
	}
	return exists, nil
}

func (r *repository[TEntity]) Count(ctx context.Context, params *FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	whereClauses, args, err := r.buildFilters(params)
	if err != nil {
		return 0, errors.Wrap(err, "failed to build filters for count")
	}

	baseQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.schema.Name())

	query := baseQuery
	if len(whereClauses) > 0 {
		query = repo.Join(query, repo.JoinWhere(whereClauses...))
	}

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count entities")
	}

	return count, nil
}

func (r *repository[TEntity]) List(ctx context.Context, params *FindParams) ([]TEntity, error) {
	whereClauses, args, err := r.buildFilters(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build filters for list")
	}

	baseQuery := fmt.Sprintf("SELECT * FROM %s", r.schema.Name())
	query := baseQuery
	if len(whereClauses) > 0 {
		query = repo.Join(query, repo.JoinWhere(whereClauses...))
	}
	query = repo.Join(
		query,
		params.SortBy.ToSQL(r.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	entities, err := r.queryEntities(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list entities")
	}

	return entities, nil
}

func (r *repository[TEntity]) Create(ctx context.Context, values FieldValues) (TEntity, error) {
	var zero TEntity
	columns := make([]string, 0, len(values))
	args := make([]any, 0, len(values))

	for _, fv := range values {
		field := fv.Field()
		value := fv.Value()
		if field.Key() && fv.IsZero() {
			continue
		}
		columns = append(columns, field.Name())
		args = append(args, value)
	}

	if len(columns) == 0 {
		return zero, errors.New("no fields to create for entity")
	}

	query := repo.Insert(r.schema.Name(), columns, r.schema.Fields().Names()...)
	entities, err := r.queryEntities(ctx, query, args...)
	if err != nil {
		return zero, errors.Wrap(err, "failed to create entity")
	}
	if len(entities) != 1 {
		return zero, errors.Errorf("unexpected insert result count: %d", len(entities))
	}
	return entities[0], nil
}

func (r *repository[TEntity]) Update(ctx context.Context, values FieldValues) (TEntity, error) {
	var zero TEntity

	keyField := r.schema.Fields().KeyField()
	var fieldKeyValue FieldValue

	updates := make([]string, 0, len(values))
	args := make([]any, 0, len(values))

	for _, fv := range values {
		field := fv.Field()
		val := fv.Value()
		if field.Key() {
			fieldKeyValue = fv
			continue
		}
		updates = append(updates, field.Name())
		args = append(args, val)
	}

	if fieldKeyValue == nil || fieldKeyValue.IsZero() {
		return zero, errors.New("missing primary key or value")
	}

	whereClause := fmt.Sprintf("%s = $%d", keyField.Name(), len(args)+1)
	args = append(args, fieldKeyValue.Value())

	query := repo.Update(r.schema.Name(), updates, whereClause) + " RETURNING *"
	entities, err := r.queryEntities(ctx, query, args...)
	if err != nil {
		return zero, errors.Wrap(err, "failed to update entity")
	}
	if len(entities) != 1 {
		return zero, errors.Errorf("unexpected update result count: %d", len(entities))
	}
	return entities[0], nil
}

func (r *repository[TEntity]) Delete(ctx context.Context, value FieldValue) (TEntity, error) {
	var zero TEntity
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = $1 RETURNING *",
		r.schema.Name(),
		value.Field().Name(),
	)

	entities, err := r.queryEntities(ctx, query, value.Value())
	if err != nil {
		return zero, errors.Wrap(err, "failed to delete entity")
	}
	if len(entities) == 0 {
		return zero, errors.New("entity not found")
	}
	if len(entities) > 1 {
		return zero, errors.New("multiple entities deleted")
	}

	return entities[0], nil
}

func (r *repository[TEntity]) buildFilters(params *FindParams) ([]string, []any, error) {
	where := make([]string, 0)
	args := make([]any, 0)
	currentArgIdx := 1

	for _, filter := range params.Filters {
		column, ok := r.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
		}

		where = append(where, filter.Filter.String(column, currentArgIdx))
		filterValues := filter.Filter.Value()
		args = append(args, filterValues...)
		currentArgIdx += len(filterValues)
	}

	if params.Query != "" {
		searchClauses := make([]string, 0)
		for _, sf := range r.schema.Fields().Searchable() {
			searchClauses = append(
				searchClauses,
				fmt.Sprintf("LOWER(%s) LIKE $%d", sf.Name(), currentArgIdx),
			)
		}
		if len(searchClauses) > 0 {
			where = append(where, "("+strings.Join(searchClauses, " OR ")+")")
			args = append(args, "%"+strings.ToLower(params.Query)+"%")
		}
	}

	return where, args, nil
}

func (r *repository[TEntity]) queryEntities(ctx context.Context, query string, args ...any) ([]TEntity, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	columnDescriptions := rows.FieldDescriptions()
	columnOrder := make([]Field, len(columnDescriptions))
	for i, col := range columnDescriptions {
		f, err := r.schema.Fields().Field(col.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get field %q", col.Name)
		}
		columnOrder[i] = f
	}

	var entities []TEntity
	for rows.Next() {
		rawValues := make([]any, len(columnOrder))
		scanTargets := make([]any, len(columnOrder))
		for i := range rawValues {
			scanTargets[i] = &rawValues[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, errors.Wrap(err, "failed to scan entity row")
		}

		var fieldValues []FieldValue
		for i, val := range rawValues {
			fv := columnOrder[i].Value(val)
			fieldValues = append(fieldValues, fv)
		}

		entity, err := r.schema.Mapper().ToEntity(ctx, fieldValues)
		if err != nil {
			return nil, errors.Wrap(err, "failed to map field values to entity")
		}

		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	return entities, nil
}
