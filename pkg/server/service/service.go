package service

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type Field struct {
	Name string
	Type string
}

type Model struct {
	Pk     string
	Table  string
	Fields []*Field
}

type GetQuery struct {
	Id    int64
	Attrs []string
}

type FindQuery struct {
	Query []goqu.Expression
	Attrs []string
}

// Service A wrapper around SQLx to provide CRUD operations for a service
type Service interface {
	Model() *Model
	Get(q *GetQuery) (map[string]interface{}, error)
	Find(q *FindQuery) ([]map[string]interface{}, error)
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
	var data []map[string]interface{}
	var attrs []interface{}
	for _, attr := range q.Attrs {
		attrs = append(attrs, attr)
	}
	stmt, _, err := goqu.From(s.model.Table).Select(attrs...).Where(q.Query...).ToSQL()
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
