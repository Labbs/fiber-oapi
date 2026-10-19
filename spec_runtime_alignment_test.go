package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http"
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

// TestAlign_ProblemDetails_RuntimeMatchesSpecExample checks, for each
// category a route documents, that the runtime Problem has the status, the
// Content-Type and the code the spec advertises.
func TestAlign_ProblemDetails_RuntimeMatchesSpecExample(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
		ProblemDetails:      true,
	})
	Post(oapi, "/users/:name", func(c fiber.Ctx, _ alignInput) (alignOutput, struct{}) {
		return alignOutput{Message: "ok"}, struct{}{}
	}, WithRoles(OpenAPIOptions{OperationID: "createUser"}, "admin"))
	oapi.UseNotFoundHandler()

	responses := oapi.GenerateOpenAPISpec()["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)["responses"].(map[string]any)
	advertised := func(code string) *Problem {
		content := responses[code].(map[string]any)["content"].(map[string]any)
		return content[MIMEApplicationProblemJSON].(map[string]any)["example"].(*Problem)
	}

	withToken := func(req *http.Request, token string) *http.Request {
		req.Header.Set("Authorization", "Bearer "+token)
		return req
	}
	cases := map[string]*http.Request{
		"400": withToken(jsonRequest("POST", "/users/alice", `{"age":"not a number"}`), "admin-token"),
		"401": jsonRequest("POST", "/users/alice", `{}`),
		"403": withToken(jsonRequest("POST", "/users/alice", `{}`), "valid-token"),
		"404": httptest.NewRequest("GET", "/missing", nil),
		"422": withToken(jsonRequest("POST", "/users/a", `{}`), "admin-token"),
	}
	for code, req := range cases {
		status, runtime := readProblem(t, app, req)
		spec := advertised(code)
		assert.Equal(t, code, statusCodeKey(status), code)
		assert.Equal(t, spec.Status, runtime.Status, code)
		assert.Equal(t, spec.Code, runtime.Code, code)
		assert.Equal(t, spec.Title, runtime.Title, code)
		assert.Equal(t, len(spec.Errors) > 0, len(runtime.Errors) > 0, "%s: per-field entries", code)
	}
	assert.Equal(t, advertised("400").Detail, func() string {
		_, p := readProblem(t, app, withToken(jsonRequest("POST", "/users/alice", `{"age":"not a number"}`), "admin-token"))
		return p.Detail
	}(), "the type-mismatch message format is the same in the spec and at runtime")
}

// TestAlign_V33Options_RuntimeMatchesSpec mirrors the setup of a sync server
// using every v3.3 option together and checks, route by route, that the
// status and the Content-Type returned at runtime are documented in the spec.
func TestAlign_V33Options_RuntimeMatchesSpec(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:     true,
		EnableAuthorization:  true,
		AuthService:          staticTokens,
		ProblemDetails:       true,
		ValidationStatusCode: 400,
		ProblemCodes:         map[string]string{errTypeValidation: "invalid_request"},
		SecuritySchemes:      map[string]SecurityScheme{"bearerAuth": {Type: "http", Scheme: "bearer"}},
		DefaultSecurity:      []map[string][]string{{"bearerAuth": {}}},
	})
	v1 := oapi.Group("/v1")
	Put(v1, "/blobs/:id", func(c fiber.Ctx, in putBlobInput) (*blobResult, struct{}) {
		return &blobResult{ID: in.ID, created: in.ID != "existing"}, struct{}{}
	}, OpenAPIOptions{
		SuccessStatus: 201,
		Responses:     map[string]any{"200": map[string]any{"description": "Same content already present"}},
		Middlewares: []fiber.Handler{func(c fiber.Ctx) error {
			if len(c.Body()) > 8 {
				return c.SendStatus(fiber.StatusRequestEntityTooLarge)
			}
			return c.Next()
		}},
	})
	Get(v1, "/blobs/:id", func(c fiber.Ctx, in blobInput) (*RawResponse, struct{}) {
		return &RawResponse{Body: []byte(in.ID)}, struct{}{}
	}, OpenAPIOptions{ResponseContentType: fiber.MIMEOctetStream})
	Post(v1, "/auth/logout", func(c fiber.Ctx, _ struct{}) (*struct{}, struct{}) {
		return nil, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 204})
	Post(v1, "/auth/login", func(c fiber.Ctx, in authFirstBody) (fiber.Map, struct{}) {
		return fiber.Map{"token": "user"}, struct{}{}
	}, OpenAPIOptions{Security: "disabled"})

	spec := oapi.GenerateOpenAPISpec()
	documented := func(path, method string, status int, contentType string) {
		t.Helper()
		operation := spec["paths"].(map[string]any)[path].(map[string]any)[method].(map[string]any)
		response, ok := operation["responses"].(map[string]any)[statusCodeKey(status)].(map[string]any)
		require.True(t, ok, "%s %s: status %d is not documented", method, path, status)
		if contentType == "" {
			assert.NotContains(t, response, "content", "%s %s %d", method, path, status)
			return
		}
		mediaType, _, _ := strings.Cut(contentType, ";")
		assert.Contains(t, response["content"], mediaType, "%s %s %d", method, path, status)
	}
	call := func(req *http.Request, token string, wantStatus int) string {
		t.Helper()
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, wantStatus, resp.StatusCode, "%s %s", req.Method, req.URL.Path)
		return resp.Header.Get("Content-Type")
	}
	blob := func(id, body string) *http.Request {
		req := httptest.NewRequest("PUT", "/v1/blobs/"+id, strings.NewReader(body))
		req.Header.Set("Content-Type", fiber.MIMEOctetStream)
		req.Header.Set("If-None-Match", "*")
		return req
	}

	documented("/v1/blobs/{id}", "put", 201, call(blob("new", "data"), "user", 201))
	documented("/v1/blobs/{id}", "put", 200, "") // declared through Responses, without content
	call(blob("existing", "data"), "user", 200)
	documented("/v1/blobs/{id}", "put", 401, call(blob("new", "data"), "", 401))
	documented("/v1/blobs/{id}", "put", 400, call(blob("new", ""), "user", 400))
	call(blob("new", "far too large"), "user", 413) // the route middleware answers
	documented("/v1/blobs/{id}", "get", 200, call(httptest.NewRequest("GET", "/v1/blobs/b-1", nil), "user", 200))
	documented("/v1/auth/logout", "post", 204, call(httptest.NewRequest("POST", "/v1/auth/logout", nil), "user", 204))
	documented("/v1/auth/login", "post", 200, call(jsonRequest("POST", "/v1/auth/login", `{"name":"alice"}`), "", 200))
	documented("/v1/auth/login", "post", 400, call(jsonRequest("POST", "/v1/auth/login", `{"name":"al"}`), "", 400))

	login := spec["paths"].(map[string]any)["/v1/auth/login"].(map[string]any)["post"].(map[string]any)["responses"].(map[string]any)
	assert.NotContains(t, login, "401", "a public route documents no authentication error")
}
