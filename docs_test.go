package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSetupDocs_Documentation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Setup documentation
	oapi.SetupDocs()

	// Test documentation endpoint
	req := httptest.NewRequest("GET", "/docs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	// Check HTML content contains Redoc
	body, _ := io.ReadAll(resp.Body)
	htmlContent := string(body)

	if !strings.Contains(htmlContent, "redoc") {
		t.Error("Expected HTML to contain Redoc elements")
	}
	if !strings.Contains(htmlContent, "/openapi.json") {
		t.Error("Expected HTML to reference OpenAPI JSON endpoint")
	}
	if !strings.Contains(htmlContent, "API Documentation") {
		t.Error("Expected HTML to contain default title")
	}
}

func TestSetupDocs_CustomConfig(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Add a test route
	Get(oapi, "/health", func(c *fiber.Ctx, input struct{}) (interface{}, struct{}) {
		return map[string]string{"status": "ok"}, struct{}{}
	}, OpenAPIOptions{
		OperationID: "health-check",
		Summary:     "Health check",
	})

	// Setup documentation with custom config
	customConfig := DocConfig{
		Title:       "My Custom API",
		Description: "Custom API description",
		Version:     "2.0.0",
		DocsPath:    "/api-docs",
		JSONPath:    "/api-spec.json",
	}
	oapi.SetupDocs(customConfig)

	// Test custom JSON endpoint
	req := httptest.NewRequest("GET", "/api-spec.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse the OpenAPI spec
	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}

	// Check custom info
	if info, ok := spec["info"].(map[string]interface{}); ok {
		if info["title"] != "My Custom API" {
			t.Errorf("Expected title 'My Custom API', got %v", info["title"])
		}
		if info["description"] != "Custom API description" {
			t.Errorf("Expected custom description, got %v", info["description"])
		}
		if info["version"] != "2.0.0" {
			t.Errorf("Expected version '2.0.0', got %v", info["version"])
		}
	} else {
		t.Error("Expected info section in OpenAPI spec")
	}

	// Test custom docs endpoint
	req2 := httptest.NewRequest("GET", "/api-docs", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	body2, _ := io.ReadAll(resp2.Body)
	htmlContent := string(body2)

	if !strings.Contains(htmlContent, "My Custom API") {
		t.Error("Expected HTML to contain custom title")
	}
	if !strings.Contains(htmlContent, "/api-spec.json") {
		t.Error("Expected HTML to reference custom JSON endpoint")
	}
}

func TestConvertFiberPathToOpenAPI(t *testing.T) {
	tests := []struct {
		fiberPath    string
		expectedPath string
	}{
		{"/users/:id", "/users/{id}"},
		{"/users/:userId/posts/:postId", "/users/{userId}/posts/{postId}"},
		{"/simple", "/simple"},
		{"/mixed/:id/static/:name/end", "/mixed/{id}/static/{name}/end"},
		{"/:single", "/{single}"},
		{"/", "/"},
	}

	for _, test := range tests {
		result := convertFiberPathToOpenAPI(test.fiberPath)
		if result != test.expectedPath {
			t.Errorf("convertFiberPathToOpenAPI(%s) = %s; expected %s",
				test.fiberPath, result, test.expectedPath)
		}
	}
}
