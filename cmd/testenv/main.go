// testenv supervises dedicated application processes over a sealed database baseline.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/iota-uz/iota-sdk/pkg/testenv/process"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	template := flag.String("template", "", "sealed baseline database")
	artifacts := flag.String("artifacts", "", "artifact directory")
	revision := flag.String("revision", "", "application build revision")
	ready := flag.String("ready-path", "/login", "application readiness path")
	directory := flag.String("directory", ".", "application working directory")
	schema := flag.String("schema-fingerprint", "", "actual baseline schema fingerprint")
	baseline := flag.String("baseline-fingerprint", "", "actual seeded baseline fingerprint")
	sdkERP := flag.Bool("sdk-erp", false, "enable SDK ERP isolated control routes and ownership probe")
	tokenFile := flag.String("control-token-file", "", "local file containing SDK ERP control token")
	flag.Parse()
	db := dbconfig.Config{Host: env("DB_HOST", "localhost"), Port: env("DB_PORT", "5432"), User: env("DB_USER", "postgres"), Password: env("DB_PASSWORD", "postgres")}
	manifest := testdb.Manifest{SchemaFingerprint: *schema, BaselineFingerprint: *baseline, BuildRevision: *revision}
	settings := process.Config{Command: flag.Args(), Directory: *directory, Artifacts: *artifacts, Revision: *revision, Manifest: manifest, ReadyPath: *ready, Resources: &process.Postgres{Config: db, Template: *template, Manifest: manifest}, Capabilities: []string{"database.clone", "process.dedicated"}}
	if *sdkERP {
		bytes, err := os.ReadFile(*tokenFile)
		if err != nil {
			return err
		}
		token := strings.TrimSpace(string(bytes))
		if len(token) < 32 {
			return fmt.Errorf("control token must have at least 32 bytes")
		}
		settings.Environment = []string{"APP_ENABLETESTENDPOINTS=true", "APP_ENVIRONMENT=development"}
		settings.Configure = func(d testenv.Descriptor) []string {
			return []string{"TESTENV_ENVIRONMENT_ID=" + d.EnvironmentID, "TESTENV_CONTROL_TOKEN=" + token}
		}
		settings.Probe = func(ctx context.Context, d testenv.Descriptor) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.BaseURL+"/__test__/scenarios", nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK || response.Header.Get("X-Test-Environment-Id") != d.EnvironmentID {
				return fmt.Errorf("SDK ERP owner probe failed")
			}
			return nil
		}
	}
	a, err := process.New(settings)
	if err != nil {
		return err
	}
	c := testenv.NewCoordinator(a)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return testenv.Serve(ctx, c, os.Stdin, os.Stdout)
}
