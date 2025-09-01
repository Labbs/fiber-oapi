package fiberoapi

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Test structs pour POST
type PostUserInput struct {
	Username string `json:"username" validate:"required,min=3,max=20,alphanum"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"required,min=13,max=120"`
	Bio      string `json:"bio" validate:"omitempty,max=500"`
}

type PostUserWithPathInput struct {
	GroupID  string `path:"groupId" validate:"required,uuid4"`
	Username string `json:"username" validate:"required,min=3,max=20"`
	Email    string `json:"email" validate:"required,email"`
	Role     string `json:"role" validate:"required,oneof=member admin"`
}

type PostProductInput struct {
	CategoryID  string   `path:"categoryId" validate:"required,uuid4"`
	Name        string   `json:"name" validate:"required,min=2,max=100"`
	Description string   `json:"description" validate:"omitempty,max=1000"`
	Price       float64  `json:"price" validate:"required,min=0"`
	Quantity    int      `json:"quantity" validate:"required,min=0"`
	Tags        []string `json:"tags" validate:"omitempty,dive,min=1,max=20"`
}

type PostOutput struct {
	ID      string      `json:"id"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PostError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

func TestPostOApi_SimplePost(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with simple POST (JSON body only)
	Post(oapi, "/users", func(c *fiber.Ctx, input PostUserInput) (PostOutput, PostError) {
		return PostOutput{
			ID:      "user_123",
			Message: fmt.Sprintf("User %s created successfully", input.Username),
			Data: map[string]interface{}{
				"username": input.Username,
				"email":    input.Email,
				"age":      input.Age,
				"bio":      input.Bio,
			},
		}, PostError{}
	}, OpenAPIOptions{
		OperationID: "create-user",
		Summary:     "Create a new user",
		Tags:        []string{"users"},
	})

	// Test with valid data
	body := `{"username":"john123","email":"john@example.com","age":25,"bio":"Hello world"}`
	req := httptest.NewRequest("POST", "/users", strings.NewReader(body))
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
	if !strings.Contains(respStr, `"id":"user_123"`) {
		t.Errorf("Expected response to contain user ID, got %s", respStr)
	}
	if !strings.Contains(respStr, `"username":"john123"`) {
		t.Errorf("Expected response to contain username, got %s", respStr)
	}
}

func TestPostOApi_WithPathParams(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with POST + path parameters
	Post(oapi, "/groups/:groupId/users", func(c *fiber.Ctx, input PostUserWithPathInput) (PostOutput, PostError) {
		return PostOutput{
			ID:      "user_456",
			Message: fmt.Sprintf("User %s added to group %s", input.Username, input.GroupID),
			Data: map[string]interface{}{
				"groupId":  input.GroupID,
				"username": input.Username,
				"email":    input.Email,
				"role":     input.Role,
			},
		}, PostError{}
	}, OpenAPIOptions{
		OperationID: "add-user-to-group",
		Summary:     "Add user to a group",
		Tags:        []string{"groups", "users"},
	})

	// Test with valid data
	body := `{"username":"jane123","email":"jane@example.com","role":"member"}`
	req := httptest.NewRequest("POST", "/groups/550e8400-e29b-41d4-a716-446655440000/users", strings.NewReader(body))
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
	if !strings.Contains(respStr, `"groupId":"550e8400-e29b-41d4-a716-446655440000"`) {
		t.Errorf("Expected response to contain group ID, got %s", respStr)
	}
	if !strings.Contains(respStr, `"username":"jane123"`) {
		t.Errorf("Expected response to contain username, got %s", respStr)
	}
}

func TestPostOApi_Validation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/users", func(c *fiber.Ctx, input PostUserInput) (PostOutput, PostError) {
		return PostOutput{
			ID:      "user_789",
			Message: "User created",
		}, PostError{}
	}, OpenAPIOptions{
		OperationID: "create-user-validation",
		Summary:     "Create user with validation",
	})

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		shouldPass     bool
		errorContains  string
	}{
		{
			name:           "Valid data",
			body:           `{"username":"alice123","email":"alice@example.com","age":28}`,
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Missing required field",
			body:           `{"email":"alice@example.com","age":28}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "required",
		},
		{
			name:           "Username too short",
			body:           `{"username":"al","email":"alice@example.com","age":28}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Invalid email",
			body:           `{"username":"alice123","email":"not-an-email","age":28}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "email",
		},
		{
			name:           "Age too young",
			body:           `{"username":"alice123","email":"alice@example.com","age":10}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Bio too long",
			body:           fmt.Sprintf(`{"username":"alice123","email":"alice@example.com","age":28,"bio":"%s"}`, strings.Repeat("a", 501)),
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "max",
		},
		{
			name:           "Invalid JSON",
			body:           `{"username":"alice123","email":"alice@example.com","age":28,}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/users", strings.NewReader(tt.body))
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
				if !strings.Contains(bodyStr, "User created") {
					t.Errorf("Expected success message, got %s", bodyStr)
				}
			} else {
				if !strings.Contains(bodyStr, "validation_error") {
					t.Errorf("Expected validation error, got %s", bodyStr)
				}
				if tt.errorContains != "" && !strings.Contains(bodyStr, tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got %s", tt.errorContains, bodyStr)
				}
			}
		})
	}
}

