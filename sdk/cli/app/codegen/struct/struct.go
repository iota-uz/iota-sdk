package _struct

import (
	"go/ast"
	"go/token"
)

func New(receiverName, name string) *Struct {
	return &Struct{
		Name:         name,
		ReceiverName: receiverName,
	}
}

type Struct struct {
	Name         string
	ReceiverName string
	Fields       []*ast.Field
	Methods      []*ast.FuncDecl
}

func (s *Struct) AddField(name, typ string) {
	s.Fields = append(s.Fields, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type:  ast.NewIdent(typ),
	})
}

func (s *Struct) AddMethod(name string, params, results *ast.FieldList, body *ast.BlockStmt) {
	s.Methods = append(s.Methods, &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent(s.ReceiverName)},
					Type:  &ast.StarExpr{X: ast.NewIdent(s.Name)},
				},
			},
		},
		Type: &ast.FuncType{
			Params:  params,
			Results: results,
		},
		Body: body,
	})
}

func (s *Struct) ToStruct() []ast.Decl {
	var decls []ast.Decl
	decls = append(decls, &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(s.Name),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: s.Fields,
					},
				},
			},
		},
	})
	for _, method := range s.Methods {
		decls = append(decls, method)
	}
	return decls
}
