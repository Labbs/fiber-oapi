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

type problemInput struct {
	ID   string `uri:"id"`
	Name string `json:"name" validate:"required,min=3"`
	Age  int    `json:"age"`
}

// readProblem sends req and decodes the Problem it gets back, checking the
// RFC 9457 Content-Type on the way.
func readProblem(t *testing.T, app *fiber.App, req *http.Request) (int, Problem) {
	t.Helper()
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, MIMEApplicationProblemJSON, resp.Header.Get("Content-Type"), "body: %s", raw)
	var p Problem
	require.NoError(t, json.Unmarshal(raw, &p), "body: %s", raw)
	return resp.StatusCode, p
}

func jsonRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newProblemApp(t *testing.T, cfg Config) (*fiber.App, *OApiApp) {
	t.Helper()
	app := fiber.New()
	oapi := New(app, cfg)
	Post(oapi, "/items/:id", func(c fiber.Ctx, in problemInput) (fiber.Map, struct{}) {
		return fiber.Map{"id": in.ID}, struct{}{}
	}, WithRoles(OpenAPIOptions{}, "admin"))
	Get(oapi, "/broken", func(c fiber.Ctx, _ struct{}) (UnserializableOutput, struct{}) {
		return UnserializableOutput{Channel: make(chan string)}, struct{}{}
	}, OpenAPIOptions{Security: "disabled"})
	oapi.UseNotFoundHandler()
	return app, oapi
}

func problemConfig() Config {
	return Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
		ProblemDetails:      true,
	}
}

func TestProblemDetails_EveryLibraryErrorIsAProblem(t *testing.T) {
	app, _ := newProblemApp(t, problemConfig())
	authed := func(req *http.Request, token string) *http.Request {
		req.Header.Set("Authorization", "Bearer "+token)
		return req
	}

	status, p := readProblem(t, app, authed(jsonRequest("POST", "/items/1", `{"name": "x"}`), "admin-token"))
	assert.Equal(t, 422, status)
	assert.Equal(t, Problem{Title: "Unprocessable Entity", Status: 422, Code: errTypeValidation}, Problem{Title: p.Title, Status: p.Status, Code: p.Code})
	require.Len(t, p.Errors, 1)
	assert.Equal(t, []any{"body", "name"}, p.Errors[0].Loc)
	assert.Equal(t, "min=3", p.Errors[0].Constraint)
	assert.NotEmpty(t, p.Detail)

	status, p = readProblem(t, app, authed(jsonRequest("POST", "/items/1", `{"name": "widget", "age": "old"}`), "admin-token"))
	assert.Equal(t, 400, status)
	assert.Equal(t, errTypeTypeMismatch, p.Code)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "age", p.Errors[0].Field)

	status, p = readProblem(t, app, authed(jsonRequest("POST", "/items/1", `{"name": `), "admin-token"))
	assert.Equal(t, 400, status)
	assert.Equal(t, errTypeParse, p.Code)
	assert.Empty(t, p.Errors)

	status, p = readProblem(t, app, jsonRequest("POST", "/items/1", `{"name": "widget"}`))
	assert.Equal(t, 401, status)
	assert.Equal(t, errTypeAuthN, p.Code)
	assert.Equal(t, "Unauthorized", p.Title)

	status, p = readProblem(t, app, authed(jsonRequest("POST", "/items/1", `{"name": "widget"}`), "valid-token"))
	assert.Equal(t, 403, status)
	assert.Equal(t, errTypeAuthZ, p.Code)

	status, p = readProblem(t, app, httptest.NewRequest("GET", "/nowhere", nil))
	assert.Equal(t, 404, status)
	assert.Equal(t, errTypeNotFound, p.Code)

	status, p = readProblem(t, app, httptest.NewRequest("DELETE", "/items/1", nil))
	assert.Equal(t, 405, status)
	assert.Equal(t, errTypeMethodNotAllowed, p.Code)

	status, p = readProblem(t, app, httptest.NewRequest("GET", "/broken", nil))
	assert.Equal(t, 500, status)
	assert.Equal(t, errTypeSerialization, p.Code)
}

func TestProblemDetails_ProblemCodesAndValidationStatus(t *testing.T) {
	cfg := problemConfig()
	cfg.ValidationStatusCode = 400
	cfg.ProblemCodes = map[string]string{
		errTypeValidation:   "invalid_request",
		errTypeTypeMismatch: "invalid_request",
		errTypeAuthN:        "unauthenticated",
	}
	app, _ := newProblemApp(t, cfg)

	req := jsonRequest("POST", "/items/1", `{"name": "x"}`)
	req.Header.Set("Authorization", "Bearer admin-token")
	status, p := readProblem(t, app, req)
	assert.Equal(t, 400, status)
	assert.Equal(t, 400, p.Status)
	assert.Equal(t, "invalid_request", p.Code)
	assert.Equal(t, 400, p.Errors[0].Code, "entries carry the configured status too")

	status, p = readProblem(t, app, jsonRequest("POST", "/items/1", `{"name": "widget"}`))
	assert.Equal(t, 401, status)
	assert.Equal(t, "unauthenticated", p.Code)

	status, p = readProblem(t, app, httptest.NewRequest("GET", "/nowhere", nil))
	assert.Equal(t, 404, status)
	assert.Equal(t, errTypeNotFound, p.Code, "unmapped categories keep their default code")
}

