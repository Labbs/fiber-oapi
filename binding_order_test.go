package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bindingOrderInput carries path, query and header fields without json:"-",
// so the JSON decoder (which matches keys case-insensitively) can reach them.
type bindingOrderInput struct {
	ID     string `uri:"id" validate:"required"`
	Filter string `query:"filter"`
	Tenant string `header:"X-Tenant"`
	Name   string `json:"name"`
}

type bindingOrderOutput struct {
	ID     string `json:"id"`
	Filter string `json:"filter"`
	Tenant string `json:"tenant"`
	Name   string `json:"name"`
}

func TestBindingOrder_BodyCannotOverrideRequestParameters(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Put(oapi, "/items/:id", func(c fiber.Ctx, in bindingOrderInput) (bindingOrderOutput, struct{}) {
		return bindingOrderOutput(in), struct{}{}
	}, OpenAPIOptions{})

	body := `{"id":"from-body","filter":"from-body","tenant":"from-body","name":"widget"}`
	req := httptest.NewRequest("PUT", "/items/from-path?filter=from-query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant", "from-header")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var out bindingOrderOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "from-path", out.ID, "the path parameter must win over a body key with the same name")
	assert.Equal(t, "from-query", out.Filter, "the query parameter must win over the body")
	assert.Equal(t, "from-header", out.Tenant, "the header must win over the body")
	assert.Equal(t, "widget", out.Name, "regular body fields are still bound")
}

func TestBindingOrder_BodySchemaExcludesPathParameters(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Put(oapi, "/items/:id", func(c fiber.Ctx, in bindingOrderInput) (bindingOrderOutput, struct{}) {
		return bindingOrderOutput(in), struct{}{}
	}, OpenAPIOptions{})

	spec := oapi.GenerateOpenAPISpec()
	schema := spec["components"].(map[string]any)["schemas"].(map[string]any)["bindingOrderInput"].(map[string]any)
	props := schema["properties"].(map[string]any)
	assert.NotContains(t, props, "ID", "uri fields are path parameters, not body properties")
	assert.NotContains(t, props, "Filter")
	assert.NotContains(t, props, "Tenant")
	assert.Contains(t, props, "name")
	assert.NotContains(t, schema, "required", "the only required field is the path parameter")
}
