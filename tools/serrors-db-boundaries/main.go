package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

func driverCall(expr ast.Expr, info *types.Info) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	path := fn.Pkg().Path()
	transaction := false
	if path == "github.com/iota-uz/iota-sdk/pkg/repo" {
		signature, ok := fn.Type().(*types.Signature)
		if ok && signature.Recv() != nil {
			owner, ok := signature.Recv().Type().(*types.Named)
			transaction = ok && owner.Obj().Name() == "Tx"
		}
	}
	if !transaction && path != "database/sql" && path != "github.com/jmoiron/sqlx" && !strings.HasPrefix(path, "github.com/jackc/pgx/") {
		return false
	}
	switch fn.Name() {
	case "Scan", "Exec", "ExecContext", "Query", "QueryContext", "QueryRow", "QueryRowContext", "Get", "GetContext", "Select", "SelectContext", "Err", "Commit":
		return true
	default:
		return false
	}
}

func driverErrors(stmt ast.Stmt, info *types.Info) map[types.Object]bool {
	result := make(map[types.Object]bool)
	a, ok := stmt.(*ast.AssignStmt)
	if !ok {
		return result
	}
	for i, lhs := range a.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		obj := info.ObjectOf(id)
		if obj == nil || !types.Identical(obj.Type(), types.Universe.Lookup("error").Type()) {
			continue
		}
		index := i
		if len(a.Rhs) == 1 {
			index = 0
		}
		if index < len(a.Rhs) && driverCall(a.Rhs[index], info) {
			result[obj] = true
		}
	}
	return result
}

func rewrite(file *ast.File, info *types.Info) int {
	count := 0
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
			objects := driverErrors(cond.Init, info)
			if cond.Init == nil && i > 0 {
				objects = driverErrors(block.List[i-1], info)
			}
			binary, ok := cond.Cond.(*ast.BinaryExpr)
			if !ok || binary.Op != token.NEQ {
				continue
			}
			id, ok := binary.X.(*ast.Ident)
			if !ok || !objects[info.ObjectOf(id)] {
				continue
			}
			nilID, ok := binary.Y.(*ast.Ident)
			if !ok || info.ObjectOf(nilID) != types.Universe.Lookup("nil") {
				continue
			}
			for _, branch := range cond.Body.List {
				ast.Inspect(branch, func(n ast.Node) bool {
					assignment, ok := n.(*ast.AssignStmt)
					if !ok {
						return true
					}
					for _, lhs := range assignment.Lhs {
						if target, ok := lhs.(*ast.Ident); ok && info.ObjectOf(target) == info.ObjectOf(id) {
							delete(objects, info.ObjectOf(id))
						}
					}
					return true
				})
				ret, ok := branch.(*ast.ReturnStmt)
				if !ok || !objects[info.ObjectOf(id)] {
					continue
				}
				for _, value := range ret.Results {
					call, ok := value.(*ast.CallExpr)
					if !ok || (len(call.Args) != 2 && len(call.Args) != 3) {
						continue
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					fn, ok := info.Uses[sel.Sel].(*types.Func)
					if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "github.com/iota-uz/iota-sdk/pkg/serrors" {
						continue
					}
					target := ""
					if fn.Name() == "Wrap" && len(call.Args) == 2 {
						target = "FromDB"
					} else if fn.Name() == "WrapContext" && len(call.Args) == 3 {
						target = "FromDBContext"
					}
					if target == "" {
						continue
					}
					cause, ok := call.Args[1].(*ast.Ident)
					if !ok || info.ObjectOf(cause) != info.ObjectOf(id) {
						continue
					}
					sel.Sel.Name = target
					count++
				}
			}
		}
		return true
	})
	return count
}

func run(root string, write bool) error {
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo}, "./...")
	if err != nil {
		return err
	}
	if packages.PrintErrors(pkgs) != 0 {
		return fmt.Errorf("type checking failed; refusing driver migration")
	}
	count := 0
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if !strings.HasSuffix(path, "repository.go") {
				continue
			}
			n := rewrite(file, pkg.TypesInfo)
			if n == 0 {
				continue
			}
			var buf bytes.Buffer
			if err := format.Node(&buf, pkg.Fset, file); err != nil {
				return err
			}
			if write {
				if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
					return err
				}
			}
			count += n
		}
	}
	_, _ = fmt.Fprintf(os.Stdout, "classified %d concrete driver error boundaries\n", count)
	return nil
}

func main() {
	root := flag.String("root", ".", "SDK checkout")
	write := flag.Bool("write", false, "write reviewed driver boundary replacements")
	flag.Parse()
	if err := run(*root, *write); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
