package fiberoapi

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AuthContext contains user authentication details
type AuthContext struct {
	UserID string                 `json:"user_id"`
	Roles  []string               `json:"roles"`
	Scopes []string               `json:"scopes"`
	Claims map[string]interface{} `json:"claims"`
}

// ResourcePermission defines permissions on a resource
type ResourcePermission struct {
	ResourceType string   `json:"resource_type"`
	ResourceID   string   `json:"resource_id"`
	Actions      []string `json:"actions"` // ["read", "write", "delete", "share"]
}

// AuthorizationService interface for permission checks
type AuthorizationService interface {
	// Authentication
	ValidateToken(token string) (*AuthContext, error)

	// Global authorization (roles/scopes)
	HasRole(ctx *AuthContext, role string) bool
	HasScope(ctx *AuthContext, scope string) bool

	// Dynamic authorization on resources
	CanAccessResource(ctx *AuthContext, resourceType, resourceID, action string) (bool, error)
	GetUserPermissions(ctx *AuthContext, resourceType, resourceID string) (*ResourcePermission, error)
}

// SecurityScheme for OpenAPI
type SecurityScheme struct {
	Type         string                 `json:"type"`
	Scheme       string                 `json:"scheme,omitempty"`
	BearerFormat string                 `json:"bearerFormat,omitempty"`
	In           string                 `json:"in,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Flows        map[string]interface{} `json:"flows,omitempty"`
}

// GetAuthContext extracts the authentication context from Fiber
func GetAuthContext(c *fiber.Ctx) (*AuthContext, error) {
	auth, ok := c.Locals("auth").(*AuthContext)
	if !ok {
		return nil, fmt.Errorf("no authentication context found")
	}
	return auth, nil
}

// RequireResourceAccess checks permissions in handlers
func RequireResourceAccess(c *fiber.Ctx, authService AuthorizationService, resourceType, resourceID, action string) error {
	authCtx, err := GetAuthContext(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	canAccess, err := authService.CanAccessResource(authCtx, resourceType, resourceID, action)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Authorization check failed",
			"details": err.Error(),
		})
	}

	if !canAccess {
		return c.Status(403).JSON(fiber.Map{
			"error":    "Insufficient permissions",
			"resource": resourceType,
			"action":   action,
		})
	}

	return nil
}

// BearerTokenMiddleware creates a JWT/Bearer middleware
func BearerTokenMiddleware(validator AuthorizationService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Bearer token required",
			})
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		authCtx, err := validator.ValidateToken(token)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error":   "Invalid token",
				"details": err.Error(),
			})
		}

		// Store auth context for later use
		c.Locals("auth", authCtx)
		return c.Next()
	}
}

// RoleGuard middleware for role verification
func RoleGuard(validator AuthorizationService, requiredRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authCtx, err := GetAuthContext(c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		for _, role := range requiredRoles {
			if !validator.HasRole(authCtx, role) {
				return c.Status(403).JSON(fiber.Map{
					"error":         "Insufficient permissions",
					"required_role": role,
				})
			}
		}

		return c.Next()
	}
}

// validateAuthorization validates permissions based on configured security schemes.
// When SecuritySchemes is empty, it falls back to Bearer-only validation for backward compatibility.
func validateAuthorization(c *fiber.Ctx, input interface{}, authService AuthorizationService, config *Config) error {
	if authService == nil {
		return nil
	}

	// Backward compatibility: if no SecuritySchemes are configured,
	// fall back to Bearer-only validation (original behavior).
	if config == nil || len(config.SecuritySchemes) == 0 {
		authCtx, err := validateBearerToken(c, authService)
		if err != nil {
			return &AuthError{StatusCode: 401, Message: err.Error()}
		}
		c.Locals("auth", authCtx)
		return validateResourceAccess(c, authCtx, input, authService)
	}

	// Multi-scheme validation path
	securityReqs := config.DefaultSecurity
	if len(securityReqs) == 0 {
		securityReqs = buildDefaultFromSchemes(config.SecuritySchemes)
	}

	// Try each security requirement (OR semantics per OpenAPI spec)
	var lastErr error
	for _, requirement := range securityReqs {
		authCtx, err := validateSecurityRequirement(c, requirement, config.SecuritySchemes, authService)
		if err == nil {
			c.Locals("auth", authCtx)
			return validateResourceAccess(c, authCtx, input, authService)
		}
		lastErr = err
	}

	// Propagate typed errors (AuthError, ScopeError) without re-wrapping
	var existingAuthErr *AuthError
	if errors.As(lastErr, &existingAuthErr) {
		return lastErr
	}
	var scopeErr *ScopeError
	if errors.As(lastErr, &scopeErr) {
		return &AuthError{StatusCode: 403, Message: lastErr.Error()}
	}
	return &AuthError{StatusCode: 401, Message: lastErr.Error()}
}

// validateResourceAccess validates resource access based on tags
func validateResourceAccess(c *fiber.Ctx, authCtx *AuthContext, input interface{}, authService AuthorizationService) error {
	inputValue := reflect.ValueOf(input)
	inputType := reflect.TypeOf(input)

	if isPointerType(inputType) {
		inputValue = inputValue.Elem()
		inputType = inputType.Elem()
	}

	if inputType.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)

		// New tags for authorization
		if resourceTag := field.Tag.Get("resource"); resourceTag != "" {
			actionTag := field.Tag.Get("action")
			if actionTag == "" {
				actionTag = inferActionFromMethod(c.Method())
			}

			// Get the resource ID field value
			fieldValue := inputValue.Field(i)
			if fieldValue.Kind() == reflect.String {
				resourceID := fieldValue.String()

				canAccess, err := authService.CanAccessResource(authCtx, resourceTag, resourceID, actionTag)
				if err != nil {
					return &AuthError{StatusCode: 500, Message: fmt.Sprintf("authorization check failed: %v", err)}
				}

				if !canAccess {
					return &AuthError{StatusCode: 403, Message: fmt.Sprintf("insufficient permissions for %s %s on %s", actionTag, resourceTag, resourceID)}
				}
			}
		}
	}

	return nil
}

// inferActionFromMethod infers the action from the HTTP method
func inferActionFromMethod(method string) string {
	switch method {
	case "GET":
		return "read"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "write"
	case "DELETE":
		return "delete"
	default:
		return "read"
	}
}
