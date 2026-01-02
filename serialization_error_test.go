package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// UnserializableOutput contains a channel which cannot be serialized to JSON
type UnserializableOutput struct {
	Name    string      `json:"name"`
	Channel chan string `json:"channel"` // Channels cannot be JSON serialized
}

type SerializationTestInput struct{}

type SerializationTestError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

func TestResponseSerializationError(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Handler that returns an unserializable output (contains a channel)
	Get(oapi, "/unserializable", func(c *fiber.Ctx, input SerializationTestInput) (UnserializableOutput, *SerializationTestError) {
		return UnserializableOutput{
			Name:    "test",
			Channel: make(chan string), // This will fail JSON marshaling
		}, nil
	}, OpenAPIOptions{
		Summary: "Test endpoint with unserializable response",
	})

	req := httptest.NewRequest("GET", "/unserializable", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// Should return 500 status code
	if resp.StatusCode != 500 {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Should return a proper error response
	body, _ := io.ReadAll(resp.Body)
	var errorResp ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v. Body: %s", err, string(body))
	}

	if errorResp.Code != 500 {
		t.Errorf("Expected error code 500, got %d", errorResp.Code)
	}

	if errorResp.Type != "serialization_error" {
		t.Errorf("Expected error type 'serialization_error', got '%s'", errorResp.Type)
	}
}

// UnserializableError contains a channel which cannot be serialized
type UnserializableError struct {
	StatusCode int         `json:"statusCode"`
	Channel    chan string `json:"channel"`
}

func TestErrorSerializationError(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Handler that returns an unserializable error
	Get(oapi, "/unserializable-error", func(c *fiber.Ctx, input SerializationTestInput) (map[string]string, *UnserializableError) {
		return nil, &UnserializableError{
			StatusCode: 400,
			Channel:    make(chan string), // This will fail JSON marshaling
		}
	}, OpenAPIOptions{
		Summary: "Test endpoint with unserializable error",
	})

	req := httptest.NewRequest("GET", "/unserializable-error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// Should return 500 status code (fallback error)
	if resp.StatusCode != 500 {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Should return a proper error response
	body, _ := io.ReadAll(resp.Body)
	var errorResp map[string]string
	if err := json.Unmarshal(body, &errorResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v. Body: %s", err, string(body))
	}

	if errorResp["error"] != "Failed to serialize error response" {
		t.Errorf("Expected error message 'Failed to serialize error response', got '%s'", errorResp["error"])
	}
}
