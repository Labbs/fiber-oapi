package fiberoapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ConditionalAuthMiddleware creates middleware that applies only to specified routes
func ConditionalAuthMiddleware(authMiddleware fiber.Handler, excludePaths ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Verify if the current path is in the exclude list
		for _, excludePath := range excludePaths {
			if path == excludePath || strings.HasPrefix(path, excludePath) {
				return c.Next() // Skip authentication
			}
		}

		// Apply authentication middleware
		return authMiddleware(c)
	}
}

// SmartAuthMiddleware creates middleware that automatically excludes documentation routes
func SmartAuthMiddleware(authService AuthorizationService, config Config) fiber.Handler {
	authMiddleware := BearerTokenMiddleware(authService)

	// Paths to exclude from authentication
	excludePaths := []string{
		config.OpenAPIDocsPath, // /docs
		config.OpenAPIJSONPath, // /openapi.json
		config.OpenAPIYamlPath, // /openapi.yaml
	}

	return ConditionalAuthMiddleware(authMiddleware, excludePaths...)
}
