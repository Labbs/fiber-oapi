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

// AppError is the shared shape used across these tests, mirroring the pattern
// the example in _examples/simple_error demonstrates.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Details string `json:"details,omitempty"`
}

func (e *AppError) Error() string { return e.Message }

func appConflict(msg string) *AppError {
	return &AppError{Code: 409, Message: msg, Type: "Conflict", Details: "Duplicate resource"}
}

func appNotFound(msg string) *AppError {
	return &AppError{Code: 404, Message: msg, Type: "NotFound"}
}

func appForbidden(msg string) *AppError {
	return &AppError{Code: 403, Message: msg, Type: "Forbidden"}
}

type customErrInput struct {
	Name string `uri:"name" validate:"required,min=2"`
}

type customErrOutput struct {
	Message string `json:"message"`
}

func TestCustomErrors_SpecListsEachDeclaredStatus(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors: []any{
			appConflict("already exists"),
			appNotFound("missing"),
			appForbidden("not yours"),
		},
	})

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	for _, code := range []string{"403", "404", "409"} {
		r, ok := responses[code].(map[string]any)
		require.True(t, ok, "missing %s response", code)
		content := r["content"].(map[string]any)["application/json"].(map[string]any)
		require.NotNil(t, content["schema"], "%s missing schema", code)
		require.NotNil(t, content["example"], "%s missing example", code)
	}
}

func TestCustomErrors_SchemaIsDeduplicatedViaRef(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors: []any{
			appConflict("a"),
			appNotFound("b"),
			appForbidden("c"),
		},
	})

	spec := oapi.GenerateOpenAPISpec()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	_, hasAppError := schemas["AppError"]
	assert.True(t, hasAppError, "shared AppError schema should be in components.schemas")

	post := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)
	for _, code := range []string{"403", "404", "409"} {
		schema := responses[code].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		assert.Equal(t, "#/components/schemas/AppError", schema["$ref"], "all entries should $ref the shared schema")
	}
}

func TestCustomErrors_DescriptionFallback(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	type noMessage struct {
		Code int `json:"code"`
	}

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors: []any{
			&noMessage{Code: 418}, // no Message field — fallback to HTTP reason
			appConflict("custom msg"),
		},
	})

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	assert.Equal(t, "I'm a teapot", responses["418"].(map[string]any)["description"], "should fall back to HTTP reason phrase")
	assert.Equal(t, "custom msg", responses["409"].(map[string]any)["description"], "should use Message field")
}

type explicitDescriber struct {
	Code int `json:"code"`
}

func (e *explicitDescriber) Description() string { return "explicit override wins" }
func (e *explicitDescriber) HTTPStatus() int     { return 451 }

func TestCustomErrors_MethodsTakePriorityOverFields(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors:      []any{&explicitDescriber{Code: 999 /* ignored */}},
	})

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	_, has451 := responses["451"]
	assert.True(t, has451, "HTTPStatus() method should win over Code field")
	_, has999 := responses["999"]
	assert.False(t, has999, "the Code field should be ignored when HTTPStatus() is implemented")
	assert.Equal(t, "explicit override wins", responses["451"].(map[string]any)["description"])
}

func TestCustomErrors_HandlerReturnEmitsRightStatusAndBody(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		switch input.Name {
		case "dup":
			return customErrOutput{}, appConflict("already exists")
		case "missing":
			return customErrOutput{}, appNotFound("not found")
		}
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors:      []any{appConflict("a"), appNotFound("b")},
	})

	cases := []struct {
		path    string
		status  int
		bodyHas string
	}{
		{"/items/dup", 409, "already exists"},
		{"/items/missing", 404, "not found"},
		{"/items/alice", 200, "ok"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(""))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tc.status, resp.StatusCode)
			raw, _ := io.ReadAll(resp.Body)
			assert.Contains(t, string(raw), tc.bodyHas)
		})
	}
}

func TestCustomErrors_NilErrorReturnsSuccess(t *testing.T) {
	// Regression check for the isZero hardening: a nil `error` interface must
	// not panic — it must take the success branch.
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createItem"})

	resp, err := app.Test(httptest.NewRequest("POST", "/items/alice", strings.NewReader("")))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var out customErrOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "ok", out.Message)
}

// legacyTError is a non-empty struct used to exercise the TError catch-all
// behaviour in the next two tests.
type legacyTError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func TestCustomErrors_Suppresses4XXWhenErrorsDeclared(t *testing.T) {
	// When OpenAPIOptions.Errors is populated, the legacy 4XX catch-all is
	// redundant — the user has explicitly enumerated which status codes their
	// handler can emit. The spec should list ONLY those concrete codes.
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, *legacyTError) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors:      []any{appConflict("a"), appNotFound("b")},
	})

	spec := oapi.GenerateOpenAPISpec()
	responses := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)["responses"].(map[string]any)

	_, has4xx := responses["4XX"]
	assert.False(t, has4xx, "4XX must be suppressed when Errors[] is non-empty")

	// Sanity: the explicit codes are still there.
	_, has409 := responses["409"]
	_, has404 := responses["404"]
	assert.True(t, has409 && has404, "the explicit Errors entries must still surface")
}

func TestCustomErrors_4XXStillEmittedWhenErrorsSliceOnlyContainsNils(t *testing.T) {
	// Edge case: Errors: []any{nil} should be treated as "nothing declared",
	// not as "errors declared". The downstream emission loop skips nil entries,
	// so if we suppressed the 4XX based on slice length the route would end up
	// with zero documented error responses at all.
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, *legacyTError) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors:      []any{nil, nil},
	})

	spec := oapi.GenerateOpenAPISpec()
	responses := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)["responses"].(map[string]any)

	_, has4xx := responses["4XX"]
	assert.True(t, has4xx, "4XX must still be emitted when the Errors slice contains only nil entries")
}

func TestCustomErrors_4XXStillEmittedWhenNoErrorsDeclared(t *testing.T) {
	// Backwards compatibility: routes whose handler declares a non-empty TError
	// but provides no Errors[] entries still get the legacy 4XX catch-all so
	// existing integrations are not silently broken.
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, *legacyTError) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createItem"})

	spec := oapi.GenerateOpenAPISpec()
	responses := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)["responses"].(map[string]any)

	_, has4xx := responses["4XX"]
	assert.True(t, has4xx, "4XX must still be emitted when no Errors[] is declared (legacy behaviour)")
}

func TestCustomErrors_PrecedenceOverDefault404Envelope(t *testing.T) {
	// When the user declares a 404 in Errors AND has called UseNotFoundHandler(),
	// the declared shape (their AppError) wins for the per-route spec entry —
	// the not-found envelope is for routing misses, not for handler-emitted 404s.
	app := fiber.New()
	oapi := New(app)

	Post(oapi, "/items/:name", func(c fiber.Ctx, input customErrInput) (customErrOutput, error) {
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{
		OperationID: "createItem",
		Errors:      []any{appNotFound("not found")},
	})
	oapi.UseNotFoundHandler()

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]any)["/items/{name}"].(map[string]any)["post"].(map[string]any)
	resp404 := post["responses"].(map[string]any)["404"].(map[string]any)
	schema := resp404["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	assert.Equal(t, "#/components/schemas/AppError", schema["$ref"], "Errors entry should override the default ErrorEnvelope 404")
}
