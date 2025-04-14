package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/document"
)

func main() {
	sourceDir := flag.String("dir", "./", "Source directory to document")
	outputPath := flag.String("out", "DOCUMENTATION.md", "Output file path")
	recursive := flag.Bool("recursive", false, "Process directories recursively")
	excludeStr := flag.String("exclude", "", "Comma-separated list of directories to exclude")
	flag.Parse()

	var excludeDirs []string
	if *excludeStr != "" {
		excludeDirs = strings.Split(*excludeStr, ",")
	}

	config := document.Config{
		SourceDir:   *sourceDir,
		OutputPath:  *outputPath,
		Recursive:   *recursive,
		ExcludeDirs: excludeDirs,
	}

	err := document.Generate(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Documentation generated successfully at %s", *outputPath)
}
