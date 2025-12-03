package fiberoapi

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// JSON Type Mismatch Error Handling Tests
//
// These tests verify that fiber-oapi correctly handles JSON type mismatch errors
// and transforms them into user-friendly validation error messages.
//
// Problem: When a client sends invalid JSON types (e.g., number instead of string),
// the raw Go error message is not user-friendly:
//   "json: cannot unmarshal number into Go struct field Request.Description of type string"
//
// Solution: Parse the error message to extract field name and type information,
// then transform it into a readable message:
//   "invalid type for field 'description': expected string but got number"
//
// Implementation: The error type from c.BodyParser() is *errors.UnmarshalTypeError
// (not *json.UnmarshalTypeError), so we parse the error message string to extract
// the field name, expected type, and actual type. This approach works reliably
// across different Fiber versions and handles all JSON unmarshal type errors.

// Test for JSON type mismatch errors
func TestJSONTypeMismatchErrors(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	type CreateRequest struct {
		Description string            `json:"description,omitempty" validate:"omitempty,max=255"`
		Ips         []string          `json:"ips,omitempty" validate:"dive,cidrv4|ip4_addr"`
		Count       int               `json:"count,omitempty"`
		Price       float64           `json:"price,omitempty"`
		Active      bool              `json:"active,omitempty"`
		Metadata    map[string]string `json:"metadata,omitempty"`
	}

	type CreateResponse struct {
		Message string `json:"message"`
	}

	Post(oapi, "/test", func(c *fiber.Ctx, input CreateRequest) (CreateResponse, TestError) {
		return CreateResponse{Message: "created"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "create",
		Summary:     "Create a new entry",
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
			name:           "Invalid request - description is a bool",
			body:           `{"description": true}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'description'",
		},
		{
			name:           "Invalid request - ips contains number",
			body:           `{"ips": [123]}`,
			expectedStatus: 400,
			errorContains:  "invalid type",
		},
		{
			name:           "Invalid request - ips is an int",
			body:           `{"ips": 123}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'ips'",
		},
		{
			name:           "Invalid request - ips is a float",
			body:           `{"ips": 123.45}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'ips'",
		},
		{
			name:           "Invalid request - ips is a string instead of array",
			body:           `{"ips": "0.0.0.0"}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'ips'",
		},
		{
			name:           "Invalid request - ips is a bool instead of array",
			body:           `{"ips": true}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'ips'",
		},
		{
			name:           "Invalid request - ips is an object instead of array",
			body:           `{"ips": {"key": "value"}}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'ips'",
		},
		{
			name:           "Valid request with empty body",
			body:           `{}`,
			expectedStatus: 200,
		},
		{
			name:           "Valid request with valid IPs",
			body:           `{"description": "Test", "ips": ["192.168.1.0/24", "10.0.0.1"]}`,
			expectedStatus: 200,
		},
		// Tests for int field (count)
		{
			name:           "Invalid request - count is a string",
			body:           `{"count": "123"}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'count'",
		},
		{
			name:           "Invalid request - count is a bool",
			body:           `{"count": true}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'count'",
		},
		{
			name:           "Invalid request - count is an object",
			body:           `{"count": {"value": 123}}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'count'",
		},
		{
			name:           "Invalid request - count is an array",
			body:           `{"count": [123]}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'count'",
		},
		{
			name:           "Valid request - count is a number",
			body:           `{"count": 123}`,
			expectedStatus: 200,
		},
		// Tests for float field (price)
		{
			name:           "Invalid request - price is a string",
			body:           `{"price": "99.99"}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'price'",
		},
		{
			name:           "Invalid request - price is a bool",
			body:           `{"price": false}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'price'",
		},
		{
			name:           "Invalid request - price is an object",
			body:           `{"price": {"amount": 99.99}}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'price'",
		},
		{
			name:           "Invalid request - price is an array",
			body:           `{"price": [99.99]}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'price'",
		},
		{
			name:           "Valid request - price is a number",
			body:           `{"price": 99.99}`,
			expectedStatus: 200,
		},
		// Tests for bool field (active)
		{
			name:           "Invalid request - active is a string",
			body:           `{"active": "true"}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'active'",
		},
		{
			name:           "Invalid request - active is a number",
			body:           `{"active": 1}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'active'",
		},
		{
			name:           "Invalid request - active is an object",
			body:           `{"active": {"enabled": true}}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'active'",
		},
		{
			name:           "Invalid request - active is an array",
			body:           `{"active": [true]}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'active'",
		},
		{
			name:           "Valid request - active is a bool",
			body:           `{"active": true}`,
			expectedStatus: 200,
		},
		// Tests for map field (metadata)
		{
			name:           "Invalid request - metadata is a string",
			body:           `{"metadata": "some data"}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'metadata'",
		},
		{
			name:           "Invalid request - metadata is a number",
			body:           `{"metadata": 123}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'metadata'",
		},
		{
			name:           "Invalid request - metadata is a bool",
			body:           `{"metadata": true}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'metadata'",
		},
		{
			name:           "Invalid request - metadata is an array",
			body:           `{"metadata": ["key1", "value1"]}`,
			expectedStatus: 400,
			errorContains:  "invalid type for field 'metadata'",
		},
		{
			name:           "Valid request - metadata is an object",
			body:           `{"metadata": {"key1": "value1", "key2": "value2"}}`,
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, bodyStr)
			}

			if tt.errorContains != "" {
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

// Test all type combinations: bool, map, int, etc.
func TestAllTypeMismatches(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	type ComplexRequest struct {
		Name      string            `json:"name"`
		Age       int               `json:"age"`
		Active    bool              `json:"active"`
		Tags      []string          `json:"tags"`
		Metadata  map[string]string `json:"metadata"`
		Score     float64           `json:"score"`
	}

	type TestResponse struct {
		Message string `json:"message"`
	}

	Post(oapi, "/test", func(c *fiber.Ctx, input ComplexRequest) (TestResponse, TestError) {
		return TestResponse{Message: "OK"}, TestError{}
	}, OpenAPIOptions{})

	tests := []struct {
		name          string
		body          string
		expectedField string
		expectedType  string
		actualType    string
	}{
		{
			name:          "String field receives number",
			body:          `{"name": 123}`,
			expectedField: "name",
			expectedType:  "string",
			actualType:    "number",
		},
		{
			name:          "String field receives bool",
			body:          `{"name": true}`,
			expectedField: "name",
			expectedType:  "string",
			actualType:    "bool",
		},
		{
			name:          "Int field receives string",
			body:          `{"age": "25"}`,
			expectedField: "age",
			expectedType:  "int",
			actualType:    "string",
		},
		{
			name:          "Bool field receives string",
			body:          `{"active": "true"}`,
			expectedField: "active",
			expectedType:  "bool",
			actualType:    "string",
		},
		{
			name:          "Bool field receives number",
			body:          `{"active": 1}`,
			expectedField: "active",
			expectedType:  "bool",
			actualType:    "number",
		},
		{
			name:          "Array field receives string",
			body:          `{"tags": "tag1"}`,
			expectedField: "tags",
			expectedType:  "[]string",
			actualType:    "string",
		},
		{
			name:          "Array field receives object",
			body:          `{"tags": {"key": "value"}}`,
			expectedField: "tags",
			expectedType:  "[]string",
			actualType:    "object",
		},
		{
			name:          "Map field receives string",
			body:          `{"metadata": "data"}`,
			expectedField: "metadata",
			expectedType:  "map[string]string",
			actualType:    "string",
		},
		{
			name:          "Map field receives array",
			body:          `{"metadata": ["a", "b"]}`,
			expectedField: "metadata",
			expectedType:  "map[string]string",
			actualType:    "array",
		},
		{
			name:          "Float field receives string",
			body:          `{"score": "99.5"}`,
			expectedField: "score",
			expectedType:  "float64",
			actualType:    "string",
		},
		{
			name:          "Float field receives bool",
			body:          `{"score": true}`,
			expectedField: "score",
			expectedType:  "float64",
			actualType:    "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("Test error: %v", err)
			}

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			if resp.StatusCode != 400 {
				t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, bodyStr)
			}

			// Check for clean error message
			if !strings.Contains(bodyStr, "invalid type for field") {
				t.Errorf("Expected 'invalid type for field' in error, got: %s", bodyStr)
			}

			// Check field name is present
			if !strings.Contains(bodyStr, tt.expectedField) {
				t.Errorf("Expected field name '%s' in error, got: %s", tt.expectedField, bodyStr)
			}

			// Log the error for debugging
			t.Logf("Error message: %s", bodyStr)
		})
	}
}
