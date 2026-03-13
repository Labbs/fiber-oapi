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

// WithRoles adds required roles to a route with OR semantics (user needs at least one)
func WithRoles(options OpenAPIOptions, roles ...string) OpenAPIOptions {
	options.RequiredRoles = append(options.RequiredRoles, roles...)
	return options
}

// WithAllRoles adds required roles to a route with AND semantics (user needs all of them)
func WithAllRoles(options OpenAPIOptions, roles ...string) OpenAPIOptions {
	options.RequiredRoles = append(options.RequiredRoles, roles...)
	options.RequireAllRoles = true
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
