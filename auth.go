package fiberoapi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AuthContext contient les informations d'authentification
type AuthContext struct {
	UserID string                 `json:"user_id"`
	Roles  []string               `json:"roles"`
	Scopes []string               `json:"scopes"`
	Claims map[string]interface{} `json:"claims"`
}

// ResourcePermission définit les permissions sur une ressource
type ResourcePermission struct {
	ResourceType string   `json:"resource_type"`
	ResourceID   string   `json:"resource_id"`
	Actions      []string `json:"actions"` // ["read", "write", "delete", "share"]
}

// AuthorizationService interface pour la vérification des permissions
type AuthorizationService interface {
	// Authentification
	ValidateToken(token string) (*AuthContext, error)

	// Autorisation globale (rôles/scopes)
	HasRole(ctx *AuthContext, role string) bool
	HasScope(ctx *AuthContext, scope string) bool

	// Autorisation dynamique sur les ressources
	CanAccessResource(ctx *AuthContext, resourceType, resourceID, action string) (bool, error)
	GetUserPermissions(ctx *AuthContext, resourceType, resourceID string) (*ResourcePermission, error)
}

// SecurityScheme pour OpenAPI
type SecurityScheme struct {
	Type         string                 `json:"type"`
	Scheme       string                 `json:"scheme,omitempty"`
	BearerFormat string                 `json:"bearerFormat,omitempty"`
	In           string                 `json:"in,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Flows        map[string]interface{} `json:"flows,omitempty"`
}

// GetAuthContext extrait le contexte d'auth depuis Fiber
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

		// Stocker le contexte d'auth dans les locals
		c.Locals("auth", authCtx)
		return c.Next()
	}
}

// RoleGuard middleware pour vérifier les rôles
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

// validateAuthorization valide les autorisations basées sur les tags
func validateAuthorization(c *fiber.Ctx, input interface{}, authService AuthorizationService) error {
	if authService == nil {
		return nil
	}

	// Extraire et valider le token directement
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("authentication required")
	}

	// Vérifier le format Bearer
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Valider le token
	authCtx, err := authService.ValidateToken(token)
	if err != nil {
		return fmt.Errorf("invalid token: %v", err)
	}

	// Stocker le contexte d'auth pour utilisation ultérieure
	c.Locals("auth", authCtx)

	// Analyse les tags d'autorisation dans la struct
	return validateResourceAccess(c, authCtx, input, authService)
}

// validateResourceAccess valide l'accès aux ressources basé sur des tags
func validateResourceAccess(c *fiber.Ctx, authCtx *AuthContext, input interface{}, authService AuthorizationService) error {
	inputValue := reflect.ValueOf(input)
	inputType := reflect.TypeOf(input)

	if inputType.Kind() == reflect.Ptr {
		inputValue = inputValue.Elem()
		inputType = inputType.Elem()
	}

	if inputType.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)

		// Nouveaux tags pour l'autorisation
		if resourceTag := field.Tag.Get("resource"); resourceTag != "" {
			actionTag := field.Tag.Get("action")
			if actionTag == "" {
				actionTag = inferActionFromMethod(c.Method())
			}

			// Obtenir la valeur de l'ID de la ressource
			fieldValue := inputValue.Field(i)
			if fieldValue.Kind() == reflect.String {
				resourceID := fieldValue.String()

				canAccess, err := authService.CanAccessResource(authCtx, resourceTag, resourceID, actionTag)
				if err != nil {
					return fmt.Errorf("authorization check failed: %w", err)
				}

				if !canAccess {
					return fmt.Errorf("insufficient permissions for %s %s on %s", actionTag, resourceTag, resourceID)
				}
			}
		}
	}

	return nil
}

// inferActionFromMethod déduit l'action à partir de la méthode HTTP
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
