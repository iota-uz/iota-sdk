package file

import (
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/utils/fs"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
)

type File struct {
	Module string
	Force  bool
	fSet   *token.FileSet
	file   *ast.File
}

func FromString(src string) (*File, error) {
	fSet := token.NewFileSet()
	file, err := parser.ParseFile(fSet, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return &File{
		fSet: fSet,
		file: file,
	}, nil
}

func NewCodeGen(module string) *File {
	return &File{
		Module: module,
		fSet:   token.NewFileSet(),
		file: &ast.File{
			Name: ast.NewIdent(module),
		},
	}
}

func (c *File) getImport() *ast.GenDecl {
	for _, decl := range c.file.Decls {
		switch decl.(type) {
		case *ast.GenDecl:
			genDecl := decl.(*ast.GenDecl)
			if genDecl.Tok == token.IMPORT {
				return genDecl
			}
		}
	}
	return nil
}

func (c *File) AddImport(path string) {
	_import := c.getImport()
	spec := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", path),
		},
	}
	if _import == nil {
		c.file.Decls = append(c.file.Decls, &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{spec},
		})
	} else {
		_import.Specs = append(_import.Specs, spec)
	}
}

func (c *File) AddStruct(name string, fields ...*ast.Field) {
	c.file.Decls = append(c.file.Decls, &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(name),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: fields,
					},
				},
			},
		},
	})
}

func (c *File) AddDecl(decl ast.Decl) {
	c.file.Decls = append(c.file.Decls, decl)
}

func (c *File) ToFile(path string) error {
	if !c.Force && fs.FileExists(path) {
		fmt.Printf("File %s already exists, skipping. Use --force to overwrite\n", filepath.Base(path))
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return printer.Fprint(file, c.fSet, c.file)
}
