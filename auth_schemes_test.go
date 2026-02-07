package fiberoapi

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// --- Mock services ---

// MockBasicAuthService extends MockAuthService with Basic Auth support.
type MockBasicAuthService struct {
	MockAuthService
	users map[string]string // username -> password
}

func NewMockBasicAuthService() *MockBasicAuthService {
	return &MockBasicAuthService{
		MockAuthService: *NewMockAuthService(),
		users: map[string]string{
			"admin": "secret",
			"user":  "password",
		},
	}
}

func (m *MockBasicAuthService) ValidateBasicAuth(username, password string) (*AuthContext, error) {
	expectedPassword, exists := m.users[username]
	if !exists {
		return nil, fmt.Errorf("unknown user: %s", username)
	}
	if password != expectedPassword {
		return nil, fmt.Errorf("invalid password for user: %s", username)
	}
	return &AuthContext{
		UserID: username,
		Roles:  []string{"user"},
		Scopes: []string{"read", "write"},
	}, nil
}

// MockAPIKeyAuthService extends MockAuthService with API Key support.
type MockAPIKeyAuthService struct {
	MockAuthService
	validKeys map[string]bool
}

func NewMockAPIKeyAuthService() *MockAPIKeyAuthService {
	return &MockAPIKeyAuthService{
		MockAuthService: *NewMockAuthService(),
		validKeys: map[string]bool{
			"my-api-key-123": true,
			"test-key-456":   true,
		},
	}
}

func (m *MockAPIKeyAuthService) ValidateAPIKey(key string, location string, paramName string) (*AuthContext, error) {
	if !m.validKeys[key] {
		return nil, fmt.Errorf("invalid API key")
	}
	return &AuthContext{
		UserID: "apikey-user",
		Roles:  []string{"user"},
		Scopes: []string{"read"},
	}, nil
}

// MockAWSAuthService extends MockAuthService with AWS SigV4 support.
type MockAWSAuthService struct {
	MockAuthService
	validAccessKeys map[string]bool
}

func NewMockAWSAuthService() *MockAWSAuthService {
	return &MockAWSAuthService{
		MockAuthService: *NewMockAuthService(),
		validAccessKeys: map[string]bool{
			"AKIAIOSFODNN7EXAMPLE": true,
		},
	}
}

func (m *MockAWSAuthService) ValidateAWSSignature(params *AWSSignatureParams) (*AuthContext, error) {
	if !m.validAccessKeys[params.AccessKeyID] {
		return nil, fmt.Errorf("invalid access key: %s", params.AccessKeyID)
	}
	return &AuthContext{
		UserID: "aws-user-" + params.AccessKeyID,
		Roles:  []string{"service"},
		Scopes: []string{"read", "write"},
		Claims: map[string]interface{}{
			"region":  params.Region,
			"service": params.Service,
		},
	}, nil
}

// --- Basic Auth tests ---

