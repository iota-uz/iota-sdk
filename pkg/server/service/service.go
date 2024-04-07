package service

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
)

type DataType int

const (
	BigInt                DataType = iota // BigInt represents a signed eight-byte integer
	BigSerial                             // BigSerial represents an autoincrementing eight-byte integer
	Bit                                   // Bit represents a fixed-length bit string
	BitVarying                            // BitVarying represents a variable-length bit string
	Boolean                               // Boolean represents a logical Boolean (true/false)
	Box                                   // Box represents a rectangular box on a plane
	Bytea                                 // Bytea represents binary data (“byte array”)
	Character                             // Character represents a fixed-length character string
	CharacterVarying                      // CharacterVarying represents a variable-length character string
	Cidr                                  // Cidr represents an IPv4 or IPv6 network address
	Circle                                // Circle represents a circle on a plane
	Date                                  // Date represents a calendar date (year, month, day)
	DoublePrecision                       // DoublePrecision represents a double precision floating-point number (8 bytes)
	Inet                                  // Inet represents an IPv4 or IPv6 host address
	Integer                               // Integer represents a signed four-byte integer
	Interval                              // Interval represents a time span
	Json                                  // Json represents textual JSON data
	Jsonb                                 // Jsonb represents binary JSON data, decomposed
	Line                                  // Line represents an infinite line on a plane
	Lseg                                  // Lseg represents a line segment on a plane
	Macaddr                               // Macaddr represents a MAC (Media Access Control) address
	Macaddr8                              // Macaddr8 represents a MAC (Media Access Control) address (EUI-64 format)
	Money                                 // Money represents a currency amount
	Numeric                               // Numeric represents an exact numeric of selectable precision
	Path                                  // Path represents a geometric path on a plane
	PgLsn                                 // PgLsn represents a PostgreSQL Log Sequence Number
	PgSnapshot                            // PgSnapshot represents a user-level transaction ID snapshot
	Point                                 // Point represents a geometric point on a plane
	Polygon                               // Polygon represents a closed geometric path on a plane
	Real                                  // Real represents a single precision floating-point number (4 bytes)
	SmallInt                              // SmallInt represents a signed two-byte integer
	SmallSerial                           // SmallSerial represents an autoincrementing two-byte integer
	Serial                                // Serial represents an autoincrementing four-byte integer
	Text                                  // Text represents a variable-length character string
	Time                                  // Time represents a time of day (no time zone)
	TimeWithTimeZone                      // TimeWithTimeZone represents a time of day, including time zone
	Timestamp                             // Timestamp represents a date and time (no time zone)
	TimestampWithTimeZone                 // TimestampWithTimeZone represents a date and time, including time zone
	Tsquery                               // Tsquery represents a text search query
	Tsvector                              // Tsvector represents a text search document
	TxidSnapshot                          // TxidSnapshot represents a user-level transaction ID snapshot (deprecated; see pg_snapshot)
	Uuid                                  // Uuid represents a universally unique identifier
	Xml                                   // Xml represents XML data
)

type Association struct {
	To     *Model
	Column string
	As     string
}

type Field struct {
	Name        string
	Type        DataType
	Nullable    bool
	Association *Association
}

type Model struct {
	Pk     *Field
	Table  string
	Fields []*Field
}

type CountQuery struct {
	Query []goqu.Expression
}

type GetQuery struct {
	Id    int64
	Attrs []interface{}
}

type FindQuery struct {
	Query  []goqu.Expression
	Attrs  []interface{}
	Sort   []string
	Limit  int
	Offset int
}

type AggregateQuery struct {
	Query       []goqu.Expression
	Expressions []goqu.Expression
	GroupBy     []string
	Sort        []string
	Limit       int
	Offset      int
}

// Service A wrapper around SQLx to provide CRUD operations for a service
type Service interface {
	Model() *Model
	Count(q *CountQuery) (int, error)
	Get(q *GetQuery) (map[string]interface{}, error)
	Find(q *FindQuery) ([]map[string]interface{}, error)
	ExecuteFind(query *goqu.SelectDataset) ([]map[string]interface{}, error)
	Aggregate(q *AggregateQuery) ([]map[string]interface{}, error)
	Create(data map[string]interface{}) (map[string]interface{}, error)
	Patch(id int64, data map[string]interface{}) (map[string]interface{}, error)
	Remove(id int64) error
}

type serviceImpl struct {
	Db    *sqlx.DB
	model *Model
}

