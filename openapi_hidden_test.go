package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Structs for testing openapi:"-" tag

type HiddenFieldInput struct {
	Name     string `json:"name" validate:"required"`
	Internal string `json:"internal" openapi:"-"`
}

type HiddenFieldOutput struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Secret   string `json:"secret" openapi:"-"`
}

type HiddenFieldError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

type HiddenQueryInput struct {
	Name   string `query:"name"`
	Hidden string `query:"hidden" openapi:"-"`
}

func TestOpenAPIHiddenField_ExcludedFromBodySchema(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items", func(c *fiber.Ctx, input *HiddenFieldInput) (*HiddenFieldOutput, *HiddenFieldError) {
		return &HiddenFieldOutput{ID: 1, Name: input.Name}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Summary:     "Create item",
		Tags:        []string{"items"},
	})

	oapi.SetupDocs()

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &spec))

	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})

	// Check input schema — "internal" field should be hidden
	inputSchema := schemas["HiddenFieldInput"].(map[string]interface{})
	inputProps := inputSchema["properties"].(map[string]interface{})
	_, hasName := inputProps["name"]
	_, hasInternal := inputProps["internal"]
	assert.True(t, hasName, "visible field 'name' should be in schema")
	assert.False(t, hasInternal, "hidden field 'internal' should NOT be in schema")

	// Check output schema — "secret" field should be hidden
	outputSchema := schemas["HiddenFieldOutput"].(map[string]interface{})
	outputProps := outputSchema["properties"].(map[string]interface{})
	_, hasID := outputProps["id"]
	_, hasSecret := outputProps["secret"]
	assert.True(t, hasID, "visible field 'id' should be in schema")
	assert.False(t, hasSecret, "hidden field 'secret' should NOT be in schema")
}

func TestOpenAPIHiddenField_ExcludedFromQueryParams(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Get(oapi, "/search", func(c *fiber.Ctx, input HiddenQueryInput) (*HiddenFieldOutput, *HiddenFieldError) {
		return &HiddenFieldOutput{ID: 1, Name: input.Name}, nil
	}, OpenAPIOptions{
		OperationID: "searchItems",
		Summary:     "Search items",
	})

	oapi.SetupDocs()

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &spec))

	paths := spec["paths"].(map[string]interface{})
	searchPath := paths["/search"].(map[string]interface{})
	getOp := searchPath["get"].(map[string]interface{})

	params, hasParams := getOp["parameters"].([]interface{})
	if hasParams {
		for _, p := range params {
			param := p.(map[string]interface{})
			assert.NotEqual(t, "hidden", param["name"], "hidden query param should NOT appear in OpenAPI parameters")
		}
	}
}
