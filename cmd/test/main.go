package main

import (
	"log"

	"github.com/auxten/postgresql-parser/pkg/sql/parser"
	"github.com/auxten/postgresql-parser/pkg/walk"
)

func main() {
	sql := `ALTER TABLE table_name ADD COLUMN column_name VARCHAR(255);`
	w := &walk.AstWalker{
		Fn: func(ctx interface{}, node interface{}) (stop bool) {
			log.Printf("node type %T", node)
			return false
		},
	}

	stmts, err := parser.Parse(sql)
	if err != nil {
		panic(err)
	}

	_, err = w.Walk(stmts, nil)
	if err != nil {
		panic(err)
	}
}
