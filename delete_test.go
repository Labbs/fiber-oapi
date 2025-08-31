package fiberoapi

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Test structs pour DELETE
type DeleteUserInput struct {
	ID string `path:"id" validate:"required"`
}

type DeleteProductInput struct {
	CategoryID string `path:"categoryId" validate:"required,uuid4"`
	ProductID  string `path:"productId" validate:"required,uuid4"`
}

type DeleteWithQueryInput struct {
	ID     string `path:"id" validate:"required"`
	Force  bool   `query:"force"`
	Reason string `query:"reason" validate:"omitempty,min=5,max=100"`
}

type DeleteOutput struct {
	ID      string      `json:"id"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Deleted bool        `json:"deleted"`
}

type DeleteError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

func TestDeleteOApi_SimpleDelete(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with simple DELETE (path param only)
	Delete(oapi, "/users/:id", func(c *fiber.Ctx, input DeleteUserInput) (DeleteOutput, DeleteError) {
		return DeleteOutput{
			ID:      input.ID,
			Message: fmt.Sprintf("User %s deleted successfully", input.ID),
			Deleted: true,
		}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "delete-user",
		Summary:     "Delete a user",
		Tags:        []string{"users"},
	})

	// Test with valid ID
	req := httptest.NewRequest("DELETE", "/users/user123", nil)
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
	if !strings.Contains(respStr, `"deleted":true`) {
		t.Errorf("Expected response to contain deleted flag, got %s", respStr)
	}
	if !strings.Contains(respStr, "deleted successfully") {
		t.Errorf("Expected response to contain success message, got %s", respStr)
	}
}

func TestDeleteOApi_MultiplePathParams(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with DELETE + multiple path parameters
	Delete(oapi, "/categories/:categoryId/products/:productId", func(c *fiber.Ctx, input DeleteProductInput) (DeleteOutput, DeleteError) {
		return DeleteOutput{
			ID:      input.ProductID,
			Message: fmt.Sprintf("Product %s deleted from category %s", input.ProductID, input.CategoryID),
			Data: map[string]interface{}{
				"categoryId": input.CategoryID,
				"productId":  input.ProductID,
			},
			Deleted: true,
		}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "delete-product",
		Summary:     "Delete a product from category",
		Tags:        []string{"products"},
	})

	// Test with valid UUIDs
	req := httptest.NewRequest("DELETE", "/categories/550e8400-e29b-41d4-a716-446655440000/products/550e8400-e29b-41d4-a716-446655440001", nil)
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
	if !strings.Contains(respStr, `"deleted":true`) {
		t.Errorf("Expected response to contain deleted flag, got %s", respStr)
	}
}

func TestDeleteOApi_WithQueryParams(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with DELETE + path + query parameters
	Delete(oapi, "/users/:id", func(c *fiber.Ctx, input DeleteWithQueryInput) (DeleteOutput, DeleteError) {
		message := fmt.Sprintf("User %s deleted", input.ID)
		if input.Force {
			message += " (forced)"
		}
		if input.Reason != "" {
			message += fmt.Sprintf(" - Reason: %s", input.Reason)
		}

		return DeleteOutput{
			ID:      input.ID,
			Message: message,
			Data: map[string]interface{}{
				"force":  input.Force,
				"reason": input.Reason,
			},
			Deleted: true,
		}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "delete-user-with-options",
		Summary:     "Delete a user with options",
		Tags:        []string{"users"},
	})

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		shouldPass     bool
		expectedText   string
	}{
		{
			name:           "Simple delete",
			url:            "/users/user123",
			expectedStatus: 200,
			shouldPass:     true,
			expectedText:   "User user123 deleted",
		},
		{
			name:           "Delete with force",
			url:            "/users/user456?force=true",
			expectedStatus: 200,
			shouldPass:     true,
			expectedText:   "(forced)",
		},
		{
			name:           "Delete with reason",
			url:            "/users/user789?reason=Account%20violation",
			expectedStatus: 200,
			shouldPass:     true,
			expectedText:   "Account violation",
		},
		{
			name:           "Delete with force and reason",
			url:            "/users/user999?force=true&reason=Security%20breach",
			expectedStatus: 200,
			shouldPass:     true,
			expectedText:   "Security breach",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", tt.url, nil)
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
				if !strings.Contains(bodyStr, `"deleted":true`) {
					t.Errorf("Expected deleted flag, got %s", bodyStr)
				}
				if tt.expectedText != "" && !strings.Contains(bodyStr, tt.expectedText) {
					t.Errorf("Expected text '%s' in response, got %s", tt.expectedText, bodyStr)
				}
			}
		})
	}
}

func TestDeleteOApi_Validation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Delete(oapi, "/categories/:categoryId/products/:productId", func(c *fiber.Ctx, input DeleteProductInput) (DeleteOutput, DeleteError) {
		return DeleteOutput{
			ID:      input.ProductID,
			Message: "Product deleted",
			Deleted: true,
		}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "delete-product-validation",
		Summary:     "Delete product with validation",
	})

	Delete(oapi, "/users/:id", func(c *fiber.Ctx, input DeleteWithQueryInput) (DeleteOutput, DeleteError) {
		return DeleteOutput{
			ID:      input.ID,
			Message: "User deleted",
			Deleted: true,
		}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "delete-user-validation",
		Summary:     "Delete user with validation",
	})

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		shouldPass     bool
		errorContains  string
	}{
		{
			name:           "Valid UUID path params",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products/550e8400-e29b-41d4-a716-446655440001",
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Invalid category UUID",
			url:            "/categories/invalid-uuid/products/550e8400-e29b-41d4-a716-446655440001",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "uuid4",
		},
		{
			name:           "Invalid product UUID",
			url:            "/categories/550e8400-e29b-41d4-a716-446655440000/products/invalid-uuid",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "uuid4",
		},
		{
			name:           "Valid user delete",
			url:            "/users/user123",
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Valid reason length",
			url:            "/users/user123?reason=Valid%20reason%20text",
			expectedStatus: 200,
			shouldPass:     true,
		},
		{
			name:           "Reason too short",
			url:            "/users/user123?reason=bad",
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "min",
		},
		{
			name:           "Reason too long",
			url:            fmt.Sprintf("/users/user123?reason=%s", strings.Repeat("a", 101)),
			expectedStatus: 400,
			shouldPass:     false,
			errorContains:  "max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", tt.url, nil)
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
				if !strings.Contains(bodyStr, "deleted") {
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

func TestDeleteOApi_ErrorHandling(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with custom error handling
	Delete(oapi, "/users/:id", func(c *fiber.Ctx, input DeleteUserInput) (DeleteOutput, DeleteError) {
		if input.ID == "notfound" {
			return DeleteOutput{}, DeleteError{
				StatusCode: 404,
				Message:    "User not found",
				Details:    "The specified user does not exist",
			}
		}
		if input.ID == "protected" {
			return DeleteOutput{}, DeleteError{
				StatusCode: 403,
				Message:    "User is protected",
				Details:    "This user cannot be deleted",
			}
		}
		if input.ID == "hasdata" {
			return DeleteOutput{}, DeleteError{
				StatusCode: 409,
				Message:    "User has associated data",
				Details:    "Cannot delete user with existing data",
			}
		}
		return DeleteOutput{
			ID:      input.ID,
			Message: "User deleted successfully",
			Deleted: true,
		}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "delete-user-with-errors",
		Summary:     "Delete user with custom error handling",
	})

	// Test 1: User not found
	req1 := httptest.NewRequest("DELETE", "/users/notfound", nil)
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

	// Test 2: Protected user
	req2 := httptest.NewRequest("DELETE", "/users/protected", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.StatusCode != 403 {
		t.Errorf("Expected status 403, got %d", resp2.StatusCode)
	}

	body2Resp, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2Resp), "User is protected") {
		t.Errorf("Expected protected error message, got %s", string(body2Resp))
	}

	// Test 3: User has data
	req3 := httptest.NewRequest("DELETE", "/users/hasdata", nil)
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp3.StatusCode != 409 {
		t.Errorf("Expected status 409, got %d", resp3.StatusCode)
	}

	body3Resp, _ := io.ReadAll(resp3.Body)
	if !strings.Contains(string(body3Resp), "User has associated data") {
		t.Errorf("Expected conflict error message, got %s", string(body3Resp))
	}

	// Test 4: Success case
	req4 := httptest.NewRequest("DELETE", "/users/user123", nil)
	resp4, err := app.Test(req4)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp4.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp4.StatusCode)
	}

	body4Resp, _ := io.ReadAll(resp4.Body)
	if !strings.Contains(string(body4Resp), "User deleted successfully") {
		t.Errorf("Expected success message, got %s", string(body4Resp))
	}
	if !strings.Contains(string(body4Resp), `"deleted":true`) {
		t.Errorf("Expected deleted flag, got %s", string(body4Resp))
	}
}

func TestDeleteOApi_OperationStorage(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Check that DELETE operations are properly stored
	initialCount := len(oapi.operations)

	Delete(oapi, "/test/:id", func(c *fiber.Ctx, input DeleteUserInput) (DeleteOutput, DeleteError) {
		return DeleteOutput{Message: "test", Deleted: true}, DeleteError{}
	}, OpenAPIOptions{
		OperationID: "test-delete-operation",
		Summary:     "Test DELETE operation",
		Tags:        []string{"test", "delete"},
		Description: "This is a test DELETE operation",
	})

	// Check that an operation was added
	if len(oapi.operations) != initialCount+1 {
		t.Errorf("Expected %d operations, got %d", initialCount+1, len(oapi.operations))
	}

	// Check the operation content
	lastOp := oapi.operations[len(oapi.operations)-1]
	if lastOp.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", lastOp.Method)
	}
	if lastOp.Path != "/test/:id" {
		t.Errorf("Expected path /test/:id, got %s", lastOp.Path)
	}
	if lastOp.Options.OperationID != "test-delete-operation" {
		t.Errorf("Expected operationId test-delete-operation, got %s", lastOp.Options.OperationID)
	}
	if lastOp.Options.Summary != "Test DELETE operation" {
		t.Errorf("Expected summary 'Test DELETE operation', got %s", lastOp.Options.Summary)
	}
	if lastOp.Options.Description != "This is a test DELETE operation" {
		t.Errorf("Expected description 'This is a test DELETE operation', got %s", lastOp.Options.Description)
	}

	// Check the tags
	expectedTags := []string{"test", "delete"}
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
