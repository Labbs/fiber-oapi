package fiberoapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type authFirstBody struct {
	Name string `json:"name" validate:"required,min=3"`
}

// newSpyApp returns a Fiber app whose JSON decoder counts its calls, so a
// test can tell whether the request body was read.
func newSpyApp() (*fiber.App, *atomic.Int32) {
	var decodes atomic.Int32
	app := fiber.New(fiber.Config{
		JSONDecoder: func(data []byte, v any) error {
			decodes.Add(1)
			return json.Unmarshal(data, v)
		},
	})
	return app, &decodes
}

func postJSON(t *testing.T, app *fiber.App, path, token, body string) int {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp.StatusCode
}

func registerAuthFirstRoute(oapi *OApiApp) {
	Post(oapi, "/items", func(c fiber.Ctx, in authFirstBody) (fiber.Map, struct{}) {
		return fiber.Map{"name": in.Name}, struct{}{}
	}, OpenAPIOptions{})
}

func TestAuthFirst_RejectsBeforeReadingTheBody(t *testing.T) {
	app, decodes := newSpyApp()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
	})
	registerAuthFirstRoute(oapi)

	assert.Equal(t, 401, postJSON(t, app, "/items", "", `{"name": 42`))
	assert.Equal(t, 401, postJSON(t, app, "/items", "bad-token", `{"name": "x"}`))
	assert.Zero(t, decodes.Load(), "the body must not be decoded for an unauthenticated request")

	assert.Equal(t, 422, postJSON(t, app, "/items", "valid-token", `{"name": "x"}`))
	assert.Equal(t, 200, postJSON(t, app, "/items", "valid-token", `{"name": "widget"}`))
	assert.Equal(t, int32(2), decodes.Load())
}

func TestAuthFirst_AuthErrorsComeBeforeInputErrors(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
	})
	registerAuthFirstRoute(oapi)

	cases := []struct {
		name        string
		token, body string
		want        int
	}{
		{name: "malformed body, no token", body: `{"name": 42`, want: 401},
		{name: "invalid body, no token", body: `{"name": "x"}`, want: 401},
		{name: "valid body, no token", body: `{"name": "widget"}`, want: 401},
		{name: "malformed body, bad token", token: "bad-token", body: `{"name": 42`, want: 401},
		{name: "malformed body, valid token", token: "valid-token", body: `{"name": 42`, want: 400},
		{name: "invalid body, valid token", token: "valid-token", body: `{"name": "x"}`, want: 422},
		{name: "valid body, valid token", token: "valid-token", body: `{"name": "widget"}`, want: 200},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, postJSON(t, app, "/items", tc.token, tc.body), tc.name)
	}
}

func TestAuthFirst_RouteMiddlewareSeesTheAuthContext(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
	})
	var seen string
	Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{Middlewares: []fiber.Handler{func(c fiber.Ctx) error {
		authCtx, err := GetAuthContext(c)
		if err != nil {
			return err
		}
		seen = authCtx.UserID
		return c.Next()
	}}})

	status, _ := doBearer(t, app, "/me", "valid-token")
	assert.Equal(t, 200, status)
	assert.Equal(t, "user-123", seen)
}

type authFirstDocInput struct {
	DocID string `uri:"docId" resource:"document" action:"delete"`
}

func TestAuthFirst_RolesAndResourcesStillApply(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
	})
	Get(oapi, "/admin", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, WithRoles(OpenAPIOptions{}, "admin"))
	Delete(oapi, "/documents/:docId", func(c fiber.Ctx, in authFirstDocInput) (fiber.Map, struct{}) {
		return fiber.Map{"deleted": in.DocID}, struct{}{}
	}, OpenAPIOptions{})

	status, _ := doBearer(t, app, "/admin", "valid-token")
	assert.Equal(t, 403, status)
	status, _ = doBearer(t, app, "/admin", "admin-token")
	assert.Equal(t, 200, status)

	del := func(token string) int {
		resp, err := app.Test(newBearerRequest("DELETE", "/documents/doc-1", token))
		require.NoError(t, err)
		return resp.StatusCode
	}
	assert.Equal(t, 401, del(""))
	assert.Equal(t, 403, del("valid-token"))
	assert.Equal(t, 200, del("admin-token"))
}

func TestAuthFirst_DisabledRouteSkipsAuthentication(t *testing.T) {
	var validations atomic.Int32
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService: TokenValidatorFunc(func(string) (*AuthContext, error) {
			validations.Add(1)
			return nil, errTokenExpired
		}),
	})
	Get(oapi, "/health", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{Security: "disabled"})

	status, _ := doBearer(t, app, "/health", "whatever")
	assert.Equal(t, 200, status, "a public route must not reject an invalid token")
	assert.Zero(t, validations.Load())
}

func TestAuthFirst_UsesTheErrorHandlers(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         causeAuthService{},
		AuthErrorHandler:    classifyingAuthErrorHandler,
	})
	Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{})

	status, body := doBearer(t, app, "/me", "expired")
	assert.Equal(t, 401, status)
	assert.Equal(t, "token_expired", body["code"])
}

func TestSpec_ProtectedRoutesDocumentAuthErrors(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         NewMockAuthService(),
	})
	handler := func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) { return fiber.Map{}, struct{}{} }
	Get(oapi, "/me", handler, OpenAPIOptions{})
	Get(oapi, "/admin", handler, WithRoles(OpenAPIOptions{}, "admin"))
	Get(oapi, "/public", handler, OpenAPIOptions{Security: "disabled"})
	Get(oapi, "/documents/:docId", func(c fiber.Ctx, _ authFirstDocInput) (fiber.Map, struct{}) {
		return fiber.Map{}, struct{}{}
	}, OpenAPIOptions{})

	spec := oapi.GenerateOpenAPISpec()
	responsesOf := func(path string) map[string]any {
		return spec["paths"].(map[string]any)[path].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)
	}

	me := responsesOf("/me")
	assert.Contains(t, me, "401")
	assert.NotContains(t, me, "403")
	example := me["401"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["example"].(ErrorEnvelope)
	assert.Equal(t, errTypeAuthN, example.Errors[0].Type)

	assert.Contains(t, responsesOf("/admin"), "403")
	assert.Contains(t, responsesOf("/documents/{docId}"), "403")
	assert.NotContains(t, responsesOf("/public"), "401")

	// Without authorization, nothing is documented.
	plain := New(fiber.New())
	Get(plain, "/me", handler, OpenAPIOptions{})
	plainResponses := plain.GenerateOpenAPISpec()["paths"].(map[string]any)["/me"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)
	assert.NotContains(t, plainResponses, "401")
}
