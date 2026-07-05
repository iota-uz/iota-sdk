package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/iota-uz/applets"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/helpcenter"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	kbsources "github.com/iota-uz/iota-sdk/pkg/bichat/kb/sources"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/server"
)

func main() {
	bootstrap.Main(run)
}

func run() error {
	src, err := config.Build(envprov.New(".env", ".env.local"))
	if err != nil {
		return fmt.Errorf("failed to build config source: %w", err)
	}

	rt, cleanup, err := bootstrap.NewRuntime(context.Background(), bootstrap.IotaSource(src))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up runtime")
		}
	}()

	previewSearcher, err := buildPreviewHelpCenterSearcher(context.Background())
	if err != nil {
		return fmt.Errorf("failed to build help center preview search index: %w", err)
	}

	if err := rt.Install(
		context.Background(),
		bootstrap.InstallComponents(
			[]composition.Capability{composition.CapabilityAPI, composition.CapabilityWorker},
			append(
				modules.Components(),
				bichat.NewComponent(),
				previewSearchComponent{searcher: previewSearcher},
				helpcenter.NewComponent(helpcenter.ContentConfig{
					Root:          "modules/helpcenter/content",
					Locales:       []string{"en"},
					DefaultLocale: "en",
				}),
			)...,
		),
		bootstrap.InstallHashFS(internalassets.HashFS),
		bootstrap.InstallApplets(bootstrap.AppletsOptions{
			SessionConfig: applets.DefaultSessionConfig,
			WithHTTP:      true,
			WithRuntime:   true,
		}),
		bootstrap.InstallCoreControllers(),
		bootstrap.StartComposition(),
	); err != nil {
		return fmt.Errorf("failed to compose server runtime: %w", err)
	}

	serverInstance, err := server.New(rt)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	httpCfg, err := composition.Resolve[*httpconfig.Config](rt.Container())
	if err != nil {
		return fmt.Errorf("failed to resolve httpconfig: %w", err)
	}
	appCfg, err := composition.Resolve[*appconfig.Config](rt.Container())
	if err != nil {
		return fmt.Errorf("failed to resolve appconfig: %w", err)
	}

	socketAddr := appCfg.SocketAddress(httpCfg.Port)
	log.Printf("Listening on: %s\n", httpCfg.Origin(appCfg))
	if err := serverInstance.Start(socketAddr); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

type previewSearchComponent struct {
	searcher kb.KBSearcher
}

func (c previewSearchComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "helpcenter-preview-search"}
}

func (c previewSearchComponent) LocaleFS() []*embed.FS {
	return nil
}

func (c previewSearchComponent) Build(builder *composition.Builder) error {
	composition.Provide[kb.KBSearcher](builder, c.searcher)
	return nil
}

func buildPreviewHelpCenterSearcher(ctx context.Context) (kb.KBSearcher, error) {
	indexer, searcher, err := kb.NewBleveIndex("var/helpcenter-preview/search.bleve")
	if err != nil {
		return nil, err
	}
	source := kbsources.NewFileSystemSource(
		"modules/helpcenter/content",
		kbsources.WithExtensions(".md"),
		kbsources.WithRecursive(true),
		kbsources.WithExtractTitle(true),
	)
	if err := indexer.Rebuild(ctx, source); err != nil {
		return nil, err
	}
	return searcher, nil
}
