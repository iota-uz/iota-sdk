package internal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func New() *App {
	return &App{}
}

type App struct{}

type Arg struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

type Template struct {
	Template string `yaml:"template"`
	Output   string `yaml:"output"`
}

type Command struct {
	Args      map[string]*Arg `yaml:"args"`
	Templates []*Template     `yaml:"templates"`
}

type Config struct {
	Commands map[string]*Command `yaml:"commands"`
}

func GetConfigPath() string {
	if FileExists("codegen.yml") {
		return "codegen.yml"
	}
	if FileExists("codegen.yaml") {
		return "codegen.yaml"
	}
	panic("config file not found. tried codegen.yml and codegen.yaml")
}

func parseTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		return "", err
	}
	return tpl.String(), nil
}

func CommandAction(cmd *Command) cli.ActionFunc {
	return func(c *cli.Context) error {
		var errs []error
		data := map[string]string{}
		for argName, arg := range cmd.Args {
			v := c.String(argName)
			if arg.Required && v == "" {
				return fmt.Errorf("%s is required", argName)
			}
			data[Title(argName)] = v
		}
		for _, tmpl := range cmd.Templates {
			output, err := parseTemplate(tmpl.Output, data)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			outputDir := filepath.Dir(output)
			if err := MkDirIfNone(outputDir); err != nil {
				errs = append(errs, err)
				continue
			}
			errs = append(errs, GenerateFromTemplate(tmpl.Template, output, data, c.Bool("force")))
		}
		return errors.Join(errs...)
	}
}

func (a *App) Run() error {
	confFile := GetConfigPath()
	file, err := os.ReadFile(confFile)
	if err != nil {
		return err
	}
	config := Config{
		Commands: make(map[string]*Command),
	}
	if err := yaml.NewDecoder(bytes.NewBuffer(file)).Decode(config); err != nil {
		return err
	}
	commands := make([]*cli.Command, 0, len(config.Commands))
	for name, command := range config.Commands {
		flags := []cli.Flag{
			&cli.BoolFlag{ //nolint:exhaustruct
				Name:    "force",
				Aliases: []string{"f"},
			},
		}
		for argName, arg := range command.Args {
			flags = append(flags, &cli.StringFlag{ //nolint:exhaustruct
				Name:     argName,
				Aliases:  []string{argName[:1]},
				Required: arg.Required,
			})
		}
		cmd := &cli.Command{ //nolint:exhaustruct
			Name:   name,
			Action: CommandAction(command),
			Flags:  flags,
		}
		commands = append(commands, cmd)
	}
	app := &cli.App{ //nolint:exhaustruct
		Commands: commands,
	}
	return app.Run(os.Args)
}
