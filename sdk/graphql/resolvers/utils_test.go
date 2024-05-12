package resolvers

import (
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/models"
	"testing"
)

type Image struct {
	Id int `graphql:"id" db:"id"`
}

func (i *Image) Pk() interface{} {
	return i.Id
}

func (i *Image) PkField() *models.Field {
	return &models.Field{
		Name: "id",
		Type: models.BigSerial,
	}
}

func (i *Image) Table() string {
	return "images"
}

type Company struct {
	Id   int    `graphql:"id" db:"id"`
	Name string `graphql:"name" db:"name"`
	Logo Image  `graphql:"logo" db:"logo_id"`
}

func (c *Company) Pk() interface{} {
	return c.Id
}

func (c *Company) PkField() *models.Field {
	return &models.Field{
		Name: "id",
		Type: models.BigSerial,
	}
}

func (c *Company) Table() string {
	return "companies"
}

type Employees struct {
	Id      int      `graphql:"id" db:"id"`
	Company *Company `graphql:"company" db:"company_id"`
}

func (e *Employees) Pk() interface{} {
	return e.Id
}

func (e *Employees) PkField() *models.Field {
	return &models.Field{
		Name: "id",
		Type: models.BigSerial,
	}
}

func (e *Employees) Table() string {
	return "employees"
}

func TestResolveToQuery(t *testing.T) {
	t.Run("Test ResolveToQuery", func(t *testing.T) {
		selectionSet := &ast.SelectionSet{
			Selections: []ast.Selection{
				&ast.Field{
					Name: &ast.Name{
						Value: "id",
					},
				},
				&ast.Field{
					Name: &ast.Name{
						Value: "company",
					},
					SelectionSet: &ast.SelectionSet{
						Selections: []ast.Selection{
							&ast.Field{
								Name: &ast.Name{
									Value: "id",
								},
							},
							&ast.Field{
								Name: &ast.Name{
									Value: "name",
								},
							},
							&ast.Field{
								Name: &ast.Name{
									Value: "logo",
								},
								SelectionSet: &ast.SelectionSet{
									Selections: []ast.Selection{
										&ast.Field{
											Name: &ast.Name{
												Value: "id",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		q := ResolveToQuery(selectionSet, &Employees{})
		stmt, _, err := q.ToSQL()
		if err != nil {
			t.Error(err)
		}
		expected := `SELECT "employees"."id", "companies"."id" AS "company.id", "companies"."name" AS "company.name", "images"."id" AS "company.logo.id" FROM employees LEFT JOIN companies ON employees.company_id = companies.id LEFT JOIN images AS company_logo ON companies.logo_id = company_logo.id`
		if stmt != expected {
			t.Errorf("Expected %s\nGot %s", expected, stmt)
		}
	})
}
