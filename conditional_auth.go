package fiberoapi

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ConditionalAuthMiddleware creates middleware that applies only to specified routes
func ConditionalAuthMiddleware(authMiddleware fiber.Handler, excludePaths ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Verify if the current path is in the exclude list
		for _, excludePath := range excludePaths {
			if excludePath != "" && (path == excludePath || strings.HasPrefix(path, excludePath)) {
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

		if lastErr == nil {
			// No security requirements were configured — this is a server misconfiguration,
			// not a client authentication failure.
			return c.Status(500).JSON(fiber.Map{
				"error":   "Server configuration error",
				"details": "no security schemes configured",
			})
		}

		status := 401
		errorLabel := "Authentication failed"
		var authErr *AuthError
		if errors.As(lastErr, &authErr) {
			status = authErr.StatusCode
			if status >= 500 {
				errorLabel = "Server configuration error"
			}
		} else {
			var scopeErr *ScopeError
			if errors.As(lastErr, &scopeErr) {
				status = 403
			}
		}

		return c.Status(status).JSON(fiber.Map{
			"error":   errorLabel,
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
			status, label := classifyAuthError(err)
			return c.Status(status).JSON(fiber.Map{
				"error":   label,
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
			status, label := classifyAuthError(err)
			return c.Status(status).JSON(fiber.Map{
				"error":   label,
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
			status, label := classifyAuthError(err)
			return c.Status(status).JSON(fiber.Map{
				"error":   label,
				"details": err.Error(),
			})
		}

		c.Locals("auth", authCtx)
		return c.Next()
	}
}

// classifyAuthError returns the HTTP status and error label for an authentication error.
func classifyAuthError(err error) (int, string) {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		if authErr.StatusCode >= 500 {
			return authErr.StatusCode, "Server configuration error"
		}
		return authErr.StatusCode, "Authentication failed"
	}
	return 401, "Authentication failed"
}
