package fiberoapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Mock AuthorizationService for testing
type MockAuthService struct {
	tokens map[string]*AuthContext
	fail   bool
}

func NewMockAuthService() *MockAuthService {
	return &MockAuthService{
		tokens: map[string]*AuthContext{
			"valid-token": {
				UserID: "user-123",
				Roles:  []string{"user"},
				Scopes: []string{"read", "write"},
				Claims: map[string]interface{}{
					"sub": "user-123",
					"exp": time.Now().Add(time.Hour).Unix(),
				},
			},
			"admin-token": {
				UserID: "admin-456",
				Roles:  []string{"admin", "user"},
				Scopes: []string{"read", "write", "delete", "share"},
				Claims: map[string]interface{}{
					"sub": "admin-456",
					"exp": time.Now().Add(time.Hour).Unix(),
				},
			},
			"readonly-token": {
				UserID: "readonly-789",
				Roles:  []string{"user"},
				Scopes: []string{"read"},
				Claims: map[string]interface{}{
					"sub": "readonly-789",
					"exp": time.Now().Add(time.Hour).Unix(),
				},
			},
		},
	}
}

func (m *MockAuthService) ValidateToken(token string) (*AuthContext, error) {
	if m.fail {
		return nil, fmt.Errorf("mock auth service failure")
	}
	
	auth, exists := m.tokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid token: %s", token)
	}
	return auth, nil
}

