package document

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	SourceDir   string
	OutputPath  string
	Recursive   bool
	ExcludeDirs []string
}

func Generate(config Config) error {
	outputFile, err := os.Create(config.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	fmt.Fprintf(outputFile, "# IOTA SDK Documentation (github.com/iota-uz/iota-sdk)\n\n")
	fmt.Fprintf(outputFile, "Generated automatically from source code.\n\n")

	dirs := []string{config.SourceDir}
	if config.Recursive {
		exclusions := make(map[string]bool)
		for _, dir := range config.ExcludeDirs {
			exclusions[strings.TrimSpace(dir)] = true
		}

		err = filepath.Walk(config.SourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if exclusions[info.Name()] {
					return filepath.SkipDir
				}
				if path != config.SourceDir {
					dirs = append(dirs, path)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk directory tree: %v", err)
		}
	}

	for _, dir := range dirs {
		processDirectory(dir, outputFile)
	}

	log.Printf("Documentation generated successfully at %s", config.OutputPath)
	return nil
}

func processDirectory(dir string, outputFile *os.File) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		log.Printf("Warning: failed to parse packages in %s: %v", dir, err)
		return
	}

	if len(pkgs) == 0 {
		return
	}

	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}

		fmt.Fprintf(outputFile, "## Package `%s` (%s)\n\n", name, dir)

		docPkg := doc.New(pkg, "./", doc.AllDecls)

		if docPkg.Doc != "" {
			fmt.Fprintf(outputFile, "%s\n\n", docPkg.Doc)
		}

		if len(docPkg.Types) > 0 {
			fmt.Fprintf(outputFile, "### Types\n\n")
			for _, t := range docPkg.Types {
				if !isExported(t.Name) {
					continue
				}

				fmt.Fprintf(outputFile, "#### %s\n\n", t.Name)
				if t.Doc != "" {
					fmt.Fprintf(outputFile, "%s\n\n", t.Doc)
				}

				for _, spec := range t.Decl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						switch underlying := typeSpec.Type.(type) {
						case *ast.StructType:
							fields := extractStructFields(underlying)
							if len(fields) > 0 {
								fmt.Fprintf(outputFile, "```go\ntype %s struct {\n", t.Name)
								for _, field := range fields {
									fmt.Fprintf(outputFile, "    %s\n", field)
								}
								fmt.Fprintf(outputFile, "}\n```\n\n")
							}
						case *ast.InterfaceType:
							methods := extractInterfaceMethods(underlying)
							if len(methods) > 0 {
								fmt.Fprintf(outputFile, "##### Interface Methods\n\n")
								for _, method := range methods {
									fmt.Fprintf(outputFile, "- `%s`\n", method)
								}
								fmt.Fprintf(outputFile, "\n")
							}
						}
					}
				}

				if len(t.Methods) > 0 {
					fmt.Fprintf(outputFile, "##### Methods\n\n")
					for _, m := range t.Methods {
						if !isExported(m.Name) {
							continue
						}

						methodSig := getMethodSignature(fset, m, t.Name)
						fmt.Fprintf(outputFile, "- `func %s`\n", methodSig)
						if m.Doc != "" {
							fmt.Fprintf(outputFile, "  %s\n\n", strings.Replace(m.Doc, "\n", "\n  ", -1))
						} else {
							fmt.Fprintf(outputFile, "\n")
						}
					}
				}
			}
		}

		if len(docPkg.Funcs) > 0 {
			fmt.Fprintf(outputFile, "### Functions\n\n")
			for _, f := range docPkg.Funcs {
				if !isExported(f.Name) {
					continue
				}

				sig := getFunctionSignature(fset, f)
				fmt.Fprintf(outputFile, "#### `func %s`\n\n", sig)
				if f.Doc != "" {
					fmt.Fprintf(outputFile, "%s\n\n", f.Doc)
				}
			}
		}

		if len(docPkg.Vars) > 0 || len(docPkg.Consts) > 0 {
			fmt.Fprintf(outputFile, "### Variables and Constants\n\n")

			for _, v := range docPkg.Vars {
				if !hasExportedName(v.Names) {
					continue
				}

				fmt.Fprintf(outputFile, "- Var: `%s`\n", v.Names)
				if v.Doc != "" {
					fmt.Fprintf(outputFile, "  %s\n\n", strings.Replace(v.Doc, "\n", "\n  ", -1))
				} else {
					fmt.Fprintf(outputFile, "\n")
				}
			}

			for _, c := range docPkg.Consts {
				if !hasExportedName(c.Names) {
					continue
				}

				fmt.Fprintf(outputFile, "- Const: `%s`\n", c.Names)
				if c.Doc != "" {
					fmt.Fprintf(outputFile, "  %s\n\n", strings.Replace(c.Doc, "\n", "\n  ", -1))
				} else {
					fmt.Fprintf(outputFile, "\n")
				}
			}
		}

		fmt.Fprintf(outputFile, "---\n\n")
	}
}
