package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hiddenInput struct {
	ID string `uri:"id" validate:"required"`
}

type hiddenOutput struct {
	Message string `json:"message"`
}

// HiddenOnlyType is referenced ONLY by the hidden route in the leak test.
// We deliberately give it a distinctive name so we can grep for it in the
// generated spec.
type HiddenOnlyType struct {
	Secret string `json:"secret"`
}

func TestHidden_RouteServesTrafficButAbsentFromSpec(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Get(oapi, "/admin/debug/:id", func(c fiber.Ctx, in hiddenInput) (hiddenOutput, struct{}) {
		return hiddenOutput{Message: "debug " + in.ID}, struct{}{}
	}, OpenAPIOptions{
		OperationID: "internalDebug",
		Hidden:      true,
	})
	Get(oapi, "/public/:id", func(c fiber.Ctx, in hiddenInput) (hiddenOutput, struct{}) {
		return hiddenOutput{Message: "public " + in.ID}, struct{}{}
	}, OpenAPIOptions{OperationID: "public"})

	// Runtime: both routes serve traffic.
	respHidden, err := app.Test(httptest.NewRequest("GET", "/admin/debug/abc", nil))
	require.NoError(t, err)
	require.Equal(t, 200, respHidden.StatusCode)
	body, _ := io.ReadAll(respHidden.Body)
	var got hiddenOutput
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "debug abc", got.Message)

	respPublic, err := app.Test(httptest.NewRequest("GET", "/public/xyz", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, respPublic.StatusCode)

	// Spec: only the public path appears.
	spec := oapi.GenerateOpenAPISpec()
	paths := spec["paths"].(map[string]any)
	_, hasPublic := paths["/public/{id}"]
	_, hasHidden := paths["/admin/debug/{id}"]
	assert.True(t, hasPublic, "public route must appear in the spec")
	assert.False(t, hasHidden, "hidden route must NOT appear in the spec")
}

func TestHidden_TypesOnlyUsedByHiddenRouteDoNotLeak(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// A type that only the hidden route references — it must not surface
	// under components.schemas. Otherwise an attacker reading the spec could
	// guess the route's shape even without the path entry.
	Get(oapi, "/admin/secret", func(c fiber.Ctx, _ struct{}) (HiddenOnlyType, struct{}) {
		return HiddenOnlyType{Secret: "shh"}, struct{}{}
	}, OpenAPIOptions{
		OperationID: "adminSecret",
		Hidden:      true,
	})
	// Visible route using a distinct type, so we can confirm schema gen still works.
	Get(oapi, "/public", func(c fiber.Ctx, _ struct{}) (hiddenOutput, struct{}) {
		return hiddenOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "publicProbe"})

	spec := oapi.GenerateOpenAPISpec()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	_, hasHiddenType := schemas["HiddenOnlyType"]
	_, hasVisibleType := schemas["hiddenOutput"]
	assert.False(t, hasHiddenType, "type only used by a hidden route must not appear in components.schemas")
	assert.True(t, hasVisibleType, "type used by a visible route should still appear")
}

func TestHidden_AbsentOptionKeepsRouteInSpec(t *testing.T) {
	// Regression: by default (Hidden zero value = false), every route surfaces.
	app := fiber.New()
	oapi := New(app)

	Get(oapi, "/public/:id", func(c fiber.Ctx, in hiddenInput) (hiddenOutput, struct{}) {
		return hiddenOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "public"})

	spec := oapi.GenerateOpenAPISpec()
	paths := spec["paths"].(map[string]any)
	_, hasPublic := paths["/public/{id}"]
	assert.True(t, hasPublic, "route without Hidden must appear in the spec")
}

func TestHidden_TypeSharedBetweenHiddenAndVisibleStillSurfaces(t *testing.T) {
	// Edge case: when a type is used by BOTH a hidden and a visible route,
	// it must remain in components.schemas because the visible route still
	// needs to $ref it. The Hidden skip only suppresses contributions from
	// hidden routes — it does not retroactively remove a type that another
	// visible route depends on.
	app := fiber.New()
	oapi := New(app)

	Get(oapi, "/admin/secret/:id", func(c fiber.Ctx, in hiddenInput) (hiddenOutput, struct{}) {
		return hiddenOutput{Message: "secret " + in.ID}, struct{}{}
	}, OpenAPIOptions{OperationID: "secret", Hidden: true})

	Get(oapi, "/public/:id", func(c fiber.Ctx, in hiddenInput) (hiddenOutput, struct{}) {
		return hiddenOutput{Message: "public " + in.ID}, struct{}{}
	}, OpenAPIOptions{OperationID: "public"})

	spec := oapi.GenerateOpenAPISpec()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	_, hasShared := schemas["hiddenInput"]
	assert.True(t, hasShared, "type shared with a visible route must remain in components.schemas")
}
