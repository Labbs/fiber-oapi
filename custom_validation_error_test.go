package fiberoapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// CustomValidationError represents a custom error structure
type CustomValidationError struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func TestCustomValidationErrorHandler(t *testing.T) {
	app := fiber.New()

	// Configure with custom validation error handler
	oapi := New(app, Config{
		EnableValidation:  true,
		EnableOpenAPIDocs: false,
		ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusBadRequest).JSON(CustomValidationError{
				Success: false,
				Message: err.Error(),
				Code:    "CUSTOM_VALIDATION_ERROR",
			})
		},
	})

	type TestInput struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	type TestOutput struct {
		Message string `json:"message"`
	}

	Post[TestInput, TestOutput, struct{}](
		oapi,
		"/test",
		func(c *fiber.Ctx, input TestInput) (TestOutput, struct{}) {
			return TestOutput{Message: "success"}, struct{}{}
		},
		OpenAPIOptions{},
	)

	// Test with invalid input (missing required field)
	reqBody := map[string]interface{}{
		"name": "ab", // Too short (min=3)
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify custom error structure
	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response body: %s", string(body))
	var customErr CustomValidationError
	err = json.Unmarshal(body, &customErr)
	assert.NoError(t, err)
	assert.False(t, customErr.Success)
	assert.Equal(t, "CUSTOM_VALIDATION_ERROR", customErr.Code)
	assert.NotEmpty(t, customErr.Message)
}

func TestDefaultValidationErrorWhenNoCustomHandler(t *testing.T) {
	app := fiber.New()

	// Configure without custom validation error handler
	oapi := New(app, Config{
		EnableValidation:  true,
		EnableOpenAPIDocs: false,
	})

	type TestInput struct {
		Name string `json:"name" validate:"required"`
	}

	type TestOutput struct {
		Message string `json:"message"`
	}

	Post[TestInput, TestOutput, struct{}](
		oapi,
		"/test",
		func(c *fiber.Ctx, input TestInput) (TestOutput, struct{}) {
			return TestOutput{Message: "success"}, struct{}{}
		},
		OpenAPIOptions{},
	)

	// Test with invalid input
	reqBody := map[string]interface{}{}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify default error structure (ErrorResponse)
	body, _ := io.ReadAll(resp.Body)
	var defaultErr ErrorResponse
	err = json.Unmarshal(body, &defaultErr)
	assert.NoError(t, err)
	assert.Equal(t, 400, defaultErr.Code)
	assert.Equal(t, "validation_error", defaultErr.Type)
	assert.NotEmpty(t, defaultErr.Details)
}

func TestCustomValidationErrorHandlerWithDisabledDocs(t *testing.T) {
	app := fiber.New()

	// Configure with custom validation error handler AND EnableOpenAPIDocs: false
	// This tests that boolean config is properly respected when ValidationErrorHandler is set
	oapi := New(app, Config{
		EnableValidation:  true,
		EnableOpenAPIDocs: false, // This should be respected
		ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusBadRequest).JSON(CustomValidationError{
				Success: false,
				Message: err.Error(),
				Code:    "CUSTOM_ERROR",
			})
		},
	})

	type TestInput struct {
		Name string `json:"name" validate:"required"`
	}

	type TestOutput struct {
		Message string `json:"message"`
	}

	Post[TestInput, TestOutput, struct{}](
		oapi,
		"/test",
		func(c *fiber.Ctx, input TestInput) (TestOutput, struct{}) {
			return TestOutput{Message: "success"}, struct{}{}
		},
		OpenAPIOptions{},
	)

	// Test validation error uses custom handler
	reqBody := map[string]interface{}{}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify custom error structure
	body, _ := io.ReadAll(resp.Body)
	var customErr CustomValidationError
	err = json.Unmarshal(body, &customErr)
	assert.NoError(t, err)
	assert.Equal(t, "CUSTOM_ERROR", customErr.Code)

	// Verify that EnableOpenAPIDocs: false was respected
	// The docs endpoint should not exist
	req = httptest.NewRequest("GET", "/docs", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestValidationErrorHandlerImpliesValidationEnabled(t *testing.T) {
	app := fiber.New()

	// Configure ONLY ValidationErrorHandler without explicitly setting EnableValidation
	// This should keep validation enabled by default since it makes sense
	oapi := New(app, Config{
		ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusBadRequest).JSON(CustomValidationError{
				Success: false,
				Message: err.Error(),
				Code:    "VALIDATION_HANDLER_ACTIVE",
			})
		},
	})

	type TestInput struct {
		Name string `json:"name" validate:"required,min=3"`
	}

	type TestOutput struct {
		Message string `json:"message"`
	}

	Post[TestInput, TestOutput, struct{}](
		oapi,
		"/test",
		func(c *fiber.Ctx, input TestInput) (TestOutput, struct{}) {
			return TestOutput{Message: "success"}, struct{}{}
		},
		OpenAPIOptions{},
	)

	// Test that validation is still active and uses custom handler
	reqBody := map[string]interface{}{
		"name": "ab", // Too short (min=3)
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify validation is active and custom handler was used
	body, _ := io.ReadAll(resp.Body)
	var customErr CustomValidationError
	err = json.Unmarshal(body, &customErr)
	assert.NoError(t, err)
	assert.Equal(t, "VALIDATION_HANDLER_ACTIVE", customErr.Code)
	assert.Contains(t, customErr.Message, "min")

	// Test with valid data to ensure endpoint works
	reqBody = map[string]interface{}{
		"name": "John Doe",
	}
	bodyBytes, _ = json.Marshal(reqBody)

	req = httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
