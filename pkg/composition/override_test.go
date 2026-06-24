package composition

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

func describedID(t *testing.T, controller application.Controller) string {
	t.Helper()
	described, ok := controller.(application.DescribedController)
	require.True(t, ok)
	return described.Descriptor().ID
}

// overrideRepo is a minimal concrete type used to exercise
// ProvideDefault/RemoveProvider semantics.
type overrideRepo struct{ label string }

func TestProvideDefault_OverriddenByConcreteInSameBuilder(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "combo"},
		build: func(builder *Builder) error {
			// Default registered first.
			ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
				return &overrideRepo{label: "default"}
			})
			// Plain Provide for the same key must silently win.
			Provide[*overrideRepo](builder, func() *overrideRepo {
				return &overrideRepo{label: "concrete"}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "concrete", resolved.label)
}

func TestProvideDefault_DownstreamConcreteWinsAcrossComponents(t *testing.T) {
	defaultCalls := 0
	concreteCalls := 0

	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
					defaultCalls++
					return &overrideRepo{label: "default"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				Provide[*overrideRepo](builder, func() *overrideRepo {
					concreteCalls++
					return &overrideRepo{label: "concrete"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "concrete", resolved.label)

	// The default factory must never run — the whole point of the override
	// mechanism is that resolved values come from the winning provider only.
	require.Equal(t, 0, defaultCalls, "default factory must not run after override")
	require.Equal(t, 1, concreteCalls, "concrete factory runs exactly once")
}

func TestProvideDefault_OnlyDefaultResolves_WhenNoOverride(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "solo"},
		build: func(builder *Builder) error {
			ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
				return &overrideRepo{label: "default"}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "default", resolved.label)
}

func TestProvideDefault_TwoDefaultsCollide_Error(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "alpha"},
			build: func(builder *Builder) error {
				ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
					return &overrideRepo{label: "alpha"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "beta", Requires: []string{"alpha"}},
			build: func(builder *Builder) error {
				ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
					return &overrideRepo{label: "beta"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate default provider")
	require.Contains(t, err.Error(), `"alpha"`)
	require.Contains(t, err.Error(), `"beta"`)
}

func TestRemoveProvider_ReplacesUpstreamNonDefault(t *testing.T) {
	upstreamCalls := 0
	downstreamCalls := 0

	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				// Upstream does NOT mark this overridable — downstream
				// still needs the escape hatch.
				Provide[*overrideRepo](builder, func() *overrideRepo {
					upstreamCalls++
					return &overrideRepo{label: "upstream"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				RemoveProvider[*overrideRepo](builder)
				Provide[*overrideRepo](builder, func() *overrideRepo {
					downstreamCalls++
					return &overrideRepo{label: "downstream"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "downstream", resolved.label)
	require.Equal(t, 0, upstreamCalls, "removed provider must not run")
	require.Equal(t, 1, downstreamCalls)
}

func TestRemoveProvider_NoopForMissingKey(t *testing.T) {
	// RemoveProvider for a key that was never provided should be a no-op,
	// not an error — downstream should be able to write defensive removals
	// without probing the container first.
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			RemoveProvider[*overrideRepo](builder)
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.NoError(t, err)
}

func TestRemoveProvider_WithoutReplacement_ResolvesToNotProvided(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				Provide[*overrideRepo](builder, func() *overrideRepo {
					return &overrideRepo{label: "upstream"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				// Remove without providing a replacement — the engine
				// should surface NOT_PROVIDED when something resolves T.
				RemoveProvider[*overrideRepo](builder)
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	_, err = Resolve[*overrideRepo](container)
	require.Error(t, err)
	require.True(t, IsNotProvided(err))
}

// ----- Controller descriptors -----

type overrideCtrl struct {
	id       string
	label    string
	order    int
	replaces []string
	routes   []application.RouteSpec
	nav      []application.NavNode
}

func (c *overrideCtrl) Descriptor() application.ControllerDescriptor {
	return application.ControllerDescriptor{
		ID:       c.id,
		Order:    c.order,
		Replaces: c.replaces,
		Routes:   c.routes,
		Nav:      c.nav,
	}
}

func (c *overrideCtrl) Register(_ *mux.Router) {}

func TestControllerDescriptors_MustHaveUniqueIDs(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{id: "settings", label: "upstream"}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{id: "settings", label: "downstream"}}, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `duplicate controller ID "settings"`)
	require.Contains(t, err.Error(), `"upstream"`)
	require.Contains(t, err.Error(), `"downstream"`)
}

func TestControllerDescriptors_AllowExplicitReplacement(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{id: "core.settings", label: "upstream"}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{
						id:       "custom.settings",
						label:    "downstream",
						replaces: []string{"core.settings"},
					}}, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	ctrls := container.Controllers()
	require.Len(t, ctrls, 1)
	ctrl, ok := ctrls[0].(*overrideCtrl)
	require.True(t, ok)
	require.Equal(t, "downstream", ctrl.label)
}

func TestControllerDescriptors_FailForMissingReplacement(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{&overrideCtrl{
					id:       "custom.settings",
					replaces: []string{"core.settings"},
				}}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `replaces missing controller "core.settings"`)
}

func TestControllerDescriptors_AllowNestedDistinctRoutes(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "hub"},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{
						id:     "settings.hub",
						routes: []application.RouteSpec{application.Get("/settings")},
					}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "logo", Requires: []string{"hub"}},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{
						id:     "settings.logo",
						routes: []application.RouteSpec{application.Get("/settings/logo")},
					}}, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	ctrls := container.Controllers()
	require.Len(t, ctrls, 2)
	require.Equal(t, "settings.hub", describedID(t, ctrls[0]))
	require.Equal(t, "settings.logo", describedID(t, ctrls[1]))
}

func TestControllerDescriptors_DetectDuplicateSurvivingRoutes(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "a"},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{
						id:     "a",
						routes: []application.RouteSpec{application.Get("/settings")},
					}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "b", Requires: []string{"a"}},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{&overrideCtrl{
						id:     "b",
						routes: []application.RouteSpec{application.Get("/settings")},
					}}, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "controller route collision")
}

func TestControllerDescriptors_AllowDifferentMethodsAndHosts(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{
					&overrideCtrl{id: "get", routes: []application.RouteSpec{application.Get("/settings")}},
					&overrideCtrl{id: "post", routes: []application.RouteSpec{application.Post("/settings")}},
					&overrideCtrl{id: "host-a", routes: []application.RouteSpec{application.WithHost("a.example", application.Get("/hosted"))}},
					&overrideCtrl{id: "host-b", routes: []application.RouteSpec{application.WithHost("b.example", application.Get("/hosted"))}},
				}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.NoError(t, err)
}

func TestControllerDescriptors_DetectPrefixOverlap(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{
					&overrideCtrl{id: "assets", routes: []application.RouteSpec{application.Prefix("/assets")}},
					&overrideCtrl{id: "asset-file", routes: []application.RouteSpec{application.Get("/assets/app.js")}},
				}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "controller route collision")
}

func TestControllerDescriptors_OrderControlsFinalControllerOrder(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{
					&overrideCtrl{id: "last", order: 10},
					&overrideCtrl{id: "first", order: -10},
					&overrideCtrl{id: "middle", order: 0},
				}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	ctrls := container.Controllers()
	require.Len(t, ctrls, 3)
	require.Equal(t, "first", describedID(t, ctrls[0]))
	require.Equal(t, "middle", describedID(t, ctrls[1]))
	require.Equal(t, "last", describedID(t, ctrls[2]))
}

func TestNavModel_ProjectsDescriptorNavAndQuickLinks(t *testing.T) {
	viewReports := permission.New(
		permission.WithName("reports.view"),
		permission.WithResource("reports"),
		permission.WithAction(permission.ActionRead),
	)
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "reports"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{&overrideCtrl{
					id:     "reports.index",
					routes: []application.RouteSpec{application.Get("/reports", application.RequireAll(viewReports))},
					nav: []application.NavNode{{
						ID:       "reports.index",
						TitleKey: "NavigationLinks.Reports",
						Path:     "/reports?tab=all",
						Keywords: []string{"NavigationLinks.Reports.Keyword"},
					}},
				}}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	items := container.NavItems()
	require.Len(t, items, 1)
	require.Equal(t, "reports.index", items[0].Key)
	require.Equal(t, "/reports?tab=all", items[0].Href)
	require.Equal(t, []permission.Permission{viewReports}, items[0].Permissions)

	docs, err := spotlight.CollectDocuments(context.Background(), spotlightQuickLinks(container.QuickLinks()), spotlight.ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "/reports?tab=all", docs[0].URL)
	require.Equal(t, []string{"reports.view"}, docs[0].Access.AllowedPermissions)
	require.Equal(t, spotlight.PermissionLogicAll, docs[0].Access.PermissionLogic)
}

func TestNavModel_RejectsDuplicateIDs(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "nav"},
		build: func(builder *Builder) error {
			AddNavNodes(builder,
				application.NavNode{ID: "nav.reports", TitleKey: "Reports"},
				application.NavNode{ID: "nav.reports", TitleKey: "Reports 2"},
			)
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `duplicate nav node ID "nav.reports"`)
}