func New(db *sqlx.DB, model *Model) Service {
	return &serviceImpl{
		Db:    db,
		model: model,
	}
}

func (s *serviceImpl) PkCol() string {
	return s.model.Pk.Name
}

func (s *serviceImpl) Model() *Model {
	return s.model
}

func (s *serviceImpl) Count(q *CountQuery) (int, error) {
	query := goqu.From(s.model.Table).Where(q.Query...)
	stmt, _, err := query.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return 0, err
	}
	var count int
	if err := s.Db.Get(&count, stmt); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *serviceImpl) Get(q *GetQuery) (map[string]interface{}, error) {
	stmt, _, err := goqu.From(s.model.Table).Select(q.Attrs...).Where(goqu.Ex{s.PkCol(): q.Id}).ToSQL()
	if err != nil {
		return nil, err
	}
	row := s.Db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func (s *serviceImpl) ExecuteFind(query *goqu.SelectDataset) ([]map[string]interface{}, error) {
	stmt, _, err := query.ToSQL()
	fmt.Println(stmt)
	if err != nil {
		return nil, err
	}
	rows, err := s.Db.Queryx(stmt)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var data []map[string]interface{}
	for rows.Next() {
		row := map[string]interface{}{}
		if err := rows.MapScan(row); err != nil {
			return data, err
		}
		data = append(data, nestMap(row))
	}
	return data, nil
}

func orderStringToExpression(order []string) []exp.OrderedExpression {
	var orderExpr []exp.OrderedExpression
	for _, sort := range order {
		if sort[0] == '-' {
			orderExpr = append(orderExpr, goqu.I(sort[1:]).Desc())
		} else {
			orderExpr = append(orderExpr, goqu.I(sort).Asc())
		}
	}
	return orderExpr
}

func (s *serviceImpl) Find(q *FindQuery) ([]map[string]interface{}, error) {
	query := goqu.From(s.model.Table).Select(q.Attrs...).Where(q.Query...)
	if q.Limit > 0 {
		query = query.Limit(uint(q.Limit))
	}
	if q.Offset > 0 {
		query = query.Offset(uint(q.Offset))
	}
	if len(q.Sort) > 0 {
		query = query.Order(orderStringToExpression(q.Sort)...)
	}
	return s.ExecuteFind(query)
}

func (s *serviceImpl) Aggregate(q *AggregateQuery) ([]map[string]interface{}, error) {
	var selects []interface{}
	for _, expr := range q.GroupBy {
		selects = append(selects, goqu.I(expr))
	}
	for _, expr := range q.Expressions {
		selects = append(selects, expr)
	}
	var groupBy []interface{}
	for _, field := range q.GroupBy {
		groupBy = append(groupBy, goqu.I(field))
	}
	query := goqu.From(s.model.Table).Select(selects...).Where(q.Query...).GroupBy(groupBy...)
	if q.Limit > 0 {
		query = query.Limit(uint(q.Limit))
	}
	if q.Offset > 0 {
		query = query.Offset(uint(q.Offset))
	}
	stmt, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := s.Db.Queryx(stmt)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var data []map[string]interface{}
	for rows.Next() {
		row := map[string]interface{}{}
		if err := rows.MapScan(row); err != nil {
			return nil, err
		}
		data = append(data, nestMap(row))
	}
	return data, nil
}

func (s *serviceImpl) Create(data map[string]interface{}) (map[string]interface{}, error) {
	stmt, _, err := goqu.Insert(s.model.Table).Rows(data).Returning("*").ToSQL()
	if err != nil {
		return nil, err
	}
	row := s.Db.QueryRowx(stmt)
	data = make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *serviceImpl) Patch(id int64, data map[string]interface{}) (map[string]interface{}, error) {
	stmt, _, err := goqu.Update(s.model.Table).Set(data).Where(goqu.Ex{s.PkCol(): id}).Returning("*").ToSQL()
	if err != nil {
		return nil, err
	}
	row := s.Db.QueryRowx(stmt)
	data = make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *serviceImpl) Remove(id int64) error {
	stmt, _, err := goqu.Delete(s.model.Table).Where(goqu.Ex{s.PkCol(): id}).ToSQL()
	if err != nil {
		return err
	}
	_, err = s.Db.Exec(stmt)
	return err
}

func NewService(db *sqlx.DB, model *Model) Service {
	return &serviceImpl{
		Db:    db,
		model: model,
	}
}
