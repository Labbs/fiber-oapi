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

type envelopeInput struct {
	ID          string `uri:"id" validate:"required,len=5"`
	Filter      string `query:"filter" validate:"omitempty,min=3"`
	RequestID   string `header:"X-Request-Id" validate:"omitempty,uuid4"`
	WorkspaceID string `json:"workspaceId" validate:"required,min=11"`
	Email       string `json:"email" validate:"required,email"`
	Nested      struct {
		Slug string `json:"slug" validate:"required,min=2"`
	} `json:"nested" validate:"required"`
}

type envelopeOutput struct {
	Message string `json:"message"`
}

func registerEnvelopeRoute(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/items/:id", func(c fiber.Ctx, input envelopeInput) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createItem"})
	return app
}

func TestEnvelope_MultiFieldValidation(t *testing.T) {
	app := registerEnvelopeRoute(t)

	// All three of: WorkspaceID too short, Email malformed, Nested.Slug too short.
	body := `{"workspaceId":"short","email":"not-an-email","nested":{"slug":"a"}}`
	req := httptest.NewRequest("POST", "/items/abcde", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))

	// One entry per failing field.
	require.GreaterOrEqual(t, len(env.Errors), 3, "expected at least 3 entries, got: %s", raw)
	seenFields := map[string]ValidationErrorEntry{}
	for _, e := range env.Errors {
		assert.Equal(t, errTypeValidation, e.Type)
		assert.Equal(t, 422, e.Code)
		assert.NotEmpty(t, e.Loc)
		assert.NotEmpty(t, e.Msg)
		assert.NotEmpty(t, e.Constraint)
		seenFields[e.Field] = e
	}

	if entry, ok := seenFields["workspaceId"]; ok {
		assert.Equal(t, []any{"body", "workspaceId"}, entry.Loc)
		assert.Equal(t, "min=11", entry.Constraint)
	} else {
		t.Errorf("missing workspaceId entry: %s", raw)
	}
	if entry, ok := seenFields["email"]; ok {
		assert.Equal(t, "email", entry.Constraint)
	} else {
		t.Errorf("missing email entry: %s", raw)
	}
	if entry, ok := seenFields["slug"]; ok {
		assert.Equal(t, []any{"body", "nested", "slug"}, entry.Loc, "nested loc should walk through JSON tags")
	} else {
		t.Errorf("missing nested slug entry: %s", raw)
	}
}

func TestEnvelope_TypeMismatch_NestedLocPreserved(t *testing.T) {
	// Regression: when the failing field is inside a nested struct, the loc
	// array must carry the full JSON path — not just ["body", "<leaf>"]. The
	// previous implementation routed ute.Field through the Go-name resolver,
	// which couldn't match JSON tag names and silently dropped intermediate
	// segments.
	type Address struct {
		Zipcode string `json:"zipcode" validate:"required"`
	}
	type Person struct {
		Name string  `json:"name" validate:"required"`
		Addr Address `json:"address" validate:"required"`
	}

	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/persons", func(c fiber.Ctx, in Person) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createPerson"})

	// zipcode expects a string but receives a number → UnmarshalTypeError on
	// the nested path "address.zipcode".
	body := `{"name":"Alice","address":{"zipcode":12345}}`
	req := httptest.NewRequest("POST", "/persons", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Len(t, env.Errors, 1)
	entry := env.Errors[0]
	assert.Equal(t, errTypeTypeMismatch, entry.Type)
	assert.Equal(t, "zipcode", entry.Field)
	// The whole JSON path must be preserved — body → address → zipcode.
	assert.Equal(t, []any{"body", "address", "zipcode"}, entry.Loc, "nested JSON path segments must be preserved in loc")
}