func TestNavModel_RejectsUnresolvedPath(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "nav"},
		build: func(builder *Builder) error {
			AddNavNodes(builder, application.NavNode{
				ID:       "nav.missing",
				TitleKey: "Missing",
				Path:     "/missing",
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `path "/missing" does not resolve to a controller route`)
}

func TestNavModel_SameModuleOrphanFailsAndCrossModuleOrphanSkips(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "nav"},
		build: func(builder *Builder) error {
			AddNavNodes(builder, application.NavNode{
				ID:       "finance.expenses",
				Parent:   "finance.missing",
				TitleKey: "Expenses",
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `parent "finance.missing" was not contributed`)

	engine = NewEngine()
	err = engine.Register(testComponent{
		descriptor: Descriptor{Name: "nav"},
		build: func(builder *Builder) error {
			AddNavNodes(builder, application.NavNode{
				ID:       "finance.expenses",
				Parent:   "core.finance",
				TitleKey: "Expenses",
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)
	require.Empty(t, container.NavItems())
}

func TestRouteAuthLookup_MatchesMuxVariables(t *testing.T) {
	viewReports := permission.New(
		permission.WithName("reports.view"),
		permission.WithResource("reports"),
		permission.WithAction(permission.ActionRead),
	)
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "reports"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{&overrideCtrl{
					id:     "reports.show",
					routes: []application.RouteSpec{application.Get("/reports/{id}", application.RequireAny(viewReports))},
				}}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	policy, ok := container.AuthPolicyForRoute("GET", "example.test", "/reports/123")
	require.True(t, ok)
	require.Equal(t, application.PermissionLogicAny, policy.Logic)
	require.Equal(t, []permission.Permission{viewReports}, policy.Permissions)
}

func TestNavModel_RequireAnyProjectsAnyLogicToSidebarAndSpotlight(t *testing.T) {
	viewA := permission.New(
		permission.WithName("reports.a"),
		permission.WithResource("reports"),
		permission.WithAction(permission.ActionRead),
	)
	viewB := permission.New(
		permission.WithName("reports.b"),
		permission.WithResource("reports"),
		permission.WithAction(permission.ActionRead),
	)
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "reports"},
		build: func(builder *Builder) error {
			ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
				return []application.Controller{&overrideCtrl{
					id:     "reports.index",
					routes: []application.RouteSpec{application.Get("/reports", application.RequireAny(viewA, viewB))},
					nav: []application.NavNode{{
						ID:       "reports.index",
						TitleKey: "NavigationLinks.Reports",
						Path:     "/reports",
					}},
				}}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	items := container.NavItems()
	require.Len(t, items, 1)
	require.Equal(t, types.PermissionLogicAny, items[0].Logic)

	docs, err := spotlight.CollectDocuments(context.Background(), spotlightQuickLinks(container.QuickLinks()), spotlight.ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, spotlight.PermissionLogicAny, docs[0].Access.PermissionLogic)
}

func TestNavModel_RejectsCycle(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "nav"},
		build: func(builder *Builder) error {
			AddNavNodes(builder,
				application.NavNode{ID: "nav.a", Parent: "nav.b", TitleKey: "A"},
				application.NavNode{ID: "nav.b", Parent: "nav.a", TitleKey: "B"},
			)
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nav cycle detected")
}

func TestNavModel_PrunesDeepOrphanChains(t *testing.T) {
	// mid's parent belongs to an absent (optional) module, so it is skipped
	// rather than failing; child points at mid and must be pruned too.
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "nav"},
		build: func(builder *Builder) error {
			AddNavNodes(builder,
				application.NavNode{ID: "alpha.mid", Parent: "beta.root", TitleKey: "Mid"},
				application.NavNode{ID: "alpha.child", Parent: "alpha.mid", TitleKey: "Child"},
			)
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)
	require.Empty(t, container.NavItems())
}

func spotlightQuickLinks(links []*spotlight.QuickLink) *spotlight.QuickLinks {
	quickLinks := spotlight.NewQuickLinks(nil, nil)
	quickLinks.Add(links...)
	return quickLinks
}

// ----- RemoveHook -----

func TestRemoveHook_FiltersByName(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				// noopStop exists so the hook Start closures can return
				// an explicit no-op StopFn instead of tripping the nilnil
				// linter with a double-nil return.
				noopStop := func(context.Context) error { return nil }
				ContributeHooks(builder, func(*Container) ([]Hook, error) {
					return []Hook{
						{
							Name: "keep",
							Start: func(context.Context) (StopFn, error) {
								return noopStop, nil
							},
						},
						{
							Name: "drop",
							Start: func(context.Context) (StopFn, error) {
								t.Fatalf("removed hook Start must not run")
								return noopStop, nil
							},
						},
					}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				RemoveHook(builder, "drop")
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	require.Len(t, container.Hooks(), 1)
	require.Equal(t, "keep", container.Hooks()[0].Name)

	// Start/Stop exercise the filtered hook list — if the "drop" hook
	// survived, its Start would t.Fatalf above.
	require.NoError(t, Start(context.Background(), container))
	require.NoError(t, Stop(context.Background(), container))
}

// ----- ProvideDefaultAs -----

// defaultImpl satisfies greetingPort (defined in fixtures_test.go) so we can
// exercise the interface-bridging variant of ProvideDefault.
type defaultImpl struct{ value string }

func (d *defaultImpl) Greet() string { return d.value }

func TestProvideDefaultAs_BridgesAndIsOverridable(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ProvideDefaultAs[greetingPort, *defaultImpl](builder, func() *defaultImpl {
					return &defaultImpl{value: "default"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				// Replace just the concrete; the interface bridge must
				// continue to resolve via the new concrete value.
				Provide[*defaultImpl](builder, func() *defaultImpl {
					return &defaultImpl{value: "override"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	concrete, err := Resolve[*defaultImpl](container)
	require.NoError(t, err)
	require.Equal(t, "override", concrete.value)

	// The interface key still resolves, and (because of the bridge) it
	// points at the same overridden concrete instance.
	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "override", port.Greet())
	require.Same(t, concrete, port.(*defaultImpl))
}

func TestProvideDefaultAs_InterfaceKeyCanAlsoBeRemoved(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ProvideDefaultAs[greetingPort, *defaultImpl](builder, func() *defaultImpl {
					return &defaultImpl{value: "default"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				// Remove the concrete via RemoveProvider, replace with a
				// completely different struct that also satisfies the
				// interface. The interface bridge from the upstream will
				// have been removed too (it was overridable), so we need
				// to provide our own for the interface key.
				RemoveProvider[*defaultImpl](builder)
				RemoveProvider[greetingPort](builder)
				Provide[greetingPort](builder, func() greetingPort {
					return &defaultImpl{value: "replacement"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "replacement", port.Greet())
}

// ----- ProvideAs / ProvideDefaultAs fail-fast guards -----

type notAnInterface struct{}

type notImplementingGreeting struct{}

func TestProvideAs_PanicsWhenTargetNotInterface(t *testing.T) {
	require.PanicsWithValue(t,
		"composition: ProvideAs target must be an interface, got composition.notAnInterface (struct)",
		func() {
			_ = compileWithBuild(t, func(builder *Builder) error {
				// `notAnInterface` is a concrete struct, not an interface.
				ProvideAs[notAnInterface, *defaultImpl](builder, &defaultImpl{})
				return nil
			})
		},
		"ProvideAs must reject a non-interface target type",
	)
}

func TestProvideAs_PanicsWhenConcreteDoesNotImplement(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			// *notImplementingGreeting does not have a Greet() method, so
			// it cannot satisfy greetingPort.
			ProvideAs[greetingPort, *notImplementingGreeting](builder, &notImplementingGreeting{})
			return nil
		})
	})
}

func TestProvideDefaultAs_PanicsWhenTargetNotInterface(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			ProvideDefaultAs[notAnInterface, *defaultImpl](builder, func() *defaultImpl {
				return &defaultImpl{}
			})
			return nil
		})
	})
}
