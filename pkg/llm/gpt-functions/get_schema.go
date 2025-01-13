package functions

import (
	"encoding/json"
	"slices"

	"gorm.io/gorm"
)

type Ref struct {
	To     string `json:"to"`
	Column string `json:"column"`
}

type Column struct {
	Type     string   `json:"type"`
	Nullable bool     `json:"nullable"`
	Enums    []string `json:"enums"`
	Ref      *Ref     `json:"ref"`
}

type Table struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Columns     map[string]Column `json:"columns"`
}

type DBColumn struct {
	ColumnName string `db:"column_name"`
	DataType   string `db:"data_type"`
	UdtName    string `db:"udt_name"`
	IsNullable string `db:"is_nullable"`
}

type Enum struct {
	EnumLabel string `db:"enumlabel"`
	TypName   string `db:"typname"`
}

func GetFkRelations(db *gorm.DB, tn string) ([]struct {
	ColumnName        string `db:"column_name"`
	ForeignTableName  string `db:"foreign_table_name"`
	ForeignColumnName string `db:"foreign_column_name"`
}, error,
) {
	var relations []struct {
		ColumnName        string `db:"column_name"`
		ForeignTableName  string `db:"foreign_table_name"`
		ForeignColumnName string `db:"foreign_column_name"`
	}
	err := db.Raw(`SELECT
										kcu.column_name,
										ccu.table_name AS foreign_table_name,
										ccu.column_name AS foreign_column_name
									FROM information_schema.table_constraints AS tc
									JOIN information_schema.key_column_usage AS kcu
										ON tc.constraint_name = kcu.constraint_name
										AND tc.table_schema = kcu.table_schema
									JOIN information_schema.constraint_column_usage AS ccu
										ON ccu.constraint_name = tc.constraint_name
									WHERE tc.constraint_type = 'FOREIGN KEY'
										AND tc.table_name=$1;`, tn).Scan(&relations).Error
	if err != nil {
		return nil, err
	}
	return relations, nil
}

func GetColumns(db *gorm.DB, tn string) ([]*DBColumn, error) {
	var columns []*DBColumn
	err := db.Raw(
		"SELECT column_name, data_type, udt_name, is_nullable FROM information_schema.columns WHERE table_name = $1",
		tn,
	).Scan(&columns).Error
	if err != nil {
		return nil, err
	}
	return columns, nil
}

func GetTables(db *gorm.DB) ([]string, error) {
	exclude := []string{"uploads", "permissions", "dialogues", "embeddings", "articles"}
	var tables []struct {
		Tablename string `db:"tablename"`
	}
	err := db.Raw("SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public'").Scan(&tables).Error
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(tables))
	for _, table := range tables {
		if slices.Contains(exclude, table.Tablename) {
			continue
		}
		result = append(result, table.Tablename)
	}
	return result, nil
}

type getSchema struct {
	db *gorm.DB
}

func NewGetSchema(db *gorm.DB) ChatFunctionDefinition {
	return getSchema{db: db}
}

func (g getSchema) Name() string {
	return "get_schema"
}

func (g getSchema) Description() string {
	return "Returns the database schema"
}

func (g getSchema) Arguments() map[string]interface{} {
	return map[string]interface{}{}
}

func (g getSchema) Execute(map[string]interface{}) (string, error) {
	tableNames, err := GetTables(g.db)
	if err != nil {
		return "", err
	}
	simpleTypes := map[string]string{
		"float8": "float",
		"int4":   "int",
	}
	tables := make([]*Table, 0, len(tableNames))
	for _, name := range tableNames {
		columns, err := GetColumns(g.db, name)
		if err != nil {
			return "", err
		}
		result := map[string]Column{}
		for _, column := range columns {
			t, ok := simpleTypes[column.UdtName]
			if !ok {
				t = column.DataType
			}
			col := Column{
				Type:     t,
				Nullable: column.IsNullable == "YES",
			}
			if column.DataType == "USER-DEFINED" {
				sql := `SELECT pg_type.typname, pg_enum.enumlabel FROM pg_type JOIN pg_enum ON pg_enum.enumtypid = pg_type.oid
						WHERE typname = $1`
				var enums []Enum
				if err := g.db.Raw(sql, column.UdtName).Scan(&enums).Error; err != nil {
					return "", err
				}
				col.Enums = []string{}
				for _, el := range enums {
					col.Enums = append(result[column.ColumnName].Enums, el.EnumLabel)
				}
			}
			result[column.ColumnName] = col
		}
		relations, err := GetFkRelations(g.db, name)
		if err != nil {
			return "", err
		}
		for _, relation := range relations {
			col := result[relation.ColumnName]
			col.Ref = &Ref{
				To:     relation.ForeignTableName,
				Column: relation.ForeignColumnName,
			}
		}
		tables = append(tables, &Table{
			Name:    name,
			Columns: result,
		})
	}
	jsonBytes, err := json.Marshal(tables)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
