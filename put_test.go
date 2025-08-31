package fiberoapi

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Test structs pour PUT
type PutUserInput struct {
	ID       string `path:"id" validate:"required"`
	Username string `json:"username" validate:"omitempty,min=3,max=20,alphanum"`
	Email    string `json:"email" validate:"omitempty,email"`
	Age      int    `json:"age" validate:"omitempty,min=13,max=120"`
	Bio      string `json:"bio" validate:"omitempty,max=500"`
}

type PutProductInput struct {
	CategoryID  string   `path:"categoryId" validate:"required,uuid4"`
	ProductID   string   `path:"productId" validate:"required,uuid4"`
	Name        string   `json:"name" validate:"omitempty,min=2,max=100"`
	Description string   `json:"description" validate:"omitempty,max=1000"`
	Price       float64  `json:"price" validate:"omitempty,min=0"`
	Quantity    int      `json:"quantity" validate:"omitempty,min=0"`
	Tags        []string `json:"tags" validate:"omitempty,dive,min=1,max=20"`
}

type PutOutput struct {
	ID      string      `json:"id"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Updated bool        `json:"updated"`
}

type PutError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

func TestPutOApi_SimplePut(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with simple PUT (path param + JSON body)
	Put(oapi, "/users/:id", func(c *fiber.Ctx, input PutUserInput) (PutOutput, PutError) {
		return PutOutput{
			ID:      input.ID,
			Message: fmt.Sprintf("User %s updated successfully", input.ID),
			Data: map[string]interface{}{
				"username": input.Username,
				"email":    input.Email,
				"age":      input.Age,
				"bio":      input.Bio,
			},
			Updated: true,
		}, PutError{}
	}, OpenAPIOptions{
		OperationID: "update-user",
		Summary:     "Update an existing user",
		Tags:        []string{"users"},
	})

	// Test with valid data
	body := `{"username":"john123","email":"john@example.com","age":25,"bio":"Updated bio"}`
	req := httptest.NewRequest("PUT", "/users/user123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)
	if !strings.Contains(respStr, `"id":"user123"`) {
		t.Errorf("Expected response to contain user ID, got %s", respStr)
	}
	if !strings.Contains(respStr, `"updated":true`) {
		t.Errorf("Expected response to contain updated flag, got %s", respStr)
	}
	if !strings.Contains(respStr, `"username":"john123"`) {
		t.Errorf("Expected response to contain username, got %s", respStr)
	}
}

func TestPutOApi_MultiplePathParams(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with PUT + multiple path parameters
	Put(oapi, "/categories/:categoryId/products/:productId", func(c *fiber.Ctx, input PutProductInput) (PutOutput, PutError) {
		return PutOutput{
			ID:      input.ProductID,
			Message: fmt.Sprintf("Product %s updated in category %s", input.ProductID, input.CategoryID),
			Data: map[string]interface{}{
				"categoryId": input.CategoryID,
				"productId":  input.ProductID,
				"name":       input.Name,
				"price":      input.Price,
				"quantity":   input.Quantity,
				"tags":       input.Tags,
			},
			Updated: true,
		}, PutError{}
	}, OpenAPIOptions{
		OperationID: "update-product",
		Summary:     "Update a product",
		Tags:        []string{"products"},
	})

	// Test with valid data
	body := `{"name":"Updated Laptop","description":"Gaming laptop updated","price":1199.99,"quantity":15,"tags":["gaming","electronics","updated"]}`
	req := httptest.NewRequest("PUT", "/categories/550e8400-e29b-41d4-a716-446655440000/products/550e8400-e29b-41d4-a716-446655440001", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)
	if !strings.Contains(respStr, `"categoryId":"550e8400-e29b-41d4-a716-446655440000"`) {
		t.Errorf("Expected response to contain category ID, got %s", respStr)
	}
	if !strings.Contains(respStr, `"productId":"550e8400-e29b-41d4-a716-446655440001"`) {
		t.Errorf("Expected response to contain product ID, got %s", respStr)
	}
	if !strings.Contains(respStr, `"name":"Updated Laptop"`) {
		t.Errorf("Expected response to contain updated name, got %s", respStr)
	}
}

func TestPutOApi_Validation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Put(oapi, "/users/:id", func(c *fiber.Ctx, input PutUserInput) (PutOutput, PutError) {
		return PutOutput{
			ID:      input.ID,
			Message: "User updated",
			Updated: true,
		}, PutError{}
	}, OpenAPIOptions{
		OperationID: "update-user-validation",
		Summary:     "Update user with validation",
	})

	tests := []struct {
		name           string
		url            string
		body           string
		expectedStatus int
		shouldPass     bool
		errorContains  string
	}{
		{
			name:           "Valid partial update",
			url:            "/users/user123",
			body:           `{"username":"alice123","email":"alice@example.com"}`,
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Valid single field update",
			url:            "/users/user456",
			body:           `{"age":30}`,
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Valid empty body (all fields optional)",
			url:            "/users/user789",
			body:           `{}`,
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Username too short",
			url:            "/users/user123",
			body:           `{"username":"al"}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Invalid email format",
			url:            "/users/user123",
			body:           `{"email":"not-an-email"}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "email",
		},
		{
			name:           "Age too young",
			url:            "/users/user123",
			body:           `{"age":10}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Bio too long",
			url:            "/users/user123",
			body:           fmt.Sprintf(`{"bio":"%s"}`, strings.Repeat("a", 501)),
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "max",
		},
		{
			name:           "Invalid JSON",
			url:            "/users/user123",
			body:           `{"username":"alice123","email":"alice@example.com",}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
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
				if !strings.Contains(bodyStr, "User updated") {
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

func TestPutOApi_ErrorHandling(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with custom error handling
	Put(oapi, "/users/:id", func(c *fiber.Ctx, input PutUserInput) (PutOutput, PutError) {
		if input.ID == "notfound" {
			return PutOutput{}, PutError{
				StatusCode: 404,
				Message:    "User not found",
				Details:    "The specified user does not exist",
			}
		}
		if input.ID == "readonly" {
			return PutOutput{}, PutError{
				StatusCode: 403,
				Message:    "User is read-only",
				Details:    "This user cannot be modified",
			}
		}
		if input.Username == "taken" {
			return PutOutput{}, PutError{
				StatusCode: 409,
				Message:    "Username already taken",
				Details:    "Please choose a different username",
			}
		}
		return PutOutput{
			ID:      input.ID,
			Message: "User updated successfully",
			Updated: true,
		}, PutError{}
	}, OpenAPIOptions{
		OperationID: "update-user-with-errors",
		Summary:     "Update user with custom error handling",
	})

	// Test 1: User not found
	body1 := `{"username":"newname","email":"test@example.com"}`
	req1 := httptest.NewRequest("PUT", "/users/notfound", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp1.StatusCode != 404 {
		t.Errorf("Expected status 404, got %d", resp1.StatusCode)
	}

	body1Resp, _ := io.ReadAll(resp1.Body)
	if !strings.Contains(string(body1Resp), "User not found") {
		t.Errorf("Expected not found error message, got %s", string(body1Resp))
	}

	// Test 2: Read-only user
	body2 := `{"username":"newname","email":"test@example.com"}`
	req2 := httptest.NewRequest("PUT", "/users/readonly", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.StatusCode != 403 {
		t.Errorf("Expected status 403, got %d", resp2.StatusCode)
	}

	body2Resp, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2Resp), "User is read-only") {
		t.Errorf("Expected read-only error message, got %s", string(body2Resp))
	}

	// Test 3: Username taken
	body3 := `{"username":"taken"}`
	req3 := httptest.NewRequest("PUT", "/users/user123", strings.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp3.StatusCode != 409 {
		t.Errorf("Expected status 409, got %d", resp3.StatusCode)
	}

	body3Resp, _ := io.ReadAll(resp3.Body)
	if !strings.Contains(string(body3Resp), "Username already taken") {
		t.Errorf("Expected username taken error message, got %s", string(body3Resp))
	}

	// Test 4: Success case
	body4 := `{"username":"validname","email":"valid@example.com"}`
	req4 := httptest.NewRequest("PUT", "/users/user123", strings.NewReader(body4))
	req4.Header.Set("Content-Type", "application/json")
	resp4, err := app.Test(req4)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp4.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp4.StatusCode)
	}

	body4Resp, _ := io.ReadAll(resp4.Body)
	if !strings.Contains(string(body4Resp), "User updated successfully") {
		t.Errorf("Expected success message, got %s", string(body4Resp))
	}
	if !strings.Contains(string(body4Resp), `"updated":true`) {
		t.Errorf("Expected updated flag, got %s", string(body4Resp))
	}
}

func TestPutOApi_OperationStorage(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Check that PUT operations are properly stored
	initialCount := len(oapi.operations)

	Put(oapi, "/test/:id", func(c *fiber.Ctx, input PutUserInput) (PutOutput, PutError) {
		return PutOutput{Message: "test", Updated: true}, PutError{}
	}, OpenAPIOptions{
		OperationID: "test-put-operation",
		Summary:     "Test PUT operation",
		Tags:        []string{"test", "put"},
		Description: "This is a test PUT operation",
	})

	// Check that an operation was added
	if len(oapi.operations) != initialCount+1 {
		t.Errorf("Expected %d operations, got %d", initialCount+1, len(oapi.operations))
	}

	// Check the operation content
	lastOp := oapi.operations[len(oapi.operations)-1]
	if lastOp.Method != "PUT" {
		t.Errorf("Expected method PUT, got %s", lastOp.Method)
	}
	if lastOp.Path != "/test/:id" {
		t.Errorf("Expected path /test/:id, got %s", lastOp.Path)
	}
	if lastOp.Options.OperationID != "test-put-operation" {
		t.Errorf("Expected operationId test-put-operation, got %s", lastOp.Options.OperationID)
	}
	if lastOp.Options.Summary != "Test PUT operation" {
		t.Errorf("Expected summary 'Test PUT operation', got %s", lastOp.Options.Summary)
	}
	if lastOp.Options.Description != "This is a test PUT operation" {
		t.Errorf("Expected description 'This is a test PUT operation', got %s", lastOp.Options.Description)
	}

	// Check the tags
	expectedTags := []string{"test", "put"}
	for _, expectedTag := range expectedTags {
		found := false
		for _, tag := range lastOp.Options.Tags {
			if tag == expectedTag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find '%s' in tags, got %v", expectedTag, lastOp.Options.Tags)
		}
	}
}
