package service

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"os"
	"testing"
)

func BeforeEach() {
	db := sqlx.MustConnect("postgres", "user=postgres password=postgres dbname=postgres sslmode=disable")
	db.MustExec("DROP TABLE IF EXISTS users")
	db.MustExec("CREATE TABLE users (id SERIAL PRIMARY KEY, first_name TEXT)")
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestServiceImpl_Get(t *testing.T) {
	BeforeEach()
	db := sqlx.MustConnect("postgres", "user=postgres password=postgres dbname=postgres sslmode=disable")
	defer db.Close()

	fields := []*Field{
		{Name: "first_name", Type: "string"},
	}
	model := &Model{
		Pk:     "id",
		Table:  "users",
		Fields: fields,
	}

	s := NewService(db, model)

	_, err := s.Create(map[string]interface{}{
		"first_name": "John",
	})

	if err != nil {
		t.Errorf("Error creating data: %v", err)
	}

	q := &GetQuery{Id: 1}
	data, err := s.Get(q)

	if err != nil {
		t.Errorf("Error getting data: %v", err)
	}

	if data["first_name"] != "John" {
		t.Errorf("Expected first_name to be John, got %v", data["first_name"])
	}
}

func TestServiceImpl_Find(t *testing.T) {
	BeforeEach()
	db := sqlx.MustConnect("postgres", "user=postgres password=postgres dbname=postgres sslmode=disable")
	defer db.Close()

	fields := []*Field{
		{Name: "first_name", Type: "string"},
	}
	model := &Model{
		Pk:     "id",
		Table:  "users",
		Fields: fields,
	}

	s := NewService(db, model)

	_, err := s.Create(map[string]interface{}{
		"first_name": "John",
	})

	if err != nil {
		t.Errorf("Error creating data: %v", err)
	}

	q := &FindQuery{Query: []goqu.Expression{goqu.Ex{"first_name": "John"}}}
	data, err := s.Find(q)

	if err != nil {
		t.Errorf("Error finding data: %v", err)
	}

	if len(data) != 1 {
		t.Errorf("Expected 1 result, got %v", len(data))
	}

	if data[0]["first_name"] != "John" {
		t.Errorf("Expected first_name to be John, got %v", data[0]["first_name"])
	}

	data2, err := s.Find(&FindQuery{
		Query: []goqu.Expression{goqu.Ex{"first_name": "Jane"}},
	})

	if err != nil {
		t.Errorf("Error finding data: %v", err)
	}

	if len(data2) != 0 {
		t.Errorf("Expected 0 result, got %v", len(data2))
	}
}
