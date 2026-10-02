// Command serrors-migrate rewrites legacy error constructors for manual review.
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
	"strconv"
	"strings"
)

func main() {
	root := flag.String("root", ".", "SDK checkout")
	write := flag.Bool("write", false, "write reviewed mechanical replacements")
	flag.Parse()
	count := 0
	err := filepath.WalkDir(*root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, "pkg/serrors/") || strings.Contains(path, "tools/serrors-migrate/") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		alias := ""
		for _, imp := range file.Imports {
			if imp.Path.Value == strconv.Quote("github.com/iota-uz/iota-sdk/pkg/serrors") {
				alias = "serrors"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
			}
		}
		if alias == "" {
			return nil
		}
		changed := false
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != alias || sel.Sel.Name != "E" {
				return true
			}
			args := call.Args
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "REVIEW %s: constructor with fewer than two arguments\n", fset.Position(call.Pos()))
				return true
			}
			op := args[0]
			code := ""
			payload := args[1:]
			if kind, ok := args[0].(*ast.SelectorExpr); ok {
				if pkg, ok := kind.X.(*ast.Ident); ok && pkg.Name == alias {
					switch kind.Sel.Name {
					case "Invalid", "KindValidation", "NotFound", "PermissionDenied", "Internal", "Other":
						op = &ast.BasicLit{Kind: token.STRING, Value: "\"\""}
						code = kind.Sel.Name
					}
				}
			}
			if kind, ok := payload[0].(*ast.SelectorExpr); ok {
				if pkg, ok := kind.X.(*ast.Ident); ok && pkg.Name == alias {
					code = kind.Sel.Name
					payload = payload[1:]
				}
			}
			if code == "KindValidation" {
				code = "Invalid"
			}
			if code == "Other" {
				code = ""
			}
			if len(payload) == 0 {
				fmt.Fprintf(os.Stderr, "REVIEW %s: empty payload\n", fset.Position(call.Pos()))
				return true
			}
			var context ast.Expr
			if len(payload) > 1 {
				if len(payload) == 2 && stringExpr(payload[0]) != stringExpr(payload[1]) {
					if stringExpr(payload[0]) {
						context = payload[0]
						payload = payload[1:]
					} else {
						context = payload[1]
						payload = payload[:1]
					}
				} else {
					fmt.Fprintf(os.Stderr, "REVIEW %s: multiple payload arguments (%d)\n", fset.Position(call.Pos()), len(payload))
					return true
				}
			}
			value := payload[len(payload)-1]
			isString := stringExpr(value)
			selector := func(name string) ast.Expr { return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(name)} }
			method := func(receiver ast.Expr, name string, arg ast.Expr) ast.Expr {
				return &ast.CallExpr{Fun: &ast.SelectorExpr{X: receiver, Sel: ast.NewIdent(name)}, Args: []ast.Expr{arg}}
			}
			var replacement ast.Expr
			if code == "" && !isString {
				if context != nil {
					replacement = &ast.CallExpr{Fun: selector("WrapContext"), Args: []ast.Expr{op, value, context}}
				} else {
					replacement = &ast.CallExpr{Fun: selector("Wrap"), Args: []ast.Expr{op, value}}
				}
			} else {
				if code == "" {
					code = "Internal"
				}
				message := ast.Expr(&ast.BasicLit{Kind: token.STRING, Value: "\"\""})
				if isString {
					message = value
				}
				if context != nil {
					message = context
				}
				replacement = &ast.CallExpr{Fun: selector("New"), Args: []ast.Expr{selector(code), message}}
				replacement = method(replacement, "WithOp", op)
				if !isString {
					replacement = method(replacement, "WithCause", value)
				}
			}
			*call = *(replacement.(*ast.CallExpr))
			changed = true
			count++
			return true
		})
		if !changed {
			return nil
		}
		var out bytes.Buffer
		if err := format.Node(&out, fset, file); err != nil {
			return err
		}
		if *write {
			return os.WriteFile(path, out.Bytes(), 0644)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Migrated %d legacy constructors\n", count)
}

func stringExpr(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return v.Kind == token.STRING
	case *ast.BinaryExpr:
		return v.Op == token.ADD && (stringExpr(v.X) || stringExpr(v.Y))
	case *ast.CallExpr:
		if s, ok := v.Fun.(*ast.SelectorExpr); ok {
			if p, ok := s.X.(*ast.Ident); ok {
				return p.Name == "fmt" && s.Sel.Name == "Sprintf"
			}
		}
	}
	return false
}
