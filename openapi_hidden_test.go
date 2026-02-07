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
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Secret string `json:"secret" openapi:"-"`
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
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &spec))

	components, ok := spec["components"].(map[string]interface{})
	require.True(t, ok, "expected components section in OpenAPI spec")
	schemas, ok := components["schemas"].(map[string]interface{})
	require.True(t, ok, "expected schemas section in components")

	// Check input schema — "internal" field should be hidden
	inputSchema, ok := schemas["HiddenFieldInput"].(map[string]interface{})
	require.True(t, ok, "expected HiddenFieldInput schema")
	inputProps, ok := inputSchema["properties"].(map[string]interface{})
	require.True(t, ok, "expected properties in HiddenFieldInput schema")
	_, hasName := inputProps["name"]
	_, hasInternal := inputProps["internal"]
	assert.True(t, hasName, "visible field 'name' should be in schema")
	assert.False(t, hasInternal, "hidden field 'internal' should NOT be in schema")

	// Check output schema — "secret" field should be hidden
	outputSchema, ok := schemas["HiddenFieldOutput"].(map[string]interface{})
	require.True(t, ok, "expected HiddenFieldOutput schema")
	outputProps, ok := outputSchema["properties"].(map[string]interface{})
	require.True(t, ok, "expected properties in HiddenFieldOutput schema")
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
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &spec))

	paths, ok := spec["paths"].(map[string]interface{})
	require.True(t, ok, "expected paths section in OpenAPI spec")
	searchPath, ok := paths["/search"].(map[string]interface{})
	require.True(t, ok, "expected /search path in spec")
	getOp, ok := searchPath["get"].(map[string]interface{})
	require.True(t, ok, "expected get operation on /search")

	params, ok := getOp["parameters"].([]interface{})
	require.True(t, ok, "expected parameters array on /search get operation")

	// Verify "name" is present and "hidden" is absent
	foundName := false
	for _, p := range params {
		param, ok := p.(map[string]interface{})
		require.True(t, ok, "expected parameter to be a map")
		if param["name"] == "name" {
			foundName = true
		}
		assert.NotEqual(t, "hidden", param["name"], "hidden query param should NOT appear in OpenAPI parameters")
	}
	assert.True(t, foundName, "visible query param 'name' should appear in OpenAPI parameters")
}