func TestEnvelope_TypeMismatch(t *testing.T) {
	app := registerEnvelopeRoute(t)

	// workspaceId expects a string but receives a number.
	body := `{"workspaceId":123,"email":"a@b.co","nested":{"slug":"ab"}}`
	req := httptest.NewRequest("POST", "/items/abcde", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Len(t, env.Errors, 1)
	entry := env.Errors[0]
	assert.Equal(t, errTypeTypeMismatch, entry.Type)
	assert.Equal(t, 400, entry.Code)
	assert.Equal(t, "workspaceId", entry.Field)
	assert.Contains(t, entry.Msg, "expected string but got number")
}

func TestEnvelope_ReadsXRequestID(t *testing.T) {
	app := registerEnvelopeRoute(t)

	body := `{"workspaceId":"short","email":"a@b.co","nested":{"slug":"ok"}}`
	req := httptest.NewRequest("POST", "/items/abcde", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "bf0e9029-576b-42e8-84f9-ad0622972f50")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.Equal(t, "bf0e9029-576b-42e8-84f9-ad0622972f50", env.ResponseContext.ResponseID)

	// No header → empty response_id.
	req2 := httptest.NewRequest("POST", "/items/abcde", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	raw2, _ := io.ReadAll(resp2.Body)
	var env2 ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw2, &env2))
	assert.Empty(t, env2.ResponseContext.ResponseID)
}

