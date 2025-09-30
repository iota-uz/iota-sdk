package middleware

// Example usage of RequireSuperAdmin middleware:
//
// In a controller's Register method:
//
//	func (c *SuperAdminController) Register(router *mux.Router) {
//		// Create a subrouter for superadmin routes
//		s := router.PathPrefix("/superadmin").Subrouter()
//
//		// Apply the superadmin middleware to all routes
//		s.Use(middleware.Authorize())      // Ensure user is authenticated
//		s.Use(middleware.ProvideUser())    // Load user into context
//		s.Use(RequireSuperAdmin())         // Enforce superadmin access
//
//		// Register protected routes
//		s.HandleFunc("/dashboard", c.Dashboard).Methods(http.MethodGet)
//		s.HandleFunc("/tenants", c.ListTenants).Methods(http.MethodGet)
//		s.HandleFunc("/system/settings", c.SystemSettings).Methods(http.MethodGet)
//	}
//
// The middleware chain ensures:
// 1. User is authenticated (has valid session)
// 2. User object is loaded into context
// 3. User type is TypeSuperAdmin (otherwise returns 403 Forbidden)
//
// All handlers registered under this router will only be accessible to superadmin users.
