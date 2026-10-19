package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
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

// proxyInput mimics a handler that trusts identity headers set by a proxy.
type proxyInput struct {
	ID     string `uri:"id"`
	UserID string `header:"X-User-Id"`
	Filter string `query:"filter"`
	Name   string `json:"name"`
	// Sort declares both: it is accepted from the query or from the body.
	Sort string `query:"sort" json:"sort"`
}

func TestBindingOrder_AbsentParametersAreNotFilledFromTheBody(t *testing.T) {
	var got proxyInput
	app := fiber.New()
	oapi := New(app)
	Put(oapi, "/items/:id", func(c fiber.Ctx, in proxyInput) (fiber.Map, struct{}) {
		got = in
		return fiber.Map{}, struct{}{}
	}, OpenAPIOptions{})

	send := func(contentType, body, query string) {
		t.Helper()
		got = proxyInput{}
		req := httptest.NewRequest("PUT", "/items/from-path"+query, strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
	}

	send("application/json", `{"id":"evil","userId":"evil","x-user-id":"evil","filter":"evil","name":"widget","sort":"from-body"}`, "")
	assert.Equal(t, "from-path", got.ID)
	assert.Empty(t, got.UserID, "a header absent from the request must not come from the body")
	assert.Empty(t, got.Filter, "a query parameter absent from the request must not come from the body")
	assert.Equal(t, "widget", got.Name)
	assert.Equal(t, "from-body", got.Sort, "a field that also declares a json tag accepts the body")

	send("application/json", `{"sort":"from-body"}`, "?sort=from-query")
	assert.Equal(t, "from-query", got.Sort, "the query still wins when present")

	send("application/x-www-form-urlencoded", "UserID=evil&Filter=evil&ID=evil", "")
	assert.Equal(t, "from-path", got.ID)
	assert.Empty(t, got.UserID, "the same holds for form bodies")
	assert.Empty(t, got.Filter)
}

func TestParamOnlyFields(t *testing.T) {
	type input struct {
		Path     string `uri:"id"`
		Header   string `header:"X-A"`
		Hidden   string `header:"X-B" json:"-"`
		Both     string `query:"q" json:"q,omitempty"`
		FormBoth string `query:"f" form:"f"`
		Body     string `json:"body"`
		private  string `header:"X-C"`
	}
	assert.Equal(t, []int{0, 1, 2}, paramOnlyFields(reflect.TypeFor[input]()))
	assert.Equal(t, []int{0, 1, 2}, paramOnlyFields(reflect.TypeFor[*input]()))
	assert.Nil(t, paramOnlyFields(reflect.TypeFor[string]()))
	assert.Nil(t, paramOnlyFields(nil))
}