func TestEnvelope_IncludeInvalidValueOptIn(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{IncludeInvalidValueInErrors: true})
	Post(oapi, "/items/:id", func(c fiber.Ctx, input envelopeInput) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createItem"})

	body := `{"workspaceId":"short","email":"a@b.co","nested":{"slug":"ok"}}`
	req := httptest.NewRequest("POST", "/items/abcde", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	var found bool
	for _, e := range env.Errors {
		if e.Field == "workspaceId" {
			assert.Equal(t, "short", e.Value)
			found = true
		}
	}
	assert.True(t, found, "workspaceId entry not found: %s", raw)
}

func TestEnvelope_NotFound_DefaultEnvelope(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/exists", func(c fiber.Ctx, input struct{}) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "exists"})
	oapi.UseNotFoundHandler()

	// Sanity: registered route still works (catch-all must not swallow it)
	okResp, err := app.Test(httptest.NewRequest("GET", "/exists", nil))
	require.NoError(t, err)
	require.Equal(t, 200, okResp.StatusCode)

	// Unmatched route returns the envelope
	req := httptest.NewRequest("GET", "/does-not-exist", nil)
	req.Header.Set("X-Request-Id", "rid-not-found")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Len(t, env.Errors, 1)
	entry := env.Errors[0]
	assert.Equal(t, errTypeNotFound, entry.Type)
	assert.Equal(t, 404, entry.Code)
	assert.Equal(t, []any{"path"}, entry.Loc)
	assert.Equal(t, "/does-not-exist", entry.Field)
	assert.Contains(t, entry.Msg, "GET /does-not-exist")
	assert.Equal(t, "rid-not-found", env.ResponseContext.ResponseID)
}

func TestEnvelope_NotFound_CustomHandlerWins(t *testing.T) {
	called := false
	app := fiber.New()
	oapi := New(app, Config{NotFoundHandler: func(c fiber.Ctx) error {
		called = true
		return c.Status(404).JSON(fiber.Map{"custom": true, "path": c.Path()})
	}})
	oapi.UseNotFoundHandler()

	resp, err := app.Test(httptest.NewRequest("GET", "/nope", nil))
	require.NoError(t, err)
	assert.True(t, called, "custom NotFoundHandler should run")
	assert.Equal(t, 404, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(raw), `"custom":true`)
}

func TestEnvelope_NotFound_NotRegisteredWithoutCall(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/exists", func(c fiber.Ctx, input struct{}) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "exists"})
	// Deliberately NOT calling UseNotFoundHandler().

	resp, err := app.Test(httptest.NewRequest("GET", "/does-not-exist", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)

	// Body should be Fiber's default (plain "Cannot GET ..."), not our envelope.
	raw, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(raw), `"errors":`, "should not produce an envelope when UseNotFoundHandler is not called")
}

func TestEnvelope_NotFound_DoesNotInterfereWithRegisteredRoutes(t *testing.T) {
	// Routes registered BEFORE UseNotFoundHandler() keep working. The catch-all is
	// a Use middleware in Fiber, matched in registration order, so it must be the
	// last middleware installed (after every route). This is documented in the
	// method comment; the test pins the invariant.
	app := fiber.New()
	oapi := New(app)

	Get(oapi, "/before", func(c fiber.Ctx, input struct{}) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "before"}, struct{}{}
	}, OpenAPIOptions{OperationID: "before"})

	type V struct {
		Name string `query:"name" validate:"required,min=3"`
	}
	Get(oapi, "/validated", func(c fiber.Ctx, input V) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "validated"})

	// Catch-all installed LAST.
	oapi.UseNotFoundHandler()

	// Registered route — 200 OK
	respBefore, err := app.Test(httptest.NewRequest("GET", "/before", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, respBefore.StatusCode)

	// Validation errors on a registered route surface a 422 envelope (not 404).
	respV, err := app.Test(httptest.NewRequest("GET", "/validated?name=a", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, respV.StatusCode)
	rawV, _ := io.ReadAll(respV.Body)
	assert.Contains(t, string(rawV), `"validation_error"`)

	// Unmatched route — 404 envelope
	respNF, err := app.Test(httptest.NewRequest("GET", "/missing", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, respNF.StatusCode)
}

func TestEnvelope_NotFound_TopLevelHelper(t *testing.T) {
	// fiberoapi.DefaultNotFoundHandler() returns a fiber.Handler that emits
	// the envelope — useful for users who manage their own fiber.Config.
	app := fiber.New()
	app.Use(DefaultNotFoundHandler())

	resp, err := app.Test(httptest.NewRequest("DELETE", "/missing", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.Equal(t, errTypeNotFound, env.Errors[0].Type)
	assert.Contains(t, env.Errors[0].Msg, "DELETE /missing")
}

func TestEnvelope_NotFound_HEADRequest_NoBody(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/exists", func(c fiber.Ctx, input struct{}) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "exists"})
	oapi.UseNotFoundHandler()

	resp, err := app.Test(httptest.NewRequest("HEAD", "/missing", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	assert.Empty(t, raw, "HEAD response must not carry a body")
}

func TestEnvelope_NotFound_405WithAllowHeader(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/items/:id", func(c fiber.Ctx, input envelopeInput) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createItem"})
	Put(oapi, "/items/:id", func(c fiber.Ctx, input envelopeInput) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "updateItem"})
	oapi.UseNotFoundHandler()

	// GET on a POST+PUT-only path → 405 with Allow header
	resp, err := app.Test(httptest.NewRequest("GET", "/items/abc", nil))
	require.NoError(t, err)
	assert.Equal(t, 405, resp.StatusCode)
	allow := resp.Header.Get("Allow")
	assert.Contains(t, allow, "POST")
	assert.Contains(t, allow, "PUT")

	raw, _ := io.ReadAll(resp.Body)
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	assert.Equal(t, "method_not_allowed", env.Errors[0].Type)
	assert.Equal(t, 405, env.Errors[0].Code)
	assert.Equal(t, []any{"method"}, env.Errors[0].Loc)
}

func TestEnvelope_NotFound_OptionsFallthrough(t *testing.T) {
	// OPTIONS preflights must be passed downstream so CORS-like middleware can
	// respond. With nothing else installed, Fiber's own stack returns 404.
	corsHandled := false
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/exists", func(c fiber.Ctx, input struct{}) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "exists"})
	// Install our catch-all FIRST so we can verify it does not eat OPTIONS.
	oapi.UseNotFoundHandler()
	// Then a CORS-like fallback registered after via app.Use — it should run
	// when the catch-all calls c.Next() on OPTIONS.
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == "OPTIONS" {
			corsHandled = true
			return c.Status(204).Send(nil)
		}
		return c.Next()
	})

	resp, err := app.Test(httptest.NewRequest("OPTIONS", "/whatever", nil))
	require.NoError(t, err)
	assert.True(t, corsHandled, "OPTIONS must reach downstream middleware")
	assert.Equal(t, 204, resp.StatusCode)
}

func TestEnvelope_NotFound_IdempotentInstall(t *testing.T) {
	// Calling UseNotFoundHandler() twice on the same OApiApp should be a no-op
	// on the second call (no double-stacking of middleware).
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/exists", func(c fiber.Ctx, input struct{}) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "exists"})

	oapi.UseNotFoundHandler()
	oapi.UseNotFoundHandler() // no-op
	oapi.UseNotFoundHandler() // no-op

	resp, err := app.Test(httptest.NewRequest("GET", "/missing", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	// Sanity: registered route still reachable.
	respOK, err := app.Test(httptest.NewRequest("GET", "/exists", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, respOK.StatusCode)
}

func TestEnvelope_NotFound_RequestIDSanitization(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	oapi.UseNotFoundHandler()

	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"valid UUID is echoed", "bf0e9029-576b-42e8-84f9-ad0622972f50", "bf0e9029-576b-42e8-84f9-ad0622972f50"},
		// fasthttp rejects header values with raw CRLF / NUL at parse time, so
		// the sanitizer only needs to cover characters fasthttp lets through.
		{"semicolons dropped", "abc;DROP TABLE", ""},
		{"too long dropped", strings.Repeat("a", 200), ""},
		{"spaces dropped", "abc def", ""},
		{"valid hex echoed", "0123abcdef", "0123abcdef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/missing", nil)
			if tc.header != "" {
				req.Header.Set("X-Request-Id", tc.header)
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			raw, _ := io.ReadAll(resp.Body)
			var env ErrorEnvelope
			require.NoError(t, json.Unmarshal(raw, &env))
			assert.Equal(t, tc.want, env.ResponseContext.ResponseID)
		})
	}
}

func TestEnvelope_NotFound_PathTruncation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	oapi.UseNotFoundHandler()

	// Construct a very long path (> 1024 bytes). fasthttp itself imposes a path
	// length limit so this also exercises that an over-long URL is gracefully
	// handled rather than crashing.
	longPath := "/" + strings.Repeat("a", 2000)
	resp, err := app.Test(httptest.NewRequest("GET", longPath, nil))
	require.NoError(t, err)
	// Status may be 404 (envelope) or 414 / 400 depending on fasthttp limits.
	// What matters here is that no panic occurs and any envelope returned has a
	// bounded msg/field length.
	if resp.StatusCode == 404 {
		raw, _ := io.ReadAll(resp.Body)
		var env ErrorEnvelope
		if json.Unmarshal(raw, &env) == nil {
			// 1024 bytes + the ellipsis "…" (3 bytes UTF-8) is the upper bound.
			assert.LessOrEqual(t, len(env.Errors[0].Field), 1024+3+8)
		}
	}
}