func (m *MockAuthService) HasRole(ctx *AuthContext, role string) bool {
	for _, r := range ctx.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (m *MockAuthService) HasScope(ctx *AuthContext, scope string) bool {
	for _, s := range ctx.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (m *MockAuthService) CanAccessResource(ctx *AuthContext, resourceType, resourceID, action string) (bool, error) {
	// Admins can access everything
	if m.HasRole(ctx, "admin") {
		return true, nil
	}
	
	// Simple logic for testing
	switch action {
	case "read":
		return m.HasScope(ctx, "read"), nil
	case "write":
		return m.HasScope(ctx, "write"), nil
	case "delete":
		return m.HasScope(ctx, "delete"), nil
	case "share":
		return m.HasScope(ctx, "share"), nil
	default:
		return false, nil
	}
}

func (m *MockAuthService) GetUserPermissions(ctx *AuthContext, resourceType, resourceID string) (*ResourcePermission, error) {
	actions := []string{}
	if m.HasScope(ctx, "read") {
		actions = append(actions, "read")
	}
	if m.HasScope(ctx, "write") {
		actions = append(actions, "write")
	}
	if m.HasScope(ctx, "delete") {
		actions = append(actions, "delete")
	}
	if m.HasScope(ctx, "share") {
		actions = append(actions, "share")
	}
	
	return &ResourcePermission{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Actions:      actions,
	}, nil
}

// Test structures
type TestRequest struct {
	ID string `path:"id" validate:"required"`
}

type TestRequestWithAuth struct {
	ID         string `path:"id" validate:"required"`
	ResourceID string `resource:"document" action:"read"`
}

type TestBody struct {
	Name string `json:"name" validate:"required"`
}

type TestResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func TestAuthenticationMiddleware(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	
	config := Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
		SecuritySchemes: map[string]SecurityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "JWT Bearer token",
			},
		},
		DefaultSecurity: []map[string][]string{
			{"bearerAuth": {}},
		},
	}
	
	oapi := New(app, config)

	t.Run("Public endpoint without auth", func(t *testing.T) {
		Get(oapi, "/public", 
			func(c *fiber.Ctx, input struct{}) (TestResponse, *ErrorResponse) {
				return TestResponse{
					ID:      "public",
					Message: "This is public",
				}, nil
			},
			OpenAPIOptions{
				Summary:  "Public endpoint",
				Security: "disabled",
			})

		req := httptest.NewRequest("GET", "/public", nil)
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Protected endpoint without token", func(t *testing.T) {
		Get(oapi, "/protected", 
			func(c *fiber.Ctx, input struct{}) (TestResponse, *ErrorResponse) {
				return TestResponse{
					ID:      "protected",
					Message: "This is protected",
				}, nil
			},
			OpenAPIOptions{
				Summary: "Protected endpoint",
			})

		req := httptest.NewRequest("GET", "/protected", nil)
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("Protected endpoint with valid token", func(t *testing.T) {
		Get(oapi, "/user-info", 
			func(c *fiber.Ctx, input struct{}) (TestResponse, *ErrorResponse) {
				authCtx, _ := GetAuthContext(c)
				return TestResponse{
					ID:      authCtx.UserID,
					Message: fmt.Sprintf("Hello user with roles: %v", authCtx.Roles),
				}, nil
			},
			OpenAPIOptions{
				Summary: "Get user info",
			})

		req := httptest.NewRequest("GET", "/user-info", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var response TestResponse
		json.NewDecoder(resp.Body).Decode(&response)
		
		if response.ID != "user-123" {
			t.Errorf("Expected user ID user-123, got %s", response.ID)
		}
	})

	t.Run("Protected endpoint with invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/user-info", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("Protected endpoint with malformed auth header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/user-info", nil)
		req.Header.Set("Authorization", "Invalid format")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestRoleBasedAccess(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	
	config := Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
	}
	
	oapi := New(app, config)

	// Endpoint requiring admin role
	Delete(oapi, "/admin/:id", 
		func(c *fiber.Ctx, input TestRequest) (TestResponse, *ErrorResponse) {
			authCtx, _ := GetAuthContext(c)
			
			// Check admin role manually in handler
			if !authService.HasRole(authCtx, "admin") {
				return TestResponse{}, &ErrorResponse{
					Code:    403,
					Details: "Admin role required",
					Type:    "authorization_error",
				}
			}
			
			return TestResponse{
				ID:      input.ID,
				Message: "Admin action completed",
			}, nil
		},
		OpenAPIOptions{
			Summary: "Admin only endpoint",
		})

	t.Run("Admin endpoint with admin token", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/123", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Admin endpoint with user token", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/123", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 403 {
			t.Errorf("Expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestScopeBasedAccess(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	
	config := Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
	}
	
	oapi := New(app, config)

	// Endpoint requiring write scope
	Put(oapi, "/documents/:id", 
		func(c *fiber.Ctx, input struct {
			ID   string   `path:"id" validate:"required"`
			Name string   `json:"name" validate:"required"`
		}) (TestResponse, *ErrorResponse) {
			authCtx, _ := GetAuthContext(c)
			
			// Check write scope manually in handler
			if !authService.HasScope(authCtx, "write") {
				return TestResponse{}, &ErrorResponse{
					Code:    403,
					Details: "Write scope required",
					Type:    "authorization_error",
				}
			}
			
			return TestResponse{
				ID:      input.ID,
				Message: fmt.Sprintf("Document updated: %s", input.Name),
			}, nil
		},
		OpenAPIOptions{
			Summary: "Update document",
		})

	t.Run("Write endpoint with write scope", func(t *testing.T) {
		body := `{"name": "Test Document"}`
		
		req := httptest.NewRequest("PUT", "/documents/123", bytes.NewReader([]byte(body)))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Write endpoint with readonly token", func(t *testing.T) {
		body := `{"name": "Test Document"}`
		
		req := httptest.NewRequest("PUT", "/documents/123", bytes.NewReader([]byte(body)))
		req.Header.Set("Authorization", "Bearer readonly-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 403 {
			t.Errorf("Expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestPOSTWithoutBody(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	
	config := Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
	}
	
	oapi := New(app, config)

	// POST endpoint without body (like share)
	Post(oapi, "/documents/:id/share", 
		func(c *fiber.Ctx, input TestRequest) (TestResponse, *ErrorResponse) {
			authCtx, _ := GetAuthContext(c)
			
			return TestResponse{
				ID:      input.ID,
				Message: fmt.Sprintf("Document shared by %s", authCtx.UserID),
			}, nil
		},
		OpenAPIOptions{
			Summary: "Share document",
		})

	t.Run("POST without body works", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/documents/123/share", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var response TestResponse
		json.NewDecoder(resp.Body).Decode(&response)
		
		if response.ID != "123" {
			t.Errorf("Expected document ID 123, got %s", response.ID)
		}
		
		if response.Message != "Document shared by user-123" {
			t.Errorf("Expected correct message, got %s", response.Message)
		}
	})
}

func TestGetAuthContext(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	
	config := Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
	}
	
	oapi := New(app, config)

	Get(oapi, "/context-test", 
		func(c *fiber.Ctx, input struct{}) (map[string]interface{}, *ErrorResponse) {
			authCtx, err := GetAuthContext(c)
			if err != nil {
				return nil, &ErrorResponse{
					Code:    500,
					Details: err.Error(),
					Type:    "context_error",
				}
			}
			
			return map[string]interface{}{
				"user_id": authCtx.UserID,
				"roles":   authCtx.Roles,
				"scopes":  authCtx.Scopes,
			}, nil
		},
		OpenAPIOptions{
			Summary: "Test auth context",
		})

	t.Run("GetAuthContext returns correct data", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/context-test", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		
		if response["user_id"] != "admin-456" {
			t.Errorf("Expected user_id admin-456, got %v", response["user_id"])
		}
		
		roles, ok := response["roles"].([]interface{})
		if !ok || len(roles) != 2 {
			t.Errorf("Expected 2 roles, got %v", response["roles"])
		}
	})
}

func TestAuthServiceFailure(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	authService.fail = true // Make auth service fail
	
	config := Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
	}
	
	oapi := New(app, config)

	Get(oapi, "/fail-test", 
		func(c *fiber.Ctx, input struct{}) (TestResponse, *ErrorResponse) {
			return TestResponse{
				ID:      "test",
				Message: "Should not reach here",
			}, nil
		},
		OpenAPIOptions{
			Summary: "Test auth failure",
		})

	t.Run("Auth service failure returns error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/fail-test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}