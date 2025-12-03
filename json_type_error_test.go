package fiberoapi

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Test for JSON type mismatch errors
func TestJSONTypeMismatchErrors(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	type CreateWorkspaceRequest struct {
		Description  string   `json:"description,omitempty" validate:"omitempty,max=255"`
		IpsWhitelist []string `json:"ips_whitelist,omitempty" validate:"dive,cidrv4|ip4_addr"`
	}

	type CreateWorkspaceResponse struct {
		Message string `json:"message"`
	}

	Post(oapi, "/workspaces", func(c *fiber.Ctx, input CreateWorkspaceRequest) (CreateWorkspaceResponse, TestError) {
		return CreateWorkspaceResponse{Message: "Workspace created"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "create-workspace",
		Summary:     "Create a new workspace",
	})

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		errorContains  string
	}{
		{
			name:           "Valid request with string description",
			body:           `{"description": "A valid description"}`,
			expectedStatus: 200,
		},
		{
			name:           "Invalid request - description is a number",
			body:           `{"description": 0.0}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'description'",
		},
		{
			name:           "Invalid request - description is an object",
			body:           `{"description": {"test": "test"}}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'description'",
		},
		{
			name:           "Invalid request - ips_whitelist contains number",
			body:           `{"ips_whitelist": [123]}`,
			expectedStatus: 400,
			errorContains:  "invalid type",
		},
		{
			name:           "Valid request with empty body",
			body:           `{}`,
			expectedStatus: 200,
		},
		{
			name:           "Valid request with valid IPs",
			body:           `{"description": "Test", "ips_whitelist": ["192.168.1.0/24", "10.0.0.1"]}`,
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/workspaces", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, string(body))
			}

			if tt.errorContains != "" {
				body, _ := io.ReadAll(resp.Body)
				bodyStr := string(body)
				if !strings.Contains(bodyStr, tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got %s", tt.errorContains, bodyStr)
				}
				// Ensure it returns validation_error type
				if !strings.Contains(bodyStr, "validation_error") {
					t.Errorf("Expected validation_error type, got %s", bodyStr)
				}
			}
		})
	}
}

// Test with custom validation error handler
func TestJSONTypeMismatchWithCustomHandler(t *testing.T) {
	app := fiber.New()

	// Create a custom validation error handler
	customHandler := func(c *fiber.Ctx, err error) error {
		return c.Status(422).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}

	oapi := New(app, Config{
		ValidationErrorHandler: customHandler,
	})

	type TestRequest struct {
		Value string `json:"value"`
	}

	type TestResponse struct {
		Result string `json:"result"`
	}

	Post(oapi, "/test", func(c *fiber.Ctx, input TestRequest) (TestResponse, TestError) {
		return TestResponse{Result: "OK"}, TestError{}
	}, OpenAPIOptions{})

	// Test with wrong type
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"value": 123}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should use custom handler status code
	if resp.StatusCode != 422 {
		t.Errorf("Expected status 422, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Should contain custom error format
	if !strings.Contains(bodyStr, "status") || !strings.Contains(bodyStr, "error") {
		t.Errorf("Expected custom error format, got %s", bodyStr)
	}

	// Should still contain the error message about invalid type
	if !strings.Contains(bodyStr, "invalid type") {
		t.Errorf("Expected 'invalid type' in error message, got %s", bodyStr)
	}
}