func TestEnvelope_NotFound_SpecEntryGatedOnInstall(t *testing.T) {
	// When UseNotFoundHandler() is NOT called, the generated spec should not
	// include a 404 response (because routing falls through to Fiber's default,
	// which is not an envelope).
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/items/:id", func(c fiber.Ctx, input envelopeInput) (envelopeOutput, struct{}) {
		return envelopeOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{OperationID: "createItem"})

	spec := oapi.GenerateOpenAPISpec()
	post := spec["paths"].(map[string]interface{})["/items/{id}"].(map[string]interface{})["post"].(map[string]interface{})
	_, has404 := post["responses"].(map[string]interface{})["404"]
	assert.False(t, has404, "404 must not appear in the spec without UseNotFoundHandler()")

	// After installing the handler, the entry appears.
	oapi.UseNotFoundHandler()
	spec = oapi.GenerateOpenAPISpec()
	post = spec["paths"].(map[string]interface{})["/items/{id}"].(map[string]interface{})["post"].(map[string]interface{})
	_, has404 = post["responses"].(map[string]interface{})["404"]
	assert.True(t, has404, "404 should appear in the spec after UseNotFoundHandler()")
}

func TestEnvelope_OpenAPISpecExposesExamples(t *testing.T) {
	app := registerEnvelopeRoute(t)

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(raw, &spec))

	post := spec["paths"].(map[string]any)["/items/{id}"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)

	// 422 — validation envelope with example
	resp422 := responses["422"].(map[string]any)
	assert.Equal(t, "Validation error", resp422["description"])
	content := resp422["content"].(map[string]any)["application/json"].(map[string]any)
	assert.Equal(t, "#/components/schemas/ErrorEnvelope", content["schema"].(map[string]any)["$ref"])
	require.NotNil(t, content["example"], "expected an example payload on the 422 response")
	example := content["example"].(map[string]any)
	require.NotNil(t, example["errors"])
	require.NotNil(t, example["response_context"])

	// 400 — parse / type-mismatch envelope (only for POST/PUT/PATCH). The
	// description was widened to cover both malformed JSON and wrong-typed
	// fields after the spec example was aligned with the runtime emission.
	resp400 := responses["400"].(map[string]any)
	assert.Equal(t, "Invalid request body (malformed JSON or wrong field type)", resp400["description"])

	// Components include the envelope schema
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	_, hasEnvelope := schemas["ErrorEnvelope"]
	assert.True(t, hasEnvelope, "ErrorEnvelope should be registered in components.schemas")
	_, hasEntry := schemas["ValidationErrorEntry"]
	assert.True(t, hasEntry, "ValidationErrorEntry should be registered in components.schemas")
}
