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
package serve
