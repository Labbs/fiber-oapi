package fiberoapi

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestBearerTokenMiddleware(t *testing.T) {
	authService := NewMockAuthService()
	middleware := BearerTokenMiddleware(authService)

	t.Run("Valid token sets auth context", func(t *testing.T) {
		app := fiber.New()
		app.Use(middleware)
		app.Get("/test", func(c *fiber.Ctx) error {
			authCtx, err := GetAuthContext(c)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{
				"user_id": authCtx.UserID,
				"roles":   authCtx.Roles,
			})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid token returns error", func(t *testing.T) {
		app := fiber.New()
		app.Use(middleware)
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"message": "should not reach here"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		resp, err := app.Test(req)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 401 {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Missing authorization header returns error", func(t *testing.T) {
		app := fiber.New()
		app.Use(middleware)
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"message": "should not reach here"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 401 {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})
}

func TestConditionalAuthMiddleware(t *testing.T) {
	authService := NewMockAuthService()
	authMiddleware := BearerTokenMiddleware(authService)

	t.Run("Excluded paths skip authentication", func(t *testing.T) {
		conditionalMiddleware := ConditionalAuthMiddleware(authMiddleware, "/docs", "/health")

		app := fiber.New()
		app.Use(conditionalMiddleware)
		app.Get("/docs", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"message": "docs page"})
		})
		app.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok"})
		})
		app.Get("/protected", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"message": "protected"})
		})

		// Test excluded path /docs
		req := httptest.NewRequest("GET", "/docs", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Expected /docs to be accessible without auth, got %d", resp.StatusCode)
		}

		// Test excluded path /health
		req = httptest.NewRequest("GET", "/health", nil)
		resp, err = app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Expected /health to be accessible without auth, got %d", resp.StatusCode)
		}

		// Test protected path without auth
		req = httptest.NewRequest("GET", "/protected", nil)
		resp, err = app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != 401 {
			t.Errorf("Expected /protected to require auth, got %d", resp.StatusCode)
		}

		// Test protected path with auth
		req = httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err = app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Expected /protected to work with auth, got %d", resp.StatusCode)
		}
	})
}

func TestAuthHelpers(t *testing.T) {
	t.Run("WithSecurity sets security options", func(t *testing.T) {
		options := OpenAPIOptions{Summary: "Test"}
		security := []map[string][]string{{"bearerAuth": {}}}

		result := WithSecurity(options, security)

		if result.Security == nil {
			t.Error("Expected security to be set")
		}
	})

	t.Run("WithSecurityDisabled disables security", func(t *testing.T) {
		options := OpenAPIOptions{Summary: "Test"}

		result := WithSecurityDisabled(options)

		if result.Security != "disabled" {
			t.Errorf("Expected security to be 'disabled', got %v", result.Security)
		}
	})

	t.Run("WithPermissions adds permissions", func(t *testing.T) {
		options := OpenAPIOptions{Summary: "Test"}

		result := WithPermissions(options, "read", "write")

		if len(result.RequiredPermissions) != 2 {
			t.Errorf("Expected 2 permissions, got %d", len(result.RequiredPermissions))
		}
		if result.RequiredPermissions[0] != "read" || result.RequiredPermissions[1] != "write" {
			t.Errorf("Expected permissions [read, write], got %v", result.RequiredPermissions)
		}
	})

	t.Run("WithResourceType sets resource type", func(t *testing.T) {
		options := OpenAPIOptions{Summary: "Test"}

		result := WithResourceType(options, "document")

		if result.ResourceType != "document" {
			t.Errorf("Expected ResourceType to be 'document', got %s", result.ResourceType)
		}
	})
}

