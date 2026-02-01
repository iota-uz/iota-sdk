package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gorilla/mux"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/internal/server"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

// noopMetrics is a no-op implementation of MetricsRecorder
type noopMetrics struct{}

func (n noopMetrics) RecordDuration(name string, duration time.Duration, labels map[string]string) {}
func (n noopMetrics) IncrementCounter(name string, labels map[string]string)                       {}

// appletControllerWrapper wraps applet.AppletController to implement application.Controller interface
type appletControllerWrapper struct {
	*applet.AppletController
	key string
}

func (w *appletControllerWrapper) Register(r *mux.Router) {
	w.RegisterRoutes(r)
}

func (w *appletControllerWrapper) Key() string {
	return w.key
}

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
	bundle := application.LoadBundle()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(logger),
		Logger:   logger,
		Huber: application.NewHub(&application.HuberOptions{
			Pool:           pool,
			Logger:         logger,
			Bundle:         bundle,
			UserRepository: persistence.NewUserRepository(persistence.NewUploadRepository()),
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}),
	})
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to load modules: %v", err)
	}
	app.RegisterNavItems(modules.NavLinks...)
	app.RegisterHashFsAssets(internalassets.HashFS)

	// Register applet controllers for all registered applets
	appletControllers := make([]application.Controller, 0)
	allApplets := app.AppletRegistry().All()
	for _, registeredApplet := range allApplets {
		// Type assert to full applet.Applet interface
		fullApplet, ok := registeredApplet.(applet.Applet)
		if !ok {
			continue
		}
		appletController := applet.NewAppletController(
			fullApplet,
			bundle,
			applet.DefaultSessionConfig,
			logger,
			noopMetrics{},
		)
		wrapped := &appletControllerWrapper{
			AppletController: appletController,
			key:              "applet_" + registeredApplet.Name(),
		}
		appletControllers = append(appletControllers, wrapped)
	}

	app.RegisterControllers(
		controllers.NewStaticFilesController(app.HashFsAssets()),
		controllers.NewGraphQLController(app),
	)
	app.RegisterControllers(appletControllers...)
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
	log.Printf("Listening on: %s\n", conf.Origin)
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
