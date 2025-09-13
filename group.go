package fiberoapi

import (
	"github.com/gofiber/fiber/v2"
)

// OApiGroup wraps a fiber.Router and adds OpenAPI methods
type OApiGroup struct {
	fiber.Router          // Embedded fiber.Router (includes all standard Fiber methods)
	oapi         *OApiApp // Reference to the parent OApiApp
	prefix       string   // Group prefix for path construction
}

// Implement OApiRouter interface for OApiGroup
func (g *OApiGroup) GetApp() *OApiApp {
	return g.oapi
}

func (g *OApiGroup) GetPrefix() string {
	return g.prefix
}

// Use adds middleware to the OApiGroup
func (g *OApiGroup) Use(middleware fiber.Handler) {
	g.Router.Use(middleware)
}

// Group creates a new OApiGroup that wraps a fiber.Router
func (app *OApiApp) Group(prefix string, handlers ...fiber.Handler) *OApiGroup {
	// Create the actual fiber group
	fiberGroup := app.f.Group(prefix, handlers...)

	return &OApiGroup{
		Router: fiberGroup, // Embed the fiber.Router
		oapi:   app,        // Keep reference to parent app
		prefix: prefix,     // Store prefix for path construction
	}
}

// Group creates a new sub-group within this group
func (g *OApiGroup) Group(prefix string, handlers ...fiber.Handler) *OApiGroup {
	// Create full prefix by combining current prefix with new prefix
	fullPrefix := g.prefix + prefix

	// Create the fiber group from the parent app
	fiberGroup := g.oapi.f.Group(fullPrefix, handlers...)

	return &OApiGroup{
		Router: fiberGroup,
		oapi:   g.oapi,
		prefix: fullPrefix,
	}
}

// Group creates a new group from an OApiRouter (app or group)
func Group(router OApiRouter, prefix string, handlers ...fiber.Handler) *OApiGroup {
	if app, ok := router.(*OApiApp); ok {
		return app.Group(prefix, handlers...)
	} else if group, ok := router.(*OApiGroup); ok {
		return group.Group(prefix, handlers...)
	}
	panic("unsupported router type")
}
