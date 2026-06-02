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

// UniErr is the shape under test — a typical flat error response struct.
type UniErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Type    string `json:"type"`
}

func (e *UniErr) Error() string { return e.Message }

type uniInput struct {
	Name string `uri:"name" validate:"required,min=2"`
	Age  int    `json:"age" validate:"omitempty,min=18"`
}

type uniOutput struct {
	Message string `json:"message"`
}

func registerUniRoute(t *testing.T) (*fiber.App, *OApiApp) {
	t.Helper()
	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: &UniErr{}})
	Post(oapi, "/users/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createUser"})
	oapi.UseNotFoundHandler()
	return app, oapi
}

func TestDefaultErrorShape_ValidationKeepsEnvelope(t *testing.T) {
	// 422 validation errors stay on the rich ErrorEnvelope shape even when
	// DefaultErrorShape is set — collapsing per-field info into a flat struct
	// would lose loc / constraint / field needed by form-level client UX.
	app, _ := registerUniRoute(t)

	resp, err := app.Test(httptest.NewRequest("POST", "/users/a", strings.NewReader("")))
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env), "validation must stay on envelope: %s", raw)
	require.GreaterOrEqual(t, len(env.Errors), 1)
	assert.Equal(t, "validation_error", env.Errors[0].Type)
	assert.Equal(t, 422, env.Errors[0].Code)
	assert.NotEmpty(t, env.Errors[0].Loc)
	assert.NotEmpty(t, env.Errors[0].Constraint)
}

func TestDefaultErrorShape_ParseEmitsUniShape(t *testing.T) {
	app, _ := registerUniRoute(t)

	// Send malformed JSON to trigger a parse error.
	req := httptest.NewRequest("POST", "/users/alice", strings.NewReader(`{"age": "not a number"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var got UniErr
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, 400, got.Code)
	assert.Equal(t, "type_error", got.Type)
	assert.Contains(t, got.Message, "expected int but got string")
}

func TestDefaultErrorShape_NotFoundEmitsUniShape(t *testing.T) {
	app, _ := registerUniRoute(t)

	resp, err := app.Test(httptest.NewRequest("GET", "/missing-route", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var got UniErr
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, 404, got.Code)
	assert.Equal(t, "not_found", got.Type)
	assert.Contains(t, got.Message, "GET /missing-route")
}

func TestDefaultErrorShape_MethodNotAllowedEmitsUniShape(t *testing.T) {
	app, _ := registerUniRoute(t)

	// GET on the POST-only route → 405 with Allow header
	resp, err := app.Test(httptest.NewRequest("GET", "/users/alice", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	assert.Equal(t, "POST", resp.Header.Get("Allow"))

	raw, _ := io.ReadAll(resp.Body)
	var got UniErr
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, 405, got.Code)
	assert.Equal(t, "method_not_allowed", got.Type)
	assert.Contains(t, got.Message, "method GET not allowed")
	assert.Equal(t, "POST", got.Details)
}

func TestDefaultErrorShape_SpecMixesShapeAndEnvelope(t *testing.T) {
	// With DefaultErrorShape set, spec entries for 400 / 404 (and 401 / 403 /
	// 405 when applicable) reference the user's flat shape. 422 keeps the
	// ErrorEnvelope schema so per-field validation info stays documented.
	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: &UniErr{}})
	Post(oapi, "/users/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createUser"})
	oapi.UseNotFoundHandler()

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	// Flat shape for 400 and 404
	for _, code := range []string{"400", "404"} {
		schema := responses[code].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		assert.Equal(t, "#/components/schemas/UniErr", schema["$ref"], "%s should use the flat user shape", code)
	}
	// Envelope kept for 422
	schema422 := responses["422"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	assert.Equal(t, "#/components/schemas/ErrorEnvelope", schema422["$ref"], "422 must keep the envelope schema for per-field info")
}

func TestDefaultErrorShape_NilShapeKeepsEnvelopeBehaviour(t *testing.T) {
	// Sanity check: no regression — when DefaultErrorShape is nil, envelopes
	// are still emitted (existing behaviour).
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/users/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createUser"})

	resp, err := app.Test(httptest.NewRequest("POST", "/users/a", strings.NewReader("")))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(raw), `"errors":`, "envelope shape must still be emitted when DefaultErrorShape is nil")
}

func TestDefaultErrorShape_PerRouteErrorsStillOverride(t *testing.T) {
	// Even with DefaultErrorShape set, an entry in OpenAPIOptions.Errors should
	// still override the default for its status code in the spec.
	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: &UniErr{}})
	type ExplicitConflict struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Hint    string `json:"hint"`
	}
	Post(oapi, "/users/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createUser",
		Errors:      []any{&ExplicitConflict{Code: 409, Message: "taken", Hint: "try another"}},
	})

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	schema := responses["409"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	assert.Equal(t, "#/components/schemas/ExplicitConflict", schema["$ref"], "per-route Errors should still override")

	// Default 422 stays on ErrorEnvelope (the validation carve-out applies even
	// when DefaultErrorShape is configured).
	schema422 := responses["422"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	assert.Equal(t, "#/components/schemas/ErrorEnvelope", schema422["$ref"])
}

func TestDefaultErrorShape_SchemaAlwaysRegistered(t *testing.T) {
	// Regression: when DefaultErrorShape is set, the type must appear in
	// components.schemas even if no operation declares an OpenAPIOptions.Errors
	// entry that would otherwise have collected it — otherwise the $ref the
	// spec emits for 400 / 404 / 405 / auth points at a missing component.
	type SoloShape struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	}

	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: &SoloShape{}})
	Post(oapi, "/u/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createUser"})
	oapi.UseNotFoundHandler()

	spec := oapi.GenerateOpenAPISpec()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	_, has := schemas["SoloShape"]
	assert.True(t, has, "DefaultErrorShape's type must be registered in components.schemas; otherwise the 400/404 $refs dangle")

	// Cross-check that the dangling reference would have actually triggered:
	// confirm at least one response references it.
	post := spec["paths"].(map[string]any)["/u/{name}"].(map[string]any)["post"].(map[string]any)
	r400 := post["responses"].(map[string]any)["400"].(map[string]any)
	schema := r400["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	assert.Equal(t, "#/components/schemas/SoloShape", schema["$ref"])
}

func TestDefaultErrorShape_ValueShape(t *testing.T) {
	// Passing a non-pointer struct as the template should also work. We exercise
	// the path-not-found case (404) since 422 deliberately keeps the envelope.
	type ValueShape struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: ValueShape{}})
	Post(oapi, "/users/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createUser"})
	oapi.UseNotFoundHandler()

	resp, err := app.Test(httptest.NewRequest("GET", "/no-such-route", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var got ValueShape
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, 404, got.Code)
	assert.Equal(t, "not_found", got.Type)
}
