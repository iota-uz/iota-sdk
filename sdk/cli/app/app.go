package app

import (
	"bytes"
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/cli/app/codegen/file"
	"github.com/iota-agency/iota-erp/sdk/cli/app/templates"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"github.com/urfave/cli/v2"
	"go/ast"
	"log"
	"os"
	"path/filepath"
	"text/template"
)

func New() *App {
	return &App{}
}

type App struct {
}

func GenerateFromTemplate(src, dst string, data interface{}) error {
	t, err := template.ParseFiles(src)
	if err != nil {
		return err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		return err
	}
	codegen, err := file.FromString(tpl.String())
	if err != nil {
		return err
	}
	return codegen.ToFile(dst)
}

func (a *App) GenerateDomain(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("name is required")
	}
	codegen := file.NewCodeGen(name)
	codegen.Force = c.Bool("force")
	codegen.AddImport("time")

	domainFields := []*ast.Field{
		{
			Names: []*ast.Ident{ast.NewIdent("Id")},
			Type:  ast.NewIdent("int64"),
		},
		{
			Names: []*ast.Ident{ast.NewIdent("CreatedAt")},
			Type:  ast.NewIdent("time.Time"),
		},
		{
			Names: []*ast.Ident{ast.NewIdent("UpdatedAt")},
			Type:  ast.NewIdent("time.Time"),
		},
	}
	codegen.AddStruct(sequence.Title(name), domainFields...)
	codegen.AddDecl(&ast.FuncDecl{
		Name: ast.NewIdent("ToGraph"),
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent(string(name[0]))},
					Type:  &ast.StarExpr{X: ast.NewIdent(sequence.Title(name))},
				},
			},
		},
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
		},
		Body: &ast.BlockStmt{},
	})
	basePath := "internal/domain"
	if err := os.MkdirAll(fmt.Sprintf("%s/%s", basePath, name), 0755); err != nil {
		return err
	}
	return codegen.ToFile(fmt.Sprintf("%s/%s/%s.go", basePath, name, name))
}

func (a *App) GenerateRepository(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("name is required")
	}
	opts := templates.RepoOptions{
		Domain:     name,
		DomainType: fmt.Sprintf("%s.%s", name, sequence.Title(name)),
		Struct:     fmt.Sprintf("%sRepository", sequence.Title(name)),
	}
	// TODO: Proper file path resolution
	basePath := "internal/infrastracture/persistence"
	src := "sdk/cli/app/templates/repo.templ"
	dst := filepath.Join(basePath, fmt.Sprintf("%s_repository.go", name))
	return GenerateFromTemplate(src, dst, opts)
}

func (a *App) GenerateService(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("name is required")
	}
	opts := templates.ServiceOptions{
		Domain:     name,
		DomainType: sequence.Title(name),
	}
	return GenerateFromTemplate("sdk/cli/app/templates/service.templ", fmt.Sprintf("internal/domain/%s/%s_service.go", name, name), opts)
}

func (a *App) Generate(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if err := a.GenerateDomain(c); err != nil {
		return err
	}
	if err := a.GenerateService(c); err != nil {
		return err
	}
	return a.GenerateRepository(c)
}

func (a *App) Run() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "generate",
				Usage:   "Generate code",
				Aliases: []string{"g"},
				Args:    true,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
					},
				},
				Action: func(c *cli.Context) error {
					return a.Generate(c)
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
