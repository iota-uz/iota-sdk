package dbutils

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

func Get(db *sqlx.DB, query *goqu.SelectDataset) (map[string]interface{}, error) {
	stmt, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func Find(db *sqlx.DB, query *goqu.SelectDataset) ([]map[string]interface{}, error) {
	stmt, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := db.Queryx(stmt)
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

func Count(db *sqlx.DB, query *goqu.SelectDataset) (int, error) {
	stmt, _, err := query.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return 0, err
	}
	var count int
	if err := db.Get(&count, stmt); err != nil {
		return 0, err
	}
	return count, nil
}

func Create(db *sqlx.DB, query *goqu.InsertDataset) (map[string]interface{}, error) {
	stmt, _, err := query.Returning("*").ToSQL()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func Patch(db *sqlx.DB, query *goqu.UpdateDataset) (map[string]interface{}, error) {
	stmt, _, err := query.Returning("*").ToSQL()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func Delete(db *sqlx.DB, query *goqu.DeleteDataset) error {
	stmt, _, err := query.ToSQL()
	if err != nil {
		return err
	}
	resp, err := db.Exec(stmt)
	if err != nil {
		return err
	}
	if rows, err := resp.RowsAffected(); err != nil || rows == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}
