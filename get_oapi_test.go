package fiberoapi

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Test structs
type SingleParamInput struct {
	Name string `path:"name" validate:"required"`
}

type MultiParamInput struct {
	UserID string `path:"userId" validate:"required"`
	PostID string `path:"postId" validate:"required"`
}

type ParamWithQueryInput struct {
	Name string `path:"name" validate:"required"`
	Lang string `query:"lang"`
}

type ValidationInput struct {
	Name  string `path:"name" validate:"required,min=2"`
	Email string `query:"email" validate:"omitempty,email"`
	Age   int    `query:"age" validate:"omitempty,min=0,max=120"`
}

type MissingParamInput struct {
	Name         string `path:"name"`
	MissingParam string `path:"missing"` // This parameter doesn't exist in the path
}

// Test outputs
type TestOutput struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type TestError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

func TestGetOApi_SingleParam(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with a single parameter
	GetOApi(oapi, "/users/:name", func(c *fiber.Ctx, input SingleParamInput) (TestOutput, TestError) {
		if input.Name == "" {
			return TestOutput{}, TestError{StatusCode: 400, Message: "Name is required"}
		}
		return TestOutput{Message: "Hello " + input.Name}, TestError{}
	}, OpenAPIOptions{
		OperationID: "get-user-by-name",
		Summary:     "Get user by name",
	})

	// Test with valid parameter
	req := httptest.NewRequest("GET", "/users/john", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"message":"Hello john"`) {
		t.Errorf("Expected response to contain 'Hello john', got %s", string(body))
	}
}

func TestGetOApi_MultipleParams(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with multiple parameters
	GetOApi(oapi, "/users/:userId/posts/:postId", func(c *fiber.Ctx, input MultiParamInput) (TestOutput, TestError) {
		return TestOutput{
			Message: "User " + input.UserID + " post " + input.PostID,
			Data: map[string]string{
				"userId": input.UserID,
				"postId": input.PostID,
			},
		}, TestError{}
	}, OpenAPIOptions{
		OperationID: "get-user-post",
		Summary:     "Get user's post",
	})

	// Test with valid parameters
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"message":"User 123 post 456"`) {
		t.Errorf("Expected response to contain 'User 123 post 456', got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"userId":"123"`) {
		t.Errorf("Expected response to contain userId:123, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"postId":"456"`) {
		t.Errorf("Expected response to contain postId:456, got %s", bodyStr)
	}
}

func TestGetOApi_ParamWithQuery(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with path parameter + query
	GetOApi(oapi, "/greeting/:name", func(c *fiber.Ctx, input ParamWithQueryInput) (TestOutput, TestError) {
		message := "Hello " + input.Name
		if input.Lang == "fr" {
			message = "Bonjour " + input.Name
		}
		return TestOutput{
			Message: message,
			Data: map[string]string{
				"name": input.Name,
				"lang": input.Lang,
			},
		}, TestError{}
	}, OpenAPIOptions{
		OperationID: "get-greeting",
		Summary:     "Get greeting with language",
	})

	// Test with parameter and query string
	req := httptest.NewRequest("GET", "/greeting/pierre?lang=fr", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"message":"Bonjour pierre"`) {
		t.Errorf("Expected response to contain 'Bonjour pierre', got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"lang":"fr"`) {
		t.Errorf("Expected response to contain lang:fr, got %s", bodyStr)
	}

	// Test without query string
	req2 := httptest.NewRequest("GET", "/greeting/john", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), `"message":"Hello john"`) {
		t.Errorf("Expected response to contain 'Hello john', got %s", string(body2))
	}
}

func TestGetOApi_MissingParamValidation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test that parameter validation works
	defer func() {
		if r := recover(); r != nil {
			panicMsg := r.(string)
			if !strings.Contains(panicMsg, "Path validation failed") {
				t.Errorf("Expected panic to contain 'Path validation failed', got %s", panicMsg)
			}
			if !strings.Contains(panicMsg, "parameter is not in path") {
				t.Errorf("Expected panic to contain 'parameter is not in path', got %s", panicMsg)
			}
			return // Test successful
		}
		t.Error("Expected panic but got none")
	}()

	// This should panic because the "missing" parameter doesn't exist in the path
	GetOApi(oapi, "/users/:name", func(c *fiber.Ctx, input MissingParamInput) (TestOutput, TestError) {
		return TestOutput{Message: "This should not work"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "should-fail",
	})
}

func TestGetOApi_EmptyPath(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with simple path (no parameters)
	GetOApi(oapi, "/health", func(c *fiber.Ctx, input struct{}) (TestOutput, TestError) {
		return TestOutput{Message: "OK"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "health-check",
		Summary:     "Health check",
	})

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"message":"OK"`) {
		t.Errorf("Expected response to contain 'OK', got %s", string(body))
	}
}

func TestGetOApi_ErrorHandling(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with error handling
	GetOApi(oapi, "/users/:name", func(c *fiber.Ctx, input SingleParamInput) (TestOutput, TestError) {
		if input.Name == "error" {
			return TestOutput{}, TestError{StatusCode: 404, Message: "User not found"}
		}
		return TestOutput{Message: "Hello " + input.Name}, TestError{}
	}, OpenAPIOptions{
		OperationID: "get-user-with-error",
	})

	// Test cas d'erreur
	req := httptest.NewRequest("GET", "/users/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"message":"User not found"`) {
		t.Errorf("Expected response to contain 'User not found', got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"statusCode":404`) {
		t.Errorf("Expected response to contain statusCode:404, got %s", bodyStr)
	}
}

func TestGetOApi_OperationStorage(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Check that operations are properly stored
	initialCount := len(oapi.operations)

	// Input pour le test avec le bon tag
	type TestInput struct {
		ID string `path:"id"`
	}

	GetOApi(oapi, "/test/:id", func(c *fiber.Ctx, input TestInput) (TestOutput, TestError) {
		return TestOutput{Message: "test"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "test-operation",
		Summary:     "Test operation",
		Tags:        []string{"test"},
	})

	// Check that an operation was added
	if len(oapi.operations) != initialCount+1 {
		t.Errorf("Expected %d operations, got %d", initialCount+1, len(oapi.operations))
	}

	// Check the operation content
	lastOp := oapi.operations[len(oapi.operations)-1]
	if lastOp.Method != "GET" {
		t.Errorf("Expected method GET, got %s", lastOp.Method)
	}
	if lastOp.Path != "/test/:id" {
		t.Errorf("Expected path /test/:id, got %s", lastOp.Path)
	}
	if lastOp.Options.OperationID != "test-operation" {
		t.Errorf("Expected operationId test-operation, got %s", lastOp.Options.OperationID)
	}
	if lastOp.Options.Summary != "Test operation" {
		t.Errorf("Expected summary 'Test operation', got %s", lastOp.Options.Summary)
	}

	// Check the tags
	found := false
	for _, tag := range lastOp.Options.Tags {
		if tag == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find 'test' in tags, got %v", lastOp.Options.Tags)
	}
}

func TestGetOApi_Validation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test with advanced validation
	GetOApi(oapi, "/users/:name", func(c *fiber.Ctx, input ValidationInput) (TestOutput, TestError) {
		return TestOutput{
			Message: fmt.Sprintf("Valid user: %s, email: %s, age: %d", input.Name, input.Email, input.Age),
		}, TestError{}
	}, OpenAPIOptions{
		OperationID: "validate-user",
		Summary:     "Validate user input",
	})

	// Test 1: Input valide
	req1 := httptest.NewRequest("GET", "/users/john?email=john@example.com&age=25", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp1.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp1.StatusCode)
	}

	// Test 2: Nom trop court (violation de min=2)
	req2 := httptest.NewRequest("GET", "/users/a?email=test@example.com", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.StatusCode != 400 {
		t.Errorf("Expected status 400 for validation error, got %d", resp2.StatusCode)
	}

	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), "Validation failed") {
		t.Errorf("Expected validation error in response, got %s", string(body2))
	}

	// Test 3: Email invalide
	req3 := httptest.NewRequest("GET", "/users/john?email=invalid-email&age=25", nil)
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp3.StatusCode != 400 {
		t.Errorf("Expected status 400 for invalid email, got %d", resp3.StatusCode)
	}

	// Test 4: Invalid age (too high)
	req4 := httptest.NewRequest("GET", "/users/john?age=150", nil)
	resp4, err := app.Test(req4)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp4.StatusCode != 400 {
		t.Errorf("Expected status 400 for invalid age, got %d", resp4.StatusCode)
	}

	// Test 5: Required parameter missing (no name in path)
	// This test cannot be done because Fiber would return 404 for /users/
	// instead of reaching our handler
}

func TestGetOApi_ValidationRequired(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test avec champ requis
	GetOApi(oapi, "/users/:name", func(c *fiber.Ctx, input SingleParamInput) (TestOutput, TestError) {
		return TestOutput{Message: "Hello " + input.Name}, TestError{}
	}, OpenAPIOptions{
		OperationID: "required-field-test",
	})

	// Test with empty name (shouldn't happen with Fiber path params, but let's test anyway)
	// To simulate, we'll create a structure with a required query field
	type QueryRequiredInput struct {
		Name     string `path:"name"`
		Required string `query:"required" validate:"required"`
	}

	GetOApi(oapi, "/test/:name", func(c *fiber.Ctx, input QueryRequiredInput) (TestOutput, TestError) {
		return TestOutput{Message: "Valid"}, TestError{}
	}, OpenAPIOptions{
		OperationID: "query-required-test",
	})

	// Test without the required parameter
	req := httptest.NewRequest("GET", "/test/john", nil) // no ?required=value
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400 for missing required field, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Validation failed") {
		t.Errorf("Expected validation error in response, got %s", string(body))
	}
}