func TestValidateBasicAuth_ValidCredentials(t *testing.T) {
	app := fiber.New()
	authService := NewMockBasicAuthService()
	app.Use(BasicAuthMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})

	creds := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestValidateBasicAuth_InvalidCredentials(t *testing.T) {
	app := fiber.New()
	authService := NewMockBasicAuthService()
	app.Use(BasicAuthMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	creds := base64.StdEncoding.EncodeToString([]byte("admin:wrongpassword"))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestValidateBasicAuth_MalformedBase64(t *testing.T) {
	app := fiber.New()
	authService := NewMockBasicAuthService()
	app.Use(BasicAuthMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic %%%not-base64%%%")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestValidateBasicAuth_MissingColon(t *testing.T) {
	app := fiber.New()
	authService := NewMockBasicAuthService()
	app.Use(BasicAuthMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	creds := base64.StdEncoding.EncodeToString([]byte("usernameonly"))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestValidateBasicAuth_MissingHeader(t *testing.T) {
	app := fiber.New()
	authService := NewMockBasicAuthService()
	app.Use(BasicAuthMiddleware(authService))
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
}

func TestValidateBasicAuth_ServiceDoesNotImplement(t *testing.T) {
	app := fiber.New()
	// Use plain MockAuthService which does NOT implement BasicAuthValidator
	authService := NewMockAuthService()
	app.Use(BasicAuthMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	creds := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// --- API Key tests ---

func TestValidateAPIKey_InHeader_Valid(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	scheme := SecurityScheme{Type: "apiKey", In: "header", Name: "X-API-Key"}
	app.Use(APIKeyMiddleware(authService, scheme))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "my-api-key-123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestValidateAPIKey_InQuery_Valid(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	scheme := SecurityScheme{Type: "apiKey", In: "query", Name: "api_key"}
	app.Use(APIKeyMiddleware(authService, scheme))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})

	req := httptest.NewRequest("GET", "/test?api_key=my-api-key-123", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestValidateAPIKey_InCookie_Valid(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	scheme := SecurityScheme{Type: "apiKey", In: "cookie", Name: "api_key"}
	app.Use(APIKeyMiddleware(authService, scheme))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "api_key", Value: "my-api-key-123"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestValidateAPIKey_Missing(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	scheme := SecurityScheme{Type: "apiKey", In: "header", Name: "X-API-Key"}
	app.Use(APIKeyMiddleware(authService, scheme))
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
}

func TestValidateAPIKey_Invalid(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	scheme := SecurityScheme{Type: "apiKey", In: "header", Name: "X-API-Key"}
	app.Use(APIKeyMiddleware(authService, scheme))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestValidateAPIKey_ServiceDoesNotImplement(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	scheme := SecurityScheme{Type: "apiKey", In: "header", Name: "X-API-Key"}
	app.Use(APIKeyMiddleware(authService, scheme))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "my-api-key-123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// --- AWS SigV4 tests ---

func TestValidateAWSSigV4_ValidSignature(t *testing.T) {
	app := fiber.New()
	authService := NewMockAWSAuthService()
	app.Use(AWSSignatureMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abcdef1234567890")
	req.Header.Set("Host", "example.com")
	req.Header.Set("X-Amz-Date", "20250101T000000Z")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestValidateAWSSigV4_InvalidAccessKey(t *testing.T) {
	app := fiber.New()
	authService := NewMockAWSAuthService()
	app.Use(AWSSignatureMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=INVALIDKEY/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abcdef1234567890")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestValidateAWSSigV4_MalformedHeader(t *testing.T) {
	app := fiber.New()
	authService := NewMockAWSAuthService()
	app.Use(AWSSignatureMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 garbage-data")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestValidateAWSSigV4_MissingHeader(t *testing.T) {
	app := fiber.New()
	authService := NewMockAWSAuthService()
	app.Use(AWSSignatureMiddleware(authService))
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
}

func TestValidateAWSSigV4_ServiceDoesNotImplement(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	app.Use(AWSSignatureMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20250101/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// --- parseAWSSigV4Header unit tests ---

func TestParseAWSSigV4Header(t *testing.T) {
	t.Run("Valid header", func(t *testing.T) {
		header := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date;x-amz-content-sha256, Signature=abcdef1234567890"
		params, err := parseAWSSigV4Header(header)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if params.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
			t.Errorf("Expected AccessKeyID AKIAIOSFODNN7EXAMPLE, got %s", params.AccessKeyID)
		}
		if params.Date != "20250101" {
			t.Errorf("Expected Date 20250101, got %s", params.Date)
		}
		if params.Region != "us-east-1" {
			t.Errorf("Expected Region us-east-1, got %s", params.Region)
		}
		if params.Service != "s3" {
			t.Errorf("Expected Service s3, got %s", params.Service)
		}
		if len(params.SignedHeaders) != 3 {
			t.Errorf("Expected 3 signed headers, got %d", len(params.SignedHeaders))
		}
		if params.Signature != "abcdef1234567890" {
			t.Errorf("Expected Signature abcdef1234567890, got %s", params.Signature)
		}
	})

	t.Run("Missing Credential", func(t *testing.T) {
		header := "AWS4-HMAC-SHA256 SignedHeaders=host, Signature=abc123"
		_, err := parseAWSSigV4Header(header)
		if err == nil {
			t.Error("Expected error for missing Credential")
		}
	})

	t.Run("Missing Signature", func(t *testing.T) {
		header := "AWS4-HMAC-SHA256 Credential=AKID/20250101/us-east-1/s3/aws4_request, SignedHeaders=host"
		_, err := parseAWSSigV4Header(header)
		if err == nil {
			t.Error("Expected error for missing Signature")
		}
	})

	t.Run("Missing SignedHeaders", func(t *testing.T) {
		header := "AWS4-HMAC-SHA256 Credential=AKID/20250101/us-east-1/s3/aws4_request, Signature=abc123"
		_, err := parseAWSSigV4Header(header)
		if err == nil {
			t.Error("Expected error for missing SignedHeaders")
		}
	})
}

// --- Multi-scheme dispatch tests ---

func TestMultiScheme_BearerStillWorks(t *testing.T) {
	app := fiber.New()
	authService := NewMockAuthService()
	config := Config{
		SecuritySchemes: map[string]SecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		},
		DefaultSecurity: []map[string][]string{
			{"bearerAuth": {}},
		},
	}
	app.Use(MultiSchemeAuthMiddleware(authService, config))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
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
}

func TestMultiScheme_FallbackToSecondScheme(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	config := Config{
		SecuritySchemes: map[string]SecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer"},
			"apiKey":     {Type: "apiKey", In: "header", Name: "X-API-Key"},
		},
		DefaultSecurity: []map[string][]string{
			{"bearerAuth": {}},
			{"apiKey": {}},
		},
	}
	app.Use(MultiSchemeAuthMiddleware(authService, config))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})

	// Send API Key instead of Bearer token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "my-api-key-123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestMultiScheme_AllSchemesFail(t *testing.T) {
	app := fiber.New()
	authService := NewMockAPIKeyAuthService()
	config := Config{
		SecuritySchemes: map[string]SecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer"},
			"apiKey":     {Type: "apiKey", In: "header", Name: "X-API-Key"},
		},
		DefaultSecurity: []map[string][]string{
			{"bearerAuth": {}},
			{"apiKey": {}},
		},
	}
	app.Use(MultiSchemeAuthMiddleware(authService, config))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "should not reach here"})
	})

	// No auth provided at all
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// --- Backward compatibility tests ---

func TestBackwardCompat_ExistingMockAuthService(t *testing.T) {
	// Existing MockAuthService (which does NOT implement any new interfaces)
	// should continue to work with Bearer token via validateAuthorization
	app := fiber.New()
	authService := NewMockAuthService()

	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         authService,
		// No SecuritySchemes configured - should fallback to Bearer-only
	})

	Get(oapi, "/test", func(c *fiber.Ctx, input struct{}) (fiber.Map, *ErrorResponse) {
		authCtx, err := GetAuthContext(c)
		if err != nil {
			return nil, &ErrorResponse{Code: 500, Details: err.Error()}
		}
		return fiber.Map{"user_id": authCtx.UserID}, nil
	}, OpenAPIOptions{Summary: "Test"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200 for backward compat, got %d", resp.StatusCode)
	}
}

func TestBackwardCompat_BearerTokenMiddleware(t *testing.T) {
	// BearerTokenMiddleware should still work independently
	app := fiber.New()
	authService := NewMockAuthService()
	app.Use(BearerTokenMiddleware(authService))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
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
}

// --- SmartAuthMiddleware with SecuritySchemes ---

func TestSmartAuthMiddleware_WithSecuritySchemes(t *testing.T) {
	app := fiber.New()
	authService := NewMockBasicAuthService()
	config := Config{
		EnableOpenAPIDocs: true,
		OpenAPIDocsPath:   "/docs",
		OpenAPIJSONPath:   "/openapi.json",
		OpenAPIYamlPath:   "/openapi.yaml",
		SecuritySchemes: map[string]SecurityScheme{
			"basicAuth": {Type: "http", Scheme: "basic"},
		},
		DefaultSecurity: []map[string][]string{
			{"basicAuth": {}},
		},
	}
	app.Use(SmartAuthMiddleware(authService, config))
	app.Get("/test", func(c *fiber.Ctx) error {
		authCtx, _ := GetAuthContext(c)
		return c.JSON(fiber.Map{"user_id": authCtx.UserID})
	})
	app.Get("/docs", func(c *fiber.Ctx) error {
		return c.SendString("docs")
	})

	// Protected route with Basic Auth
	creds := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Docs path should be excluded from auth
	req = httptest.NewRequest("GET", "/docs", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected /docs to be accessible without auth, got %d", resp.StatusCode)
	}
}