func TestAuthContextValidation(t *testing.T) {
	t.Run("Valid AuthContext", func(t *testing.T) {
		ctx := &AuthContext{
			UserID: "user-123",
			Roles:  []string{"user", "admin"},
			Scopes: []string{"read", "write"},
			Claims: map[string]interface{}{
				"sub": "user-123",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		}

		if ctx.UserID != "user-123" {
			t.Errorf("Expected UserID user-123, got %s", ctx.UserID)
		}
		if len(ctx.Roles) != 2 {
			t.Errorf("Expected 2 roles, got %d", len(ctx.Roles))
		}
		if len(ctx.Scopes) != 2 {
			t.Errorf("Expected 2 scopes, got %d", len(ctx.Scopes))
		}
	})
}

func TestSecurityScheme(t *testing.T) {
	t.Run("Bearer token security scheme", func(t *testing.T) {
		scheme := SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "JWT Bearer token",
		}

		if scheme.Type != "http" {
			t.Errorf("Expected type http, got %s", scheme.Type)
		}
		if scheme.Scheme != "bearer" {
			t.Errorf("Expected scheme bearer, got %s", scheme.Scheme)
		}
		if scheme.BearerFormat != "JWT" {
			t.Errorf("Expected bearer format JWT, got %s", scheme.BearerFormat)
		}
	})

	t.Run("API Key security scheme", func(t *testing.T) {
		scheme := SecurityScheme{
			Type:        "apiKey",
			In:          "header",
			Name:        "X-API-Key",
			Description: "API Key authentication",
		}

		if scheme.Type != "apiKey" {
			t.Errorf("Expected type apiKey, got %s", scheme.Type)
		}
		if scheme.In != "header" {
			t.Errorf("Expected in header, got %s", scheme.In)
		}
		if scheme.Name != "X-API-Key" {
			t.Errorf("Expected name X-API-Key, got %s", scheme.Name)
		}
	})
}

func TestResourcePermission(t *testing.T) {
	t.Run("Resource permission structure", func(t *testing.T) {
		perm := &ResourcePermission{
			ResourceType: "document",
			ResourceID:   "doc-123",
			Actions:      []string{"read", "write", "delete"},
		}

		if perm.ResourceType != "document" {
			t.Errorf("Expected ResourceType document, got %s", perm.ResourceType)
		}
		if perm.ResourceID != "doc-123" {
			t.Errorf("Expected ResourceID doc-123, got %s", perm.ResourceID)
		}
		if len(perm.Actions) != 3 {
			t.Errorf("Expected 3 actions, got %d", len(perm.Actions))
		}
	})
}

func TestMockAuthService(t *testing.T) {
	authService := NewMockAuthService()

	t.Run("ValidateToken with valid token", func(t *testing.T) {
		ctx, err := authService.ValidateToken("valid-token")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if ctx.UserID != "user-123" {
			t.Errorf("Expected UserID user-123, got %s", ctx.UserID)
		}
	})

	t.Run("ValidateToken with invalid token", func(t *testing.T) {
		_, err := authService.ValidateToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})

	t.Run("HasRole checks roles correctly", func(t *testing.T) {
		ctx, _ := authService.ValidateToken("admin-token")

		if !authService.HasRole(ctx, "admin") {
			t.Error("Expected admin token to have admin role")
		}
		if !authService.HasRole(ctx, "user") {
			t.Error("Expected admin token to have user role")
		}
		if authService.HasRole(ctx, "superadmin") {
			t.Error("Expected admin token to not have superadmin role")
		}
	})

	t.Run("HasScope checks scopes correctly", func(t *testing.T) {
		ctx, _ := authService.ValidateToken("readonly-token")

		if !authService.HasScope(ctx, "read") {
			t.Error("Expected readonly token to have read scope")
		}
		if authService.HasScope(ctx, "write") {
			t.Error("Expected readonly token to not have write scope")
		}
	})

	t.Run("CanAccessResource with admin", func(t *testing.T) {
		ctx, _ := authService.ValidateToken("admin-token")

		canAccess, err := authService.CanAccessResource(ctx, "document", "doc-123", "delete")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !canAccess {
			t.Error("Expected admin to be able to delete documents")
		}
	})

	t.Run("CanAccessResource with regular user", func(t *testing.T) {
		ctx, _ := authService.ValidateToken("valid-token")

		canAccess, err := authService.CanAccessResource(ctx, "document", "doc-123", "delete")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if canAccess {
			t.Error("Expected regular user to not be able to delete documents")
		}
	})

	t.Run("GetUserPermissions returns correct permissions", func(t *testing.T) {
		ctx, _ := authService.ValidateToken("valid-token")

		perms, err := authService.GetUserPermissions(ctx, "document", "doc-123")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(perms.Actions) != 2 {
			t.Errorf("Expected 2 actions for valid-token, got %d", len(perms.Actions))
		}
		if perms.ResourceType != "document" {
			t.Errorf("Expected ResourceType document, got %s", perms.ResourceType)
		}
	})
}
