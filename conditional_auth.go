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

// SmartAuthMiddleware creates middleware that automatically excludes documentation routes.
// When SecuritySchemes are configured, it uses MultiSchemeAuthMiddleware for dispatch.
// Otherwise, it falls back to BearerTokenMiddleware for backward compatibility.
func SmartAuthMiddleware(authService AuthorizationService, config Config) fiber.Handler {
	var authMiddleware fiber.Handler
	if len(config.SecuritySchemes) > 0 {
		authMiddleware = MultiSchemeAuthMiddleware(authService, config)
	} else {
		authMiddleware = BearerTokenMiddleware(authService)
	}

	// Paths to exclude from authentication
	excludePaths := []string{
		config.OpenAPIDocsPath, // /docs
		config.OpenAPIJSONPath, // /openapi.json
		config.OpenAPIYamlPath, // /openapi.yaml
	}

	return ConditionalAuthMiddleware(authMiddleware, excludePaths...)
}

// MultiSchemeAuthMiddleware creates middleware that tries configured security schemes.
// It iterates over DefaultSecurity requirements (OR semantics) and validates
// using the appropriate scheme handler.
func MultiSchemeAuthMiddleware(authService AuthorizationService, config Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		securityReqs := config.DefaultSecurity
		if len(securityReqs) == 0 {
			securityReqs = buildDefaultFromSchemes(config.SecuritySchemes)
		}

		var lastErr error
		for _, requirement := range securityReqs {
			authCtx, err := validateSecurityRequirement(c, requirement, config.SecuritySchemes, authService)
			if err == nil {
				c.Locals("auth", authCtx)
				return c.Next()
			}
			lastErr = err
		}

		return c.Status(401).JSON(fiber.Map{
			"error":   "Authentication failed",
			"details": lastErr.Error(),
		})
	}
}

// BasicAuthMiddleware creates a standalone middleware for HTTP Basic authentication.
// The authService must implement the BasicAuthValidator interface.
func BasicAuthMiddleware(validator AuthorizationService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authCtx, err := validateBasicAuth(c, validator)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error":   "Basic authentication failed",
				"details": err.Error(),
			})
		}

		c.Locals("auth", authCtx)
		return c.Next()
	}
}

// APIKeyMiddleware creates a standalone middleware for API Key authentication.
// The authService must implement the APIKeyValidator interface.
func APIKeyMiddleware(validator AuthorizationService, scheme SecurityScheme) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authCtx, err := validateAPIKey(c, scheme, validator)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error":   "API Key authentication failed",
				"details": err.Error(),
			})
		}

		c.Locals("auth", authCtx)
		return c.Next()
	}
}

// AWSSignatureMiddleware creates a standalone middleware for AWS Signature V4 authentication.
// The authService must implement the AWSSignatureValidator interface.
func AWSSignatureMiddleware(validator AuthorizationService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authCtx, err := validateAWSSigV4(c, validator)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error":   "AWS Signature V4 authentication failed",
				"details": err.Error(),
			})
		}

		c.Locals("auth", authCtx)
		return c.Next()
	}
}
