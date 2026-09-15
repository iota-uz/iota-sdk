package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/iota-uz/iota-sdk/internal/solidgen"
)

func main() {
	module := flag.String("module", ".", "Go module root to scan")
	catalog := flag.String("catalog", "web/src/solid-features.generated.ts", "generated TypeScript catalog, relative to module root")
	check := flag.Bool("check", false, "verify generated files without writing")
	watch := flag.Bool("watch", false, "watch Go/TSX graph changes and reconcile generated files")
	flag.Parse()
	config := solidgen.Config{ModuleRoot: *module, Catalog: *catalog, Check: *check}
	if *watch {
		if *check {
			_, _ = fmt.Fprintln(os.Stderr, "-watch and -check cannot be used together")
			os.Exit(2)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err := solidgen.Watch(ctx, config, func(result solidgen.Result, err error) {
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				return
			}
			_, _ = fmt.Fprintf(os.Stdout, "Solid features: %d; generated files: %d\n", result.Features, len(result.Files))
		})
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	result, err := solidgen.Run(config)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = fmt.Fprintf(os.Stdout, "Solid features: %d; generated files: %d\n", result.Features, len(result.Files))
}