func TestValidationStatusCode_RuntimeMatchesSpec(t *testing.T) {
	for _, problems := range []bool{false, true} {
		app := fiber.New()
		oapi := New(app, Config{EnableValidation: true, ProblemDetails: problems, ValidationStatusCode: 400})
		Post(oapi, "/items/:id", func(c fiber.Ctx, in problemInput) (fiber.Map, struct{}) {
			return fiber.Map{"id": in.ID}, struct{}{}
		}, OpenAPIOptions{})

		resp, err := app.Test(jsonRequest("POST", "/items/1", `{"name": "x"}`))
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)

		responses := operationResponses(oapi, "/items/{id}", "post")
		assert.NotContains(t, responses, "422", "ProblemDetails=%v", problems)
		badRequest := responses["400"].(map[string]any)
		assert.Contains(t, badRequest["description"], "Validation error")
		assert.Contains(t, badRequest["description"], "Invalid request body")
		mediaType := fiber.MIMEApplicationJSON
		if problems {
			mediaType = MIMEApplicationProblemJSON
		}
		assert.Contains(t, badRequest["content"], mediaType, "ProblemDetails=%v", problems)
	}
}

func TestProblemDetails_Spec(t *testing.T) {
	_, oapi := newProblemApp(t, problemConfig())
	spec := oapi.GenerateOpenAPISpec()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	assert.Contains(t, schemas, "Problem")

	responses := operationResponses(oapi, "/items/{id}", "post")
	for _, code := range []string{"400", "401", "403", "404", "422"} {
		require.Contains(t, responses, code)
		content := responses[code].(map[string]any)["content"].(map[string]any)
		assert.NotContains(t, content, fiber.MIMEApplicationJSON, code)
		media := content[MIMEApplicationProblemJSON].(map[string]any)
		assert.Equal(t, "#/components/schemas/Problem", media["schema"].(map[string]any)["$ref"], code)
		example := media["example"].(*Problem)
		assert.Equal(t, code, statusCodeKey(example.Status))
	}
	_, err := oapi.GenerateOpenAPISpecYAML()
	assert.NoError(t, err)
}

// conflictError picks its own Content-Type.
type conflictError struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
}

func (e *conflictError) HTTPStatus() int     { return e.Status }
func (e *conflictError) ContentType() string { return MIMEApplicationProblemJSON }

func TestContentTyper_TErrorIsSentAndDocumentedWithItsContentType(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/conflict", func(c fiber.Ctx, _ struct{}) (fiber.Map, *conflictError) {
		return nil, &conflictError{Status: 409, Title: "already exists"}
	}, OpenAPIOptions{})
	Get(oapi, "/declared", func(c fiber.Ctx, _ struct{}) (fiber.Map, *Problem) {
		return nil, &Problem{Status: 409, Title: "Conflict", Code: "already_exists"}
	}, OpenAPIOptions{Errors: []any{&Problem{Status: 409, Title: "Conflict"}}})

	resp, err := app.Test(httptest.NewRequest("GET", "/conflict", nil))
	require.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
	assert.Equal(t, MIMEApplicationProblemJSON, resp.Header.Get("Content-Type"))

	status, p := readProblem(t, app, httptest.NewRequest("GET", "/declared", nil))
	assert.Equal(t, 409, status)
	assert.Equal(t, "already_exists", p.Code)

	catchAll := operationResponses(oapi, "/conflict", "get")["4XX"].(map[string]any)["content"].(map[string]any)
	assert.Contains(t, catchAll, MIMEApplicationProblemJSON)
	declared := operationResponses(oapi, "/declared", "get")["409"].(map[string]any)["content"].(map[string]any)
	assert.Contains(t, declared, MIMEApplicationProblemJSON)
}

func TestProblemDetails_AuthMiddlewares(t *testing.T) {
	app := fiber.New()
	app.Use(SmartAuthMiddleware(NewMockAuthService(), Config{ProblemDetails: true}))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendString("ok") })

	status, p := readProblem(t, app, newBearerRequest("GET", "/me", "bad-token"))
	assert.Equal(t, 401, status)
	assert.Equal(t, errTypeAuthN, p.Code)
}

func TestProblemDetails_DisabledKeepsTheEnvelope(t *testing.T) {
	app, _ := newProblemApp(t, Config{EnableValidation: true})
	resp, err := app.Test(jsonRequest("POST", "/items/1", `{"name": "x"}`))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get("Content-Type"))
	raw, _ := io.ReadAll(resp.Body)
	var envelope ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	assert.Equal(t, errTypeValidation, envelope.Errors[0].Type)
}

// operationResponses returns the documented responses of one operation.
func operationResponses(oapi *OApiApp, path, method string) map[string]any {
	spec := oapi.GenerateOpenAPISpec()
	return spec["paths"].(map[string]any)[path].(map[string]any)[method].(map[string]any)["responses"].(map[string]any)
}
