package internal

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/urfave/cli/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var FuncMap = template.FuncMap{
	"Upper": strings.ToUpper,
	"Lower": strings.ToLower,
	"Title": Title,
}

func Title(str string) string {
	return cases.Title(language.English, cases.NoLower).String(str)
}

func GenerateFromTemplate(src, dst string, data interface{}, force bool) error {
	t, err := template.New(filepath.Base(src)).Funcs(FuncMap).ParseFiles(src)
	if err != nil {
		return err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		return err
	}
	formattedCode, err := format.Source(tpl.Bytes())
	if err != nil {
		return err
	}
	if force {
		return os.WriteFile(dst, formattedCode, 0o644)
	}
	if FileExists(dst) {
		return fmt.Errorf("file %s already exists", filepath.Base(dst))
	}
	return os.WriteFile(dst, formattedCode, 0o644)
}

func (a *App) Generate(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
