package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetDefinition handles the get_definition tool request
func GetDefinition(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok || path == "" {
		return nil, errors.New("path parameter is required and must be a string")
	}

	// Split the path into package path and symbol name
	lastDotIndex := strings.LastIndex(path, ".")
	if lastDotIndex == -1 {
		return nil, errors.New("invalid path format, expected format: 'packagepath.Symbol'")
	}

	packagePath := path[:lastDotIndex]
	symbolName := path[lastDotIndex+1:]

	// Convert the package path to a directory path
	// Assuming the repo is cloned at GOPATH/src/github.com/...
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %v", err)
		}

		// Try to find the module root by looking for the repository root
		goPathSrc := filepath.Join(cwd, "..")
		for i := 0; i < 5; i++ { // Look up to 5 levels up
			if _, err := os.Stat(filepath.Join(goPathSrc, "go.mod")); err == nil {
				goPath = goPathSrc
				break
			}
			goPathSrc = filepath.Join(goPathSrc, "..")
		}

		if goPath == "" {
			return nil, errors.New("GOPATH environment variable not set and couldn't find module root")
		}
	}

	// The path might be in the format github.com/iota-uz/iota-sdk/pkg/repo
	// But the actual filesystem path depends on the module structure
	projectRoot, err := GetProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to determine project root: %v", err)
	}

	// Extract the relative path within the module
	modulePrefix := "github.com/iota-uz/iota-sdk/"
	var dirPath string
	if strings.HasPrefix(packagePath, modulePrefix) {
		// Remove the module prefix to get the relative path
		relPath := strings.TrimPrefix(packagePath, modulePrefix)
		dirPath = filepath.Join(projectRoot, relPath)
	} else {
		return nil, fmt.Errorf("package %s is not part of this project", packagePath)
	}

	// Create a file set to keep track of positions in source files
	fset := token.NewFileSet()

	// Parse the package
	pkgs, err := parser.ParseDir(fset, dirPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("error parsing package: %v", err)
	}

	// Find the package with the matching name
	var pkg *ast.Package
	for _, p := range pkgs {
		// The last part of the import path should be the package name
		packageName := filepath.Base(packagePath)
		if p.Name == packageName {
			pkg = p
			break
		}
	}

	if pkg == nil {
		return nil, fmt.Errorf("package not found: %s", packagePath)
	}

	// Search for the symbol in the package
	definition := findDefinition(pkg, symbolName, fset)
	if definition == "" {
		return nil, fmt.Errorf("symbol %s not found in package %s", symbolName, packagePath)
	}

	return mcp.NewToolResultText(definition), nil
}

func findDefinition(pkg *ast.Package, symbolName string, fset *token.FileSet) string {
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				// Check for function or method declarations
				if d.Name.Name == symbolName {
					return formatFuncDecl(d, fset)
				}
			case *ast.GenDecl:
				// Check for type, var, and const declarations
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.Name == symbolName {
							return formatTypeSpec(s, d, fset)
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if name.Name == symbolName {
								return formatValueSpec(s, d, fset)
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func formatFuncDecl(fn *ast.FuncDecl, fset *token.FileSet) string {
	var sb strings.Builder

	// Add function comments if any
	if fn.Doc != nil {
		for _, comment := range fn.Doc.List {
			sb.WriteString(comment.Text)
			sb.WriteString("\n")
		}
	}

	// Get function signature using nodeToString
	// Clone the function but remove its body to just get the signature
	signatureNode := &ast.FuncDecl{
		Doc:  fn.Doc,
		Recv: fn.Recv,
		Name: fn.Name,
		Type: fn.Type,
		// Leave Body out to just get the signature
	}

	signature := nodeToString(fset, signatureNode)
	sb.WriteString(signature)
	sb.WriteString(" { ... }")

	return sb.String()
}

func formatTypeSpec(typeSpec *ast.TypeSpec, genDecl *ast.GenDecl, fset *token.FileSet) string {
	var sb strings.Builder

	// Add type comments if any
	if genDecl.Doc != nil {
		for _, comment := range genDecl.Doc.List {
			sb.WriteString(comment.Text)
			sb.WriteString("\n")
		}
	}

	// Create a simplified GenDecl with just the type spec
	typeDecl := &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{typeSpec},
	}

	// Get the type declaration using nodeToString
	typeStr := nodeToString(fset, typeDecl)
	sb.WriteString(typeStr)

	return sb.String()
}

func formatValueSpec(valueSpec *ast.ValueSpec, genDecl *ast.GenDecl, fset *token.FileSet) string {
	var sb strings.Builder

	// Add value comments if any
	if genDecl.Doc != nil {
		for _, comment := range genDecl.Doc.List {
			sb.WriteString(comment.Text)
			sb.WriteString("\n")
		}
	}

	// Create a simplified GenDecl with just the value spec
	valueDecl := &ast.GenDecl{
		Tok:   genDecl.Tok,
		Specs: []ast.Spec{valueSpec},
	}

	// Get the value declaration using nodeToString
	valStr := nodeToString(fset, valueDecl)
	sb.WriteString(valStr)

	return sb.String()
}

// nodeToString converts an AST node to its string representation
func nodeToString(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, node)
	return buf.String()
}

