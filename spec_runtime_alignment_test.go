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

// These tests pin the spec examples and the runtime responses together so
// they cannot drift apart silently. Each one fires a request that triggers the
// matching error category, captures the runtime envelope, and compares the
// stable identifiers (Type, Code, Constraint pattern, Loc shape) against what
// the spec advertises for that response. We deliberately do NOT compare the
// human Msg string verbatim — that's checked separately for the case it
// matters most (type mismatch) below.

type alignInput struct {
	Name string `uri:"name" validate:"required,min=2"`
	Age  int    `json:"age" validate:"omitempty,min=18"`
}

type alignOutput struct {
	Message string `json:"message"`
}

func registerAlignRoute(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/users/:name", func(c fiber.Ctx, _ alignInput) (alignOutput, struct{}) {
		return alignOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createUser"})
	oapi.UseNotFoundHandler()
	return app
}

// TestAlign_400_RuntimeMatchesSpecExample reproduces the exact scenario the
// spec example describes (a body with a wrong-typed `age` field) and asserts
// the runtime payload's Type and Msg match exampleParseEnvelope() — i.e. the
// values clients would parse from the OpenAPI document. This is the test that
// would have caught the recent drift between exampleParseEnvelope.Msg and the
// runtime format string.
func TestAlign_400_RuntimeMatchesSpecExample(t *testing.T) {
	app := registerAlignRoute(t)

	body := `{"age":"not a number"}`
	req := httptest.NewRequest("POST", "/users/alice", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var runtime ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &runtime))
	require.Len(t, runtime.Errors, 1)
	runtimeEntry := runtime.Errors[0]

	spec := exampleParseEnvelope()
	require.Len(t, spec.Errors, 1)
	specEntry := spec.Errors[0]

	assert.Equal(t, specEntry.Type, runtimeEntry.Type, "Type discriminator must match what the spec advertises")
	assert.Equal(t, specEntry.Code, runtimeEntry.Code, "Status code in entry must match the spec")
	// Msg is built from the same format constant on both sides, so the prose
	// must match exactly (modulo the per-request field name / type names).
	assert.Equal(t, specEntry.Msg, runtimeEntry.Msg, "Msg format must match between spec example and runtime")
	// Loc shape: both start with "body" then carry the JSON field name.
	require.GreaterOrEqual(t, len(runtimeEntry.Loc), 2)
	assert.Equal(t, "body", runtimeEntry.Loc[0], "first loc segment must be 'body' for body-derived type errors")
	assert.Equal(t, "body", specEntry.Loc[0], "spec example must place 'body' first")
}

// TestAlign_DefaultErrorShape_400_TypeAndDetailsMatchRuntime exercises the
// DefaultErrorShape branch of the 400 emission and confirms the spec example
// uses the same Type ("type_error") and Details ("int") the runtime computes
// via categorizeError for the same scenario.
func TestAlign_DefaultErrorShape_400_TypeAndDetailsMatchRuntime(t *testing.T) {
	type Shape struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
		Details string `json:"details,omitempty"`
	}

	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: &Shape{}})
	Post(oapi, "/users/:name", func(c fiber.Ctx, _ alignInput) (alignOutput, struct{}) {
		return alignOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createUser"})

	// 1) What the runtime emits for a *json.UnmarshalTypeError
	req := httptest.NewRequest("POST", "/users/alice", strings.NewReader(`{"age":"oops"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var runtime Shape
	require.NoError(t, json.Unmarshal(raw, &runtime))

	// 2) What the spec advertises for the same 400 response
	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)
	content := post["responses"].(map[string]any)["400"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	specExampleJSON, _ := json.Marshal(content["example"])
	var advertised Shape
	require.NoError(t, json.Unmarshal(specExampleJSON, &advertised))

	// 3) Pin the stable identifiers: Type and Details. (Msg uses different
	// field names — runtime says "age", advertised example also says "age" —
	// so it should match exactly, too.)
	assert.Equal(t, advertised.Type, runtime.Type, "Type discriminator in spec example must match runtime emission")
	assert.Equal(t, advertised.Details, runtime.Details, "Details (Go type name) must match between spec and runtime")
	assert.Equal(t, advertised.Code, runtime.Code, "Code must match between spec and runtime")
	assert.Equal(t, advertised.Message, runtime.Message, "Message format must match between spec and runtime")
	// Sanity: the discriminator is the type-mismatch one, not a generic parse_error.
	assert.Equal(t, errTypeTypeMismatch, advertised.Type)
}

// TestAlign_DefaultErrorShape_404_TypeMatchesRuntime keeps the 404 example
// and runtime in sync the same way.
func TestAlign_DefaultErrorShape_404_TypeMatchesRuntime(t *testing.T) {
	type Shape struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	app := fiber.New()
	oapi := New(app, Config{DefaultErrorShape: &Shape{}})
	Get(oapi, "/exists", func(c fiber.Ctx, _ struct{}) (alignOutput, struct{}) {
		return alignOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "exists"})
	oapi.UseNotFoundHandler()

	// Runtime
	resp, err := app.Test(httptest.NewRequest("GET", "/missing", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var runtime Shape
	require.NoError(t, json.Unmarshal(raw, &runtime))

	// Spec example
	spec := oapi.GenerateOpenAPISpec()
	get := spec["paths"].(map[string]any)["/exists"].(map[string]any)["get"].(map[string]any)
	content := get["responses"].(map[string]any)["404"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	specJSON, _ := json.Marshal(content["example"])
	var advertised Shape
	require.NoError(t, json.Unmarshal(specJSON, &advertised))

	assert.Equal(t, errTypeNotFound, advertised.Type)
	assert.Equal(t, advertised.Type, runtime.Type, "404 Type must match between spec example and runtime")
	assert.Equal(t, advertised.Code, runtime.Code)
}
