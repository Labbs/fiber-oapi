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
