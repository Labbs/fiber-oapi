package fiberoapi

// WithSecurity adds security to a route
func WithSecurity(options OpenAPIOptions, security interface{}) OpenAPIOptions {
	options.Security = security
	return options
}

// WithSecurityDisabled disables security for a route
func WithSecurityDisabled(options OpenAPIOptions) OpenAPIOptions {
	options.Security = "disabled"
	return options
}

// WithPermissions adds required permissions for documentation
func WithPermissions(options OpenAPIOptions, permissions ...string) OpenAPIOptions {
	options.RequiredPermissions = append(options.RequiredPermissions, permissions...)
	return options
}

// WithResourceType defines the concerned resource type
func WithResourceType(options OpenAPIOptions, resourceType string) OpenAPIOptions {
	options.ResourceType = resourceType
	return options
}

// SecureGet defines a GET route with authentication
func SecureGet[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Add security metadata
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Get(router, path, handler, options)
}

// SecurePost defines a POST route with authentication
func SecurePost[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Add security metadata
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Post(router, path, handler, options)
}

// SecurePut defines a PUT route with authentication
func SecurePut[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Add security metadata
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Put(router, path, handler, options)
}

// SecureDelete defines a DELETE route with authentication
func SecureDelete[TInput any, TOutput any, TError any](
	router OApiRouter,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
	requiredPermissions ...string,
) {
	// Add security metadata
	options = WithSecurity(options, map[string][]string{
		"bearerAuth": {},
	})
	options = WithPermissions(options, requiredPermissions...)

	Delete(router, path, handler, options)
}
