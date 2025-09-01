package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Define nested structs for testing
type Address struct {
	Street  string `json:"street" example:"123 Main St"`
	City    string `json:"city" example:"New York"`
	Country string `json:"country" example:"USA"`
	ZipCode string `json:"zip_code" example:"10001"`
}

type Contact struct {
	Email string `json:"email" example:"user@example.com"`
	Phone string `json:"phone" example:"+1234567890"`
}

type User struct {
	ID      int     `json:"id" example:"1"`
	Name    string  `json:"name" example:"John Doe"`
	Address Address `json:"address"`
	Contact Contact `json:"contact"`
}

type CreateUserRequest struct {
	Name    string  `json:"name" validate:"required" example:"John Doe"`
	Address Address `json:"address" validate:"required"`
	Contact Contact `json:"contact" validate:"required"`
}

type CreateUserResponse struct {
	User    User   `json:"user"`
	Message string `json:"message" example:"User created successfully"`
}

func TestNestedStructSchemaGeneration(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Add a route with nested structs
	Post(oapi, "/users", func(c *fiber.Ctx, req *CreateUserRequest) (*CreateUserResponse, *ErrorResponse) {
		user := User{
			ID:      1,
			Name:    req.Name,
			Address: req.Address,
			Contact: req.Contact,
		}
		return &CreateUserResponse{
			User:    user,
			Message: "User created successfully",
		}, nil
	}, OpenAPIOptions{
		OperationID: "createUser",
		Summary:     "Create a new user",
		Tags:        []string{"users"},
	})

	// Setup documentation
	oapi.SetupDocs()

	// Test OpenAPI spec generation
	req := httptest.NewRequest("GET", "/openapi.json", nil)
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

	// Check that components/schemas exists
	components, ok := spec["components"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected components section in OpenAPI spec")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected schemas section in components")
	}

	// Test that all nested structs are present in schemas
	expectedSchemas := []string{
		"CreateUserRequest",
		"CreateUserResponse",
		"User",
		"Address",
		"Contact",
		"ErrorResponse",
	}

	for _, schemaName := range expectedSchemas {
		if _, exists := schemas[schemaName]; !exists {
			t.Errorf("Expected schema '%s' to be present in OpenAPI spec", schemaName)
		}
	}

	// Verify Address schema structure
	addressSchema, ok := schemas["Address"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Address schema to be an object")
	}

	addressProperties, ok := addressSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Address schema to have properties")
	}

	expectedAddressFields := []string{"street", "city", "country", "zip_code"}
	for _, field := range expectedAddressFields {
		if _, exists := addressProperties[field]; !exists {
			t.Errorf("Expected Address schema to have field '%s'", field)
		}
	}

	// Verify Contact schema structure
	contactSchema, ok := schemas["Contact"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Contact schema to be an object")
	}

	contactProperties, ok := contactSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Contact schema to have properties")
	}

	expectedContactFields := []string{"email", "phone"}
	for _, field := range expectedContactFields {
		if _, exists := contactProperties[field]; !exists {
			t.Errorf("Expected Contact schema to have field '%s'", field)
		}
	}

	// Verify User schema has references to nested structs
	userSchema, ok := schemas["User"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected User schema to be an object")
	}

	userProperties, ok := userSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected User schema to have properties")
	}

	// Check that Address field has a $ref
	addressField, ok := userProperties["address"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected User schema to have address field")
	}

	if addressRef, exists := addressField["$ref"]; !exists || addressRef != "#/components/schemas/Address" {
		t.Errorf("Expected User.address field to have $ref to Address schema, got %v", addressRef)
	}

	// Check that Contact field has a $ref
	contactField, ok := userProperties["contact"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected User schema to have contact field")
	}

	if contactRef, exists := contactField["$ref"]; !exists || contactRef != "#/components/schemas/Contact" {
		t.Errorf("Expected User.contact field to have $ref to Contact schema, got %v", contactRef)
	}

	// Print the generated spec for manual inspection (optional)
	if testing.Verbose() {
		t.Logf("Generated OpenAPI spec:\n%s", string(body))
	}
}

func TestDeeplyNestedStructs(t *testing.T) {
	// Test for deeply nested structures
	type InnerMost struct {
		Value string `json:"value" example:"inner"`
	}

	type Middle struct {
		Inner InnerMost `json:"inner"`
		Name  string    `json:"name" example:"middle"`
	}

	type Outer struct {
		Middle Middle `json:"middle"`
		ID     int    `json:"id" example:"1"`
	}

	type DeepRequest struct {
		Outer Outer `json:"outer"`
	}

	type DeepResponse struct {
		Data    Outer `json:"data"`
		Success bool  `json:"success" example:"true"`
	}

	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/deep", func(c *fiber.Ctx, req *DeepRequest) (*DeepResponse, *ErrorResponse) {
		return &DeepResponse{
			Data:    req.Outer,
			Success: true,
		}, nil
	}, OpenAPIOptions{
		OperationID: "testDeep",
		Summary:     "Test deeply nested structures",
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

	// Check all nested structs are present
	expectedSchemas := []string{"DeepRequest", "DeepResponse", "Outer", "Middle", "InnerMost", "ErrorResponse"}
	for _, schemaName := range expectedSchemas {
		if _, exists := schemas[schemaName]; !exists {
			t.Errorf("Expected deeply nested schema '%s' to be present", schemaName)
		}
	}
}

func TestArrayOfNestedStructs(t *testing.T) {
	type Item struct {
		ID   int    `json:"id" example:"1"`
		Name string `json:"name" example:"Item 1"`
	}

	type ListRequest struct {
		Items []Item `json:"items"`
	}

	type ListResponse struct {
		Items []Item `json:"items"`
		Total int    `json:"total" example:"10"`
	}

	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items", func(c *fiber.Ctx, req *ListRequest) (*ListResponse, *ErrorResponse) {
		return &ListResponse{
			Items: req.Items,
			Total: len(req.Items),
		}, nil
	}, OpenAPIOptions{
		OperationID: "createItems",
		Summary:     "Create multiple items",
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

	// Check that Item struct is collected from array reference
	if _, exists := schemas["Item"]; !exists {
		t.Error("Expected Item schema to be present when referenced in array")
	}

	// Check that ListRequest has correct array schema with $ref
	listRequestSchema := schemas["ListRequest"].(map[string]interface{})
	properties := listRequestSchema["properties"].(map[string]interface{})
	itemsField := properties["items"].(map[string]interface{})

	if itemsField["type"] != "array" {
		t.Error("Expected items field to be of type array")
	}

	itemsSchema := itemsField["items"].(map[string]interface{})
	if itemsSchema["$ref"] != "#/components/schemas/Item" {
		t.Errorf("Expected items array to reference Item schema, got %v", itemsSchema["$ref"])
	}
}
