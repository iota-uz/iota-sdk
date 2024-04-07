package service

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"reflect"
)

type Field struct {
	Name     string
	Type     reflect.Kind
	Nullable bool
}

type Model struct {
	Pk     string
	Table  string
	Fields []*Field
}

type CountQuery struct {
	Query []goqu.Expression
}

type GetQuery struct {
	Id    int64
	Attrs []string
}

type FindQuery struct {
	Query  []goqu.Expression
	Attrs  []string
	Sort   []string
	Limit  int
	Offset int
}

type AggregateQuery struct {
	Query       []goqu.Expression
	Expressions []goqu.Expression
	GroupBy     []string
}

// Service A wrapper around SQLx to provide CRUD operations for a service
type Service interface {
	Model() *Model
	Count(q *CountQuery) (int, error)
	Get(q *GetQuery) (map[string]interface{}, error)
	Find(q *FindQuery) ([]map[string]interface{}, error)
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
	stmt, _, err := goqu.From(s.model.Table).Where(goqu.Ex{s.model.Pk: q.Id}).ToSQL()
	if err != nil {
		return nil, err
	}
	row := s.Db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *serviceImpl) Find(q *FindQuery) ([]map[string]interface{}, error) {
	var attrs []interface{}
	for _, attr := range q.Attrs {
		attrs = append(attrs, attr)
	}
	query := goqu.From(s.model.Table).Select(attrs...).Where(q.Query...)
	if q.Limit > 0 {
		query = query.Limit(uint(q.Limit))
	}
	if q.Offset > 0 {
		query = query.Offset(uint(q.Offset))
	}
	if len(q.Sort) > 0 {
		var order []exp.OrderedExpression
		for _, sort := range q.Sort {
			if sort[0] == '-' {
				order = append(order, goqu.I(sort[1:]).Desc())
			} else {
				order = append(order, goqu.I(sort).Asc())
			}
		}
		query = query.Order(order...)
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
			return data, err
		}
		data = append(data, row)
	}
	return data, nil
}

func (s *serviceImpl) Aggregate(q *AggregateQuery) ([]map[string]interface{}, error) {
	var selects []interface{}
	for _, expr := range q.Expressions {
		selects = append(selects, expr)
	}
	var groupBy []interface{}
	for _, field := range q.GroupBy {
		groupBy = append(groupBy, goqu.I(field))
	}
	stmt, _, err := goqu.From(s.model.Table).Select(selects...).Where(q.Query...).GroupBy(groupBy...).ToSQL()
	if err != nil {
		return nil, err
	}
	fmt.Println(stmt)
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
		data = append(data, row)
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
	stmt, _, err := goqu.Update(s.model.Table).Set(data).Where(goqu.Ex{s.model.Pk: id}).Returning("*").ToSQL()
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
	stmt, _, err := goqu.Delete(s.model.Table).Where(goqu.Ex{s.model.Pk: id}).ToSQL()
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
