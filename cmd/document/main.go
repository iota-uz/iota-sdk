package main

import (
	"flag"
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func main() {
	// Parse command-line flags
	sourceDir := flag.String("dir", ".", "Directory path to parse for documentation")
	outputPath := flag.String("output", "DOCUMENTATION.md", "Output file path")
	recursive := flag.Bool("recursive", false, "Whether to process subdirectories recursively")
	excludeDirs := flag.String("exclude", "vendor,node_modules,tmp", "Comma-separated list of directories to exclude")
	flag.Parse()

	// Create output file
	outputFile, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outputFile.Close()

	// Write documentation header
	fmt.Fprintf(outputFile, "# IOTA SDK Documentation\n\n")
	fmt.Fprintf(outputFile, "Generated automatically from source code.\n\n")

	// Get list of directories to process
	dirs := []string{*sourceDir}
	if *recursive {
		// Build exclude list
		excludeList := strings.Split(*excludeDirs, ",")
		exclusions := make(map[string]bool)
		for _, dir := range excludeList {
			exclusions[strings.TrimSpace(dir)] = true
		}

		// Walk directories
		err = filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				// Skip excluded directories
				if exclusions[info.Name()] {
					return filepath.SkipDir
				}
				// Add directory to process list
				if path != *sourceDir {
					dirs = append(dirs, path)
				}
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Failed to walk directory tree: %v", err)
		}
	}

	// Process each directory
	for _, dir := range dirs {
		processDirectory(dir, outputFile)
	}

	log.Printf("Documentation generated successfully at %s", *outputPath)
}

// isExported returns true if the name starts with an uppercase letter (Go's way of identifying exported elements)
func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// hasExportedName checks if any name in the list is exported
func hasExportedName(names []string) bool {
	for _, name := range names {
		if isExported(name) {
			return true
		}
	}
	return false
}

func processDirectory(dir string, outputFile *os.File) {
	fset := token.NewFileSet()

	// Parse the directory, filtering out test files
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		// Skip test files (*_test.go)
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		log.Printf("Warning: failed to parse packages in %s: %v", dir, err)
		return
	}

	// Skip directories with no Go packages
	if len(pkgs) == 0 {
		return
	}

	// Document each package
	for name, pkg := range pkgs {
		// Skip test packages
		if strings.HasSuffix(name, "_test") {
			continue
		}

		fmt.Fprintf(outputFile, "## Package `%s` (%s)\n\n", name, dir)

		docPkg := doc.New(pkg, "./", doc.AllDecls)

		if docPkg.Doc != "" {
			fmt.Fprintf(outputFile, "%s\n\n", docPkg.Doc)
		}

		// Document types (only exported ones)
		if len(docPkg.Types) > 0 {
			fmt.Fprintf(outputFile, "### Types\n\n")
			for _, t := range docPkg.Types {
				// Skip unexported types
				if !isExported(t.Name) {
					continue
				}
				
				fmt.Fprintf(outputFile, "#### type `%s`\n\n", t.Name)
				if t.Doc != "" {
					fmt.Fprintf(outputFile, "%s\n\n", t.Doc)
				}

				// Document methods (only exported ones)
				if len(t.Methods) > 0 {
					fmt.Fprintf(outputFile, "##### Methods\n\n")
					for _, m := range t.Methods {
						// Skip unexported methods
						if !isExported(m.Name) {
							continue
						}
						
						fmt.Fprintf(outputFile, "- `func (%s) %s`\n", t.Name, m.Name)
						if m.Doc != "" {
							fmt.Fprintf(outputFile, "  %s\n\n", strings.Replace(m.Doc, "\n", "\n  ", -1))
						} else {
							fmt.Fprintf(outputFile, "\n")
						}
					}
				}
			}
		}

		// Document functions (only exported ones)
		if len(docPkg.Funcs) > 0 {
			fmt.Fprintf(outputFile, "### Functions\n\n")
			for _, f := range docPkg.Funcs {
				// Skip unexported functions
				if !isExported(f.Name) {
					continue
				}
				
				fmt.Fprintf(outputFile, "#### `func %s`\n\n", f.Name)
				if f.Doc != "" {
					fmt.Fprintf(outputFile, "%s\n\n", f.Doc)
				}
			}
		}

		// Document variables and constants (only exported ones)
		if len(docPkg.Vars) > 0 || len(docPkg.Consts) > 0 {
			fmt.Fprintf(outputFile, "### Variables and Constants\n\n")
			
			for _, v := range docPkg.Vars {
				// Check if any of the variable names are exported
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
				// Check if any of the constant names are exported
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