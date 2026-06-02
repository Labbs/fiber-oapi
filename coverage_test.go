package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Thin helper tests targeting code paths previously at 0% coverage so the
// project stays comfortably above 80% as more features ship.

type covOut struct {
	Message string `json:"message"`
}

func TestHeadHelperRoutesAndSurfacesInSpec(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Head(oapi, "/ping", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "ping"})

	resp, err := app.Test(httptest.NewRequest("HEAD", "/ping", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	spec := oapi.GenerateOpenAPISpec()
	pingPath := spec["paths"].(map[string]any)["/ping"].(map[string]any)
	_, hasHead := pingPath["head"]
	assert.True(t, hasHead, "HEAD operation should appear in the spec")
}

func TestPatchHelperRoutesAndSurfacesInSpec(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	type patchInput struct {
		ID string `uri:"id" validate:"required"`
	}

	Patch(oapi, "/users/:id", func(c fiber.Ctx, in patchInput) (covOut, struct{}) {
		return covOut{Message: "patched " + in.ID}, struct{}{}
	}, OpenAPIOptions{OperationID: "patchUser"})

	resp, err := app.Test(httptest.NewRequest("PATCH", "/users/42", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	var out covOut
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "patched 42", out.Message)

	spec := oapi.GenerateOpenAPISpec()
	usersPath := spec["paths"].(map[string]any)["/users/{id}"].(map[string]any)
	_, hasPatch := usersPath["patch"]
	assert.True(t, hasPatch, "PATCH operation should appear in the spec")
}

func TestGroupPackageLevelHelper_RouterApp(t *testing.T) {
	// The free function Group(router, ...) dispatches to either *OApiApp or
	// *OApiGroup. Cover the *OApiApp branch.
	app := fiber.New()
	oapi := New(app)
	v1 := Group(oapi, "/api/v1")

	Get(v1, "/health", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "health"})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/health", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGroupPackageLevelHelper_RouterNestedGroup(t *testing.T) {
	// Cover the *OApiGroup branch of the free Group helper (sub-group).
	app := fiber.New()
	oapi := New(app)
	v1 := Group(oapi, "/api/v1")
	users := Group(v1, "/users")

	Get(users, "/me", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "me"}, struct{}{}
	}, OpenAPIOptions{OperationID: "me"})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/users/me", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGroupPackageLevelHelper_UnsupportedRouterPanics(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r, "Group with unsupported router type should panic")
	}()
	type fake struct{ OApiRouter }
	Group(fake{}, "/x")
}

func TestOApiAppUse_PassesMiddlewareToFiber(t *testing.T) {
	// Verify the OApiApp.Use thin passthrough actually plumbs the middleware
	// down to fiber.App.Use so registered handlers run on every request.
	app := fiber.New()
	oapi := New(app)

	calls := 0
	oapi.Use(func(c fiber.Ctx) error {
		calls++
		return c.Next()
	})

	Get(oapi, "/echo", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "echo"})

	resp, err := app.Test(httptest.NewRequest("GET", "/echo", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, calls, "middleware registered via OApiApp.Use must run")
}

func TestOApiGroupUse_PassesMiddlewareToFiber(t *testing.T) {
	// Same coverage for OApiGroup.Use — the group-scoped middleware should run
	// only for routes registered under that group.
	app := fiber.New()
	oapi := New(app)
	api := oapi.Group("/api")

	groupCalls := 0
	api.Use(func(c fiber.Ctx) error {
		groupCalls++
		return c.Next()
	})

	Get(api, "/inside", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "in"}, struct{}{}
	}, OpenAPIOptions{OperationID: "inside"})
	Get(oapi, "/outside", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "out"}, struct{}{}
	}, OpenAPIOptions{OperationID: "outside"})

	respIn, err := app.Test(httptest.NewRequest("GET", "/api/inside", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, respIn.StatusCode)
	assert.Equal(t, 1, groupCalls, "group middleware must run for /api/* routes")

	respOut, err := app.Test(httptest.NewRequest("GET", "/outside", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, respOut.StatusCode)
	assert.Equal(t, 1, groupCalls, "group middleware must NOT run for routes outside the group")
}

func TestOpenAPIYamlEndpoint_ServesYAMLSpec(t *testing.T) {
	// Exercise the YAML branch of the auto-registered docs routes.
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/things", func(c fiber.Ctx, _ struct{}) (covOut, struct{}) {
		return covOut{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "things"})

	resp, err := app.Test(httptest.NewRequest("GET", "/openapi.yaml", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "yaml")

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(body, &parsed), "served YAML must be parseable: %s", body)
	assert.Equal(t, "3.0.0", parsed["openapi"])
	paths, ok := parsed["paths"].(map[string]any)
	require.True(t, ok)
	_, hasThings := paths["/things"]
	assert.True(t, hasThings, "YAML spec should include registered routes")
}
