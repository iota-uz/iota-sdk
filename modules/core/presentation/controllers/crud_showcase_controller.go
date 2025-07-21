package controllers

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/components/multilang"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
)

const dropTableSql = `DROP TABLE IF EXISTS _showcases`
const createTableSql = `
CREATE TABLE IF NOT EXISTS _showcases
(
    _uuid      UUID PRIMARY KEY,
--     _string    VARCHAR(255),
--     _int       INTEGER,
--     _bool      BOOLEAN,
--     _float     DOUBLE PRECISION,
--     _decimal   NUMERIC,
--     _date      DATE,
--     _time      TIME,
--     _datetime  TIMESTAMP,
--     _timestamp TIMESTAMPTZ,
    _multilang JSONB,
    _entry_id  UUID
);
`

type ShowCaseOption func(se *showcaseEntity)

const (
	_uuid      string = "_uuid"
	_string    string = "_string"
	_int       string = "_int"
	_bool      string = "_bool"
	_float     string = "_float"
	_decimal   string = "_decimal"
	_date      string = "_date"
	_time      string = "_time"
	_dateTime  string = "_datetime"
	_timestamp string = "_timestamp"
	_multiLang string = "_multilang"
	_entry     string = "_entry_id"
)

type ShowcaseEntry interface {
	UUID() uuid.UUID
	String() string
}

func newShowcaseEntry(
	uuid uuid.UUID,
	_string string,
) ShowcaseEntry {
	return &showcaseEntry{
		uuid:   uuid,
		string: _string,
	}
}

type showcaseEntry struct {
	uuid   uuid.UUID
	string string
}

func (s *showcaseEntry) UUID() uuid.UUID {
	return s.uuid
}

func (s *showcaseEntry) String() string {
	return s.string
}

var entries = []ShowcaseEntry{
	newShowcaseEntry(
		uuid.MustParse("2c2ea8b8-22ac-48ea-91fe-3187d38e1550"),
		"entry_1",
	),
	newShowcaseEntry(
		uuid.MustParse("35c36c4a-9abf-43ef-b269-fda761acbf5f"),
		"entry_2",
	),
	newShowcaseEntry(
		uuid.MustParse("fab9e585-d0c6-4977-aab1-cb5e0e47f78f"),
		"entry_3",
	),
}

// WithMultiLangRenderer registers the MultiLang renderer for the showcase controller
func WithMultiLangRenderer[TEntity any]() CrudOption[TEntity] {
	return func(c *CrudController[TEntity]) {
		c.RegisterRenderer("multilang", multilang.NewMultiLangRenderer())
	}
}

type ShowcaseEntity interface {
	UUID() uuid.UUID
	String() string
	Int() int
	Bool() bool
	Float() float64
	Decimal() string
	Date() time.Time
	Time() time.Time
	DateTime() time.Time
	Timestamp() time.Time
	MultiLang() models.MultiLang
	Entry() ShowcaseEntry
}

func showcaseWithUuid(uuid uuid.UUID) ShowCaseOption {
	return func(s *showcaseEntity) {
		s.uuid = uuid
	}
}
func showcaseWithString(s string) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.string = s
	}
}
func showcaseWithInt(i int) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.int = i
	}
}
func showcaseWithBool(b bool) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.bool = b
	}
}
func showcaseWithFloat(f float64) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.float = f
	}
}
func showcaseWithDecimal(d string) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.decimal = d
	}
}
func showcaseWithDate(date time.Time) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.date = date
	}
}
func showcaseWithTime(t time.Time) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.time = t
	}
}
func showcaseWithDateTime(dateTime time.Time) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.datetime = dateTime
	}
}
func showcaseWithTimestamp(t time.Time) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.timestamp = t
	}
}
func showcaseWithMultiLang(ml models.MultiLang) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.multiLang = ml
	}
}
func showcaseWithEntry(s ShowcaseEntry) ShowCaseOption {
	return func(se *showcaseEntity) {
		se.showcaseEntry = s
	}
}

func newShowcase(
	opts ...ShowCaseOption,
) ShowcaseEntity {
	se := &showcaseEntity{}
	for _, opt := range opts {
		opt(se)
	}
	return se
}

type showcaseEntity struct {
	uuid          uuid.UUID
	string        string
	int           int
	bool          bool
	float         float64
	decimal       string
	date          time.Time
	time          time.Time
	datetime      time.Time
	timestamp     time.Time
	multiLang     models.MultiLang
	showcaseEntry ShowcaseEntry
}

func (s *showcaseEntity) UUID() uuid.UUID {
	return s.uuid
}
func (s *showcaseEntity) String() string {
	return s.string
}
func (s *showcaseEntity) Int() int {
	return s.int
}
func (s *showcaseEntity) Bool() bool {
	return s.bool
}
func (s *showcaseEntity) Float() float64 {
	return s.float
}
func (s *showcaseEntity) Decimal() string {
	return s.decimal
}
func (s *showcaseEntity) Date() time.Time {
	return s.date
}
func (s *showcaseEntity) Time() time.Time {
	return s.time
}
func (s *showcaseEntity) DateTime() time.Time {
	return s.datetime
}
func (s *showcaseEntity) Timestamp() time.Time {
	return s.timestamp
}
func (s *showcaseEntity) MultiLang() models.MultiLang {
	return s.multiLang
}
func (s *showcaseEntity) Entry() ShowcaseEntry {
	return s.showcaseEntry
}

func newShowcaseMapper(
	fields crud.Fields,
) crud.Mapper[ShowcaseEntity] {
	return &showcaseMapper{
		fields: fields,
	}
}

type showcaseMapper struct {
	fields crud.Fields
}

