package main

import (
	"fmt"
	"log"

	"github.com/iota-uz/psql-parser/sql/parser"
	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/iota-uz/psql-parser/walk"
)

func main() {
	sql := `
CREATE TABLE users
(
    id          SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email       VARCHAR(255)             NOT NULL UNIQUE,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
	`

	stmts, err := parser.Parse(sql)
	if err != nil {
		panic(err)
	}

	output := ""
	prettyPrinter := tree.PrettyCfg{
		LineWidth: 120,
		Simplify:  true,
		TabWidth:  4,
		UseTabs:   true,
		Align:     tree.PrettyNoAlign,
	}
	fmtCtx := tree.NewFmtCtx(tree.FmtSimple)
	w := &walk.AstWalker{
		Fn: func(ctx interface{}, node interface{}) (stop bool) {
			log.Printf("node type %T", node)
			if node == nil {
				return false
			}
			if v, ok := node.(*tree.ColumnTableDef); ok {
				if v.DefaultExpr.Expr != nil {
					fmt.Printf("ColumnTableDef: %T\n", v.DefaultExpr.Expr)
				}
			}
			if _, ok := node.(*tree.CreateTable); !ok {
				return false
			}
			fmtCtx.FormatNode(node.(tree.NodeFormatter))
			output += prettyPrinter.Pretty(node.(tree.NodeFormatter))
			return false
		},
	}

	_, err = w.Walk(stmts, nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(fmtCtx.CloseAndGetString())
	fmt.Println(output)
}
