package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func db(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch s.Sel.Name {
		case "Scan", "Exec", "ExecContext", "Query", "QueryContext", "QueryRow", "QueryRowContext", "Get", "Select":
			found = true
		}
		return true
	})
	return found
}
func assignment(stmt ast.Stmt) bool {
	a, ok := stmt.(*ast.AssignStmt)
	if !ok {
		return false
	}
	for _, expr := range a.Rhs {
		if db(expr) {
			return true
		}
	}
	return false
}
func main() {
	root := flag.String("root", ".", "SDK checkout")
	flag.Parse()
	count := 0
	err := filepath.WalkDir(*root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, "repository.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		changed := false
		ast.Inspect(file, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for i, stmt := range block.List {
				cond, ok := stmt.(*ast.IfStmt)
				if !ok {
					continue
				}
				boundary := assignment(cond.Init) || (i > 0 && assignment(block.List[i-1]))
				if !boundary {
					continue
				}
				ast.Inspect(cond.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					id, ok := sel.X.(*ast.Ident)
					if ok && id.Name == "serrors" && sel.Sel.Name == "Wrap" {
						sel.Sel.Name = "FromDB"
						changed = true
						count++
					}
					return true
				})
			}
			return true
		})
		if changed {
			var b bytes.Buffer
			if err := format.Node(&b, fset, file); err != nil {
				return err
			}
			return os.WriteFile(path, b.Bytes(), 0644)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(count)
}
