package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/internal/server"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			log.Println(r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	conf := configuration.Use()
	logger := conf.Logger()

	cfg := elasticsearch.Config{
		Addresses: []string{conf.ElasticSearch.URL},
		Username:  conf.ElasticSearch.Username,
		Password:  conf.ElasticSearch.Password,
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logger.Fatalf("Error creating the client: %s", err)
	}

	// Ping / Info
	res, err := es.Info()
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		log.Fatalf("Unexpected status code: %s", res.Status())
	}

	var info map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}
	log.Printf("Connected! Elasticsearch version %s",
		info["version"].(map[string]interface{})["number"],
	)

	// Set up OpenTelemetry if enabled
	var tracingCleanup func()
	if conf.OpenTelemetry.Enabled {
		tracingCleanup = logging.SetupTracing(
			context.Background(),
			conf.OpenTelemetry.ServiceName,
			conf.OpenTelemetry.TempoURL,
		)
		defer tracingCleanup()
		logger.Info("OpenTelemetry tracing enabled, exporting to Tempo at " + conf.OpenTelemetry.TempoURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panic(err)
	}
	app := application.New(pool, eventbus.NewEventPublisher(logger))
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to load modules: %v", err)
	}
	app.RegisterNavItems(modules.NavLinks...)
	app.RegisterHashFsAssets(internalassets.HashFS)
	app.RegisterControllers(
		controllers.NewStaticFilesController(app.HashFsAssets()),
		controllers.NewGraphQLController(app),
	)
	options := &server.DefaultOptions{
		Logger:        logger,
		Configuration: conf,
		Application:   app,
		Pool:          pool,
	}
	serverInstance, err := server.Default(options)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	log.Printf("Listening on: %s\n", conf.Address())
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
