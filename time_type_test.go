package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Workspace struct {
	WorkspaceID string    `json:"workspace_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkspaceResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

type EmptyRequest struct{}

func TestTimeTypeRendersAsDateTimeString(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/workspaces", func(c *fiber.Ctx, req *EmptyRequest) (*WorkspaceResponse, *ErrorResponse) {
		return &WorkspaceResponse{}, nil
	}, OpenAPIOptions{
		OperationID: "listWorkspaces",
		Tags:        []string{"workspaces"},
	})

	oapi.SetupDocs()

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}

	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})

	if _, exists := schemas["Time"]; exists {
		t.Errorf("time.Time should not be registered as a 'Time' component schema")
	}

	workspaceSchema, ok := schemas["Workspace"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Workspace schema to be present")
	}
	props := workspaceSchema["properties"].(map[string]interface{})

	for _, field := range []string{"created_at", "updated_at"} {
		f, ok := props[field].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected Workspace.%s to be an object", field)
		}
		if f["type"] != "string" {
			t.Errorf("Expected %s.type to be 'string', got %v", field, f["type"])
		}
		if f["format"] != "date-time" {
			t.Errorf("Expected %s.format to be 'date-time', got %v", field, f["format"])
		}
		if _, hasRef := f["$ref"]; hasRef {
			t.Errorf("Expected %s to be inlined, not a $ref", field)
		}
	}
}

type EventWithPointerTime struct {
	Name      string     `json:"name"`
	StartedAt *time.Time `json:"started_at,omitempty"`
}

func TestPointerTimeTypeRendersAsDateTimeString(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/events", func(c *fiber.Ctx, req *EmptyRequest) (*EventWithPointerTime, *ErrorResponse) {
		return &EventWithPointerTime{}, nil
	}, OpenAPIOptions{
		OperationID: "createEvent",
		Tags:        []string{"events"},
	})

	oapi.SetupDocs()

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}

	schemas := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	eventSchema := schemas["EventWithPointerTime"].(map[string]interface{})
	props := eventSchema["properties"].(map[string]interface{})

	startedAt, ok := props["started_at"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected started_at property to be present")
	}
	if startedAt["type"] != "string" || startedAt["format"] != "date-time" {
		t.Errorf("Expected *time.Time to render as string/date-time, got %v", startedAt)
	}
}

func TestTimeTypeAsTopLevelInputAndOutput(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/timestamp", func(c *fiber.Ctx, req *time.Time) (*time.Time, *ErrorResponse) {
		return req, nil
	}, OpenAPIOptions{
		OperationID: "echoTimestamp",
		Tags:        []string{"timestamps"},
	})

	oapi.SetupDocs()

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}

	paths := spec["paths"].(map[string]interface{})
	timestamp := paths["/timestamp"].(map[string]interface{})
	post := timestamp["post"].(map[string]interface{})

	reqBody := post["requestBody"].(map[string]interface{})
	reqSchema := reqBody["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
	if _, hasRef := reqSchema["$ref"]; hasRef {
		t.Errorf("Expected top-level time.Time request body to be inlined, got $ref: %v", reqSchema["$ref"])
	}
	if reqSchema["type"] != "string" || reqSchema["format"] != "date-time" {
		t.Errorf("Expected request body schema to be string/date-time, got %v", reqSchema)
	}

	respSchema := post["responses"].(map[string]interface{})["200"].(map[string]interface{})["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
	if _, hasRef := respSchema["$ref"]; hasRef {
		t.Errorf("Expected top-level time.Time response to be inlined, got $ref: %v", respSchema["$ref"])
	}
	if respSchema["type"] != "string" || respSchema["format"] != "date-time" {
		t.Errorf("Expected response schema to be string/date-time, got %v", respSchema)
	}

	if schemas, ok := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{}); ok {
		if _, exists := schemas["Time"]; exists {
			t.Errorf("time.Time should not produce a 'Time' component schema")
		}
	}
}

func TestTimeTypeAsTopLevelErrorBody(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/timestamp-error", func(c *fiber.Ctx, req *EmptyRequest) (*EmptyRequest, *time.Time) {
		return &EmptyRequest{}, nil
	}, OpenAPIOptions{
		OperationID: "timestampError",
		Tags:        []string{"timestamps"},
	})

	oapi.SetupDocs()

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}

	post := spec["paths"].(map[string]interface{})["/timestamp-error"].(map[string]interface{})["post"].(map[string]interface{})
	errSchema := post["responses"].(map[string]interface{})["400"].(map[string]interface{})["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})

	if _, hasRef := errSchema["$ref"]; hasRef {
		t.Errorf("Expected top-level time.Time error body to be inlined, got $ref: %v", errSchema["$ref"])
	}
	if errSchema["type"] != "string" || errSchema["format"] != "date-time" {
		t.Errorf("Expected error schema to be string/date-time, got %v", errSchema)
	}

	if schemas, ok := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{}); ok {
		if _, exists := schemas["Time"]; exists {
			t.Errorf("time.Time should not produce a 'Time' component schema")
		}
	}
}
