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