func (s *showcaseMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]ShowcaseEntity, error) {
	result := make([]ShowcaseEntity, len(values))

	selected := map[uuid.UUID]ShowcaseEntry{}
	var entryIds []uuid.UUID

	for _, fvs := range values {
		for _, fv := range fvs {
			if fv.Field().Name() == _entry && fv.Value() != nil {
				seId, err := fv.AsUUID()
				if err != nil {
					return nil, err
				}
				entryIds = append(entryIds, seId)
			}
		}
	}

	if len(entryIds) > 0 {
		for _, entry := range entries {
			if slices.Contains(entryIds, entry.UUID()) {
				selected[entry.UUID()] = entry
			}
		}
	}

	for i, fvs := range values {
		var opts []ShowCaseOption
		for _, fv := range fvs {
			switch fv.Field().Name() {
			case _uuid:
				uuidValue, err := fv.AsUUID()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithUuid(uuidValue))
			case _string:
				stringValue, err := fv.AsString()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithString(stringValue))
			case _int:
				intValue, err := fv.AsInt()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithInt(intValue))
			case _bool:
				boolValue, err := fv.AsBool()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithBool(boolValue))
			case _float:
				floatValue, err := fv.AsFloat64()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithFloat(floatValue))
			case _decimal:
				decimalValue, err := fv.AsDecimal()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithDecimal(decimalValue))
			case _date:
				dateValue, err := fv.AsTime()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithDate(dateValue))
			case _time:
				timeValue, err := fv.AsTime()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithTime(timeValue))
			case _dateTime:
				dateTimeValue, err := fv.AsTime()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithDateTime(dateTimeValue))
			case _timestamp:
				timestampValue, err := fv.AsTime()
				if err != nil {
					return nil, err
				}
				opts = append(opts, showcaseWithTimestamp(timestampValue))
			case _multiLang:
				json, err := fv.AsJSON()
				if err != nil {
					return nil, err
				}
				if json != "" {
					ml, err := models.MultiLangFromString(json)
					if err != nil {
						return nil, err
					}
					opts = append(opts, showcaseWithMultiLang(ml))
				}
			case _entry:
				if fv.Value() != nil {
					uuidValue, err := fv.AsUUID()
					if err != nil {
						return nil, err
					}
					entry := selected[uuidValue]
					opts = append(opts, showcaseWithEntry(entry))
				}
			}
		}
		result[i] = newShowcase(opts...)
	}

	return result, nil
}

func (s *showcaseMapper) ToFieldValuesList(_ context.Context, entities ...ShowcaseEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))
	for i, entity := range entities {
		values := map[string]any{
			_uuid: entity.UUID(),
			//_string:    entity.String(),
			//_int:       entity.Int(),
			//_bool:      entity.Bool(),
			//_float:     entity.Float(),
			//_decimal:   entity.Decimal(),
			//_date:      entity.Date(),
			//_time:      entity.Time(),
			//_dateTime:  entity.DateTime(),
			//_timestamp: entity.Timestamp(),
			_multiLang: entity.MultiLang(),
			_entry:     nil,
		}
		if entity.Entry() != nil {
			values[_entry] = entity.Entry().UUID()
		}

		fvs, err := s.fields.FieldValues(values)
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
}

func NewCrudShowcaseController(
	app application.Application,
	opts ...CrudOption[ShowcaseEntity],
) application.Controller {
	fields := crud.NewFields([]crud.Field{
		crud.NewUUIDField(
			_uuid,
			crud.WithKey(),
			crud.WithInitialValue(func() any {
				return uuid.New()
			}),
		),
		//crud.NewStringField(
		//	_string,
		//),
		//crud.NewIntField(
		//	_int,
		//),
		//crud.NewBoolField(
		//	_bool,
		//),
		//crud.NewFloatField(
		//	_float,
		//),
		//crud.NewDecimalField(
		//	_decimal,
		//),
		//crud.NewDateField(
		//	_date,
		//),
		//crud.NewTimeField(
		//	_time,
		//),
		//crud.NewDateTimeField(
		//	_dateTime,
		//),
		//crud.NewTimestampField(
		//	_timestamp,
		//),
		crud.NewJSONField(
			_multiLang,
			crud.JSONFieldConfig[models.MultiLang]{
				Validator: func(m models.MultiLang) error {
					return nil
				},
			},
			crud.WithRenderer("multilang"),
		),
		crud.NewSelectField(_entry).
			AsUUIDSelect().
			SetOptionsLoader(func(ctx context.Context) []crud.SelectOption {
				options := make([]crud.SelectOption, len(entries))
				for i, entry := range entries {
					options[i] = crud.SelectOption{
						Value: entry.UUID(),
						Label: entry.String(),
					}
				}
				return options
			}),
	})

	mapper := newShowcaseMapper(fields)
	schema := crud.NewSchema(
		"_showcases",
		fields,
		mapper,
	)

	builder := crud.NewBuilder[ShowcaseEntity](
		schema,
		app.EventPublisher(),
	)

	// Merge the MultiLang renderer option with user-provided options
	allOpts := make([]CrudOption[ShowcaseEntity], 0, len(opts)+1)
	allOpts = append(allOpts, WithMultiLangRenderer[ShowcaseEntity]())
	allOpts = append(allOpts, opts...)

	return NewCrudController(
		"/_dev/crud",
		app,
		builder,
		allOpts...,
	)
}

func InitCrudShowcase(app application.Application) {
	ctx := context.Background()
	tx, err := app.DB().Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}

	ctx = composables.WithTx(ctx, tx)

	_, err = tx.Exec(ctx, dropTableSql)
	if err != nil {
		log.Fatal(err)
		return
	}
	_, err = tx.Exec(ctx, createTableSql)
	if err != nil {
		log.Fatal(err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
}
