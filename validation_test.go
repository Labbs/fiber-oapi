package fiberoapi

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Advanced tests for validation with validator

type UserCreateInput struct {
	Username string `path:"username" validate:"required,min=3,max=20,alphanum"`
	Email    string `query:"email" validate:"required,email"`
	Age      int    `query:"age" validate:"required,min=13,max=120"`
	Website  string `query:"website" validate:"omitempty,url"`
	Role     string `query:"role" validate:"required,oneof=admin user moderator"`
}

type ProductInput struct {
	CategoryID string  `path:"categoryId" validate:"required,uuid4"`
	ProductID  string  `path:"productId" validate:"required,numeric"`
	MinPrice   float64 `query:"minPrice" validate:"omitempty,min=0"`
	MaxPrice   float64 `query:"maxPrice" validate:"omitempty,min=0,gtfield=MinPrice"`
	InStock    bool    `query:"inStock"`
}

func TestAdvancedValidation_UserCreate(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	GetOApi(oapi, "/users/:username", func(c *fiber.Ctx, input UserCreateInput) (TestOutput, TestError) {
		return TestOutput{
			Message: fmt.Sprintf("Valid user: %s, %s, age %d, role %s",
				input.Username, input.Email, input.Age, input.Role),
		}, TestError{}
	}, OpenAPIOptions{
		OperationID: "create-user-advanced",
		Summary:     "Create user with advanced validation",
	})

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		shouldPass     bool
		errorContains  string
	}{
		{
			name:           "Valid input",
			url:            "/users/john123?email=john@example.com&age=25&role=user&website=https://example.com",
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Username too short",
			url:            "/users/jo?email=john@example.com&age=25&role=user",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Username with special chars",
			url:            "/users/john@123?email=john@example.com&age=25&role=user",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "alphanum",
		},
		{
			name:           "Invalid email",
			url:            "/users/john123?email=not-an-email&age=25&role=user",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "email",
		},
		{
			name:           "Age too young",
			url:            "/users/john123?email=john@example.com&age=12&role=user",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Invalid role",
			url:            "/users/john123?email=john@example.com&age=25&role=superadmin",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "oneof",
		},
		{
			name:           "Invalid website URL",
			url:            "/users/john123?email=john@example.com&age=25&role=user&website=not-a-url",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			if tt.shouldPass {
				if !strings.Contains(bodyStr, "Valid user") {
					t.Errorf("Expected success message, got %s", bodyStr)
				}
			} else {
				if !strings.Contains(bodyStr, "Validation failed") {
					t.Errorf("Expected validation error, got %s", bodyStr)
				}
				if tt.errorContains != "" && !strings.Contains(bodyStr, tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got %s", tt.errorContains, bodyStr)
				}
			}
		})
	}
}

func TestAdvancedValidation_Product(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	GetOApi(oapi, "/categories/:categoryId/products/:productId",
		func(c *fiber.Ctx, input ProductInput) (TestOutput, TestError) {
			return TestOutput{
				Message: fmt.Sprintf("Product %s in category %s, price range: %.2f-%.2f, in stock: %t",
					input.ProductID, input.CategoryID, input.MinPrice, input.MaxPrice, input.InStock),
			}, TestError{}
		}, OpenAPIOptions{
			OperationID: "get-product-advanced",
			Summary:     "Get product with advanced validation",
		})

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		shouldPass     bool
		errorContains  string
	}{
		{
			name:           "Valid input with UUID and numeric",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products/12345?minPrice=10.50&maxPrice=99.99&inStock=true",
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Invalid UUID for categoryId",
			url:            "/categories/not-a-uuid/products/12345?minPrice=10.50&maxPrice=99.99",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "uuid4",
		},
		{
			name:           "Non-numeric productId",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products/abc123?minPrice=10.50&maxPrice=99.99",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "numeric",
		},
		{
			name:           "Negative price",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products/12345?minPrice=-5.00&maxPrice=99.99",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "MaxPrice less than MinPrice",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products/12345?minPrice=50.00&maxPrice=10.00",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "gtfield",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			if tt.shouldPass {
				if !strings.Contains(bodyStr, "Product") {
					t.Errorf("Expected success message, got %s", bodyStr)
				}
			} else {
				if !strings.Contains(bodyStr, "Validation failed") {
					t.Errorf("Expected validation error, got %s", bodyStr)
				}
				if tt.errorContains != "" && !strings.Contains(bodyStr, tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got %s", tt.errorContains, bodyStr)
				}
			}
		})
	}
}

func TestValidation_CustomMessages(t *testing.T) {
	// This test shows how you could customize error messages
	// by creating a custom validator with translations

	app := fiber.New()
	oapi := New(app)

	type SimpleInput struct {
		Name string `path:"name" validate:"required,min=3"`
	}

	GetOApi(oapi, "/simple/:name", func(c *fiber.Ctx, input SimpleInput) (TestOutput, TestError) {
		return TestOutput{Message: "Valid"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "simple-validation",
	})

	// Test with name too short
	req := httptest.NewRequest("GET", "/simple/ab", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check that we receive the validation error
	if !strings.Contains(bodyStr, "Validation failed") {
		t.Errorf("Expected validation error, got %s", bodyStr)
	}

	// The error message contains validator details
	if !strings.Contains(bodyStr, "min") || !strings.Contains(bodyStr, "Name") {
		t.Errorf("Expected detailed validation error with field name and rule, got %s", bodyStr)
	}
}
