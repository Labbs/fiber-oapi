package fiberoapi

// WithSecurity adds security to a route
func WithSecurity(options OpenAPIOptions, security interface{}) OpenAPIOptions {
	options.Security = security
	return options
}

// WithSecurityDisabled désactive la sécurité pour une route
func WithSecurityDisabled(options OpenAPIOptions) OpenAPIOptions {
	options.Security = "disabled"
	return options
}

// WithPermissions ajoute les permissions requises pour documentation
func WithPermissions(options OpenAPIOptions, permissions ...string) OpenAPIOptions {
	options.RequiredPermissions = append(options.RequiredPermissions, permissions...)
	return options
}

// WithResourceType defines the concerned resource type
func WithResourceType(options OpenAPIOptions, resourceType string) OpenAPIOptions {
	options.ResourceType = resourceType
	return options
}

// SecureGet définit une route GET avec authentification
func SecureGet[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Ajouter les métadonnées de sécurité
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Get(router, path, handler, options)
}

// SecurePost définit une route POST avec authentification
func SecurePost[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Ajouter les métadonnées de sécurité
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Post(router, path, handler, options)
}

// SecurePut définit une route PUT avec authentification
func SecurePut[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Ajouter les métadonnées de sécurité
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Put(router, path, handler, options)
}

// SecureDelete définit une route DELETE avec authentification
func SecureDelete[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Ajouter les métadonnées de sécurité
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Delete(router, path, handler, options)
}
