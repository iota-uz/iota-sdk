// Package serve exposes the Lens document contract over HTTP.
//
// Serve intentionally has no authentication or authorization policy. A host
// mounts handlers below its own middleware chain, for example:
//
//	handlers, err := serve.New(serve.Config{
//		Spec:        dashboard,
//		Engine:      executor,
//		Snapshots:   document.NewMemoryStore(30*time.Minute, 128),
//		BasePath:    "/analytics/premium",
//		InlineDepth: 1,
//		Request: func(r *http.Request) runtime.Request {
//			return runtime.Request{
//				Request: r.URL.Query(), DataScope: tenantScope(r.Context()),
//				DataSources: dataSources(r.Context()), DataSourceIdentities: dataSourceIdentities,
//			}
//		},
//	})
//	if err != nil {
//		return err
//	}
//	mux.HandleFunc("/analytics/premium/document", handlers.Document)
//	mux.HandleFunc("/analytics/premium/lens/query", handlers.Query)
//	mux.HandleFunc("/analytics/premium/export", handlers.Export)
//	return authMiddleware(rbacMiddleware(mux))
//
// Progressive mode is the failure-isolated serving contract: Document returns
// a valid shell and each Panel request can fail or retry independently.
// Non-progressive mode is deliberately fail-fast because its document embeds
// every frame and advertises no panel endpoint; one failed panel therefore
// fails Document. Hosts that require sibling panels to survive a datasource
// failure must enable Config.Progressive and mount Handlers.Panel.
//
// Searchable tables use the progressive Panel endpoint and operate on the
// complete cached frame. They must not also use Query's _lp/_lpage/_llimit
// datasource pagination channel: search and datasource paging are separate
// contracts and composing them would produce incomplete search results.
package serve