func TestPostOApi_ComplexValidation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with complex validation (arrays, nested validation)
	Post(oapi, "/categories/:categoryId/products",
		func(c *fiber.Ctx, input PostProductInput) (PostOutput, PostError) {
			return PostOutput{
				ID:      "product_999",
				Message: fmt.Sprintf("Product %s created in category %s", input.Name, input.CategoryID),
				Data: map[string]interface{}{
					"name":       input.Name,
					"price":      input.Price,
					"quantity":   input.Quantity,
					"tags":       input.Tags,
					"categoryId": input.CategoryID,
				},
			}, PostError{}
		}, OpenAPIOptions{
			OperationID: "create-product",
			Summary:     "Create a new product",
			Tags:        []string{"products"},
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
			name:           "Valid product",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products",
			body:           `{"name":"Laptop","description":"Gaming laptop","price":999.99,"quantity":10,"tags":["gaming","electronics"]}`,
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Invalid category UUID",
			url:            "/categories/invalid-uuid/products",
			body:           `{"name":"Laptop","price":999.99,"quantity":10}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "uuid4",
		},
		{
			name:           "Negative price",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products",
			body:           `{"name":"Laptop","price":-100,"quantity":10}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Negative quantity",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products",
			body:           `{"name":"Laptop","price":999.99,"quantity":-1}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Invalid tag (empty string)",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products",
			body:           `{"name":"Laptop","price":999.99,"quantity":10,"tags":["gaming",""]}`,
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Tag too long",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products",
			body:           fmt.Sprintf(`{"name":"Laptop","price":999.99,"quantity":10,"tags":["%s"]}`, strings.Repeat("a", 21)),
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, strings.NewReader(tt.body))
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
				if !strings.Contains(bodyStr, "Product") && !strings.Contains(bodyStr, "created") {
					t.Errorf("Expected success message, got %s", bodyStr)
				}
			} else {
				if !strings.Contains(bodyStr, "validation_error") {
					t.Errorf("Expected validation error, got %s", bodyStr)
				}
				if tt.errorContains != "" && !strings.Contains(bodyStr, tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got %s", tt.errorContains, bodyStr)
				}
			}
		})
	}
}

func TestPostOApi_ErrorHandling(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with custom error handling
	Post(oapi, "/users", func(c *fiber.Ctx, input PostUserInput) (PostOutput, PostError) {
		if input.Username == "forbidden" {
			return PostOutput{}, PostError{
				StatusCode: 403,
				Message:    "Username is forbidden",
				Details:    "This username is not allowed",
			}
		}
		if input.Email == "exists@example.com" {
			return PostOutput{}, PostError{
				StatusCode: 409,
				Message:    "Email already exists",
				Details:    "A user with this email already exists",
			}
		}
		return PostOutput{
			ID:      "user_success",
			Message: "User created successfully",
		}, PostError{}
	}, OpenAPIOptions{
		OperationID: "create-user-with-errors",
		Summary:     "Create user with custom error handling",
	})

	// Test 1: Username forbidden
	body1 := `{"username":"forbidden","email":"test@example.com","age":25}`
	req1 := httptest.NewRequest("POST", "/users", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp1.StatusCode != 403 {
		t.Errorf("Expected status 403, got %d", resp1.StatusCode)
	}

	body1Resp, _ := io.ReadAll(resp1.Body)
	if !strings.Contains(string(body1Resp), "Username is forbidden") {
		t.Errorf("Expected forbidden error message, got %s", string(body1Resp))
	}

	// Test 2: Email exists
	body2 := `{"username":"john123","email":"exists@example.com","age":25}`
	req2 := httptest.NewRequest("POST", "/users", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.StatusCode != 409 {
		t.Errorf("Expected status 409, got %d", resp2.StatusCode)
	}

	body2Resp, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2Resp), "Email already exists") {
		t.Errorf("Expected email exists error message, got %s", string(body2Resp))
	}

	// Test 3: Success case
	body3 := `{"username":"john123","email":"john@example.com","age":25}`
	req3 := httptest.NewRequest("POST", "/users", strings.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp3.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp3.StatusCode)
	}

	body3Resp, _ := io.ReadAll(resp3.Body)
	if !strings.Contains(string(body3Resp), "User created successfully") {
		t.Errorf("Expected success message, got %s", string(body3Resp))
	}
}

func TestPostOApi_OperationStorage(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Check that POST operations are properly stored
	initialCount := len(oapi.operations)

	Post(oapi, "/test", func(c *fiber.Ctx, input PostUserInput) (PostOutput, PostError) {
		return PostOutput{Message: "test"}, PostError{}
	}, OpenAPIOptions{
		OperationID: "test-post-operation",
		Summary:     "Test POST operation",
		Tags:        []string{"test", "post"},
		Description: "This is a test POST operation",
	})

	// Check that an operation was added
	if len(oapi.operations) != initialCount+1 {
		t.Errorf("Expected %d operations, got %d", initialCount+1, len(oapi.operations))
	}

	// Check the operation content
	lastOp := oapi.operations[len(oapi.operations)-1]
	if lastOp.Method != "POST" {
		t.Errorf("Expected method POST, got %s", lastOp.Method)
	}
	if lastOp.Path != "/test" {
		t.Errorf("Expected path /test, got %s", lastOp.Path)
	}
	if lastOp.Options.OperationID != "test-post-operation" {
		t.Errorf("Expected operationId test-post-operation, got %s", lastOp.Options.OperationID)
	}
	if lastOp.Options.Summary != "Test POST operation" {
		t.Errorf("Expected summary 'Test POST operation', got %s", lastOp.Options.Summary)
	}
	if lastOp.Options.Description != "This is a test POST operation" {
		t.Errorf("Expected description 'This is a test POST operation', got %s", lastOp.Options.Description)
	}

	// Check the tags
	expectedTags := []string{"test", "post"}
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
