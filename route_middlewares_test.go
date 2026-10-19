package fiberoapi

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func traceMiddleware(name string, trace *[]string) fiber.Handler {
	return func(c fiber.Ctx) error {
		*trace = append(*trace, name)
		return c.Next()
	}
}

func TestRouteMiddlewares_RunInOrderBeforeTheHandler(t *testing.T) {
	var trace []string
	app := fiber.New()
	oapi := New(app)
	v1 := oapi.Group("/v1", traceMiddleware("group", &trace))
	Get(v1, "/items", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		trace = append(trace, "handler")
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{Middlewares: []fiber.Handler{
		traceMiddleware("first", &trace),
		nil, // ignored
		traceMiddleware("second", &trace),
	}})
	Get(v1, "/other", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		trace = append(trace, "other")
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{})

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/items", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, []string{"group", "first", "second", "handler"}, trace)

	trace = nil
	resp, err = app.Test(httptest.NewRequest("GET", "/v1/other", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, []string{"group", "other"}, trace, "route middlewares only apply to their route")
}

func TestRouteMiddlewares_CanStopTheChain(t *testing.T) {
	handlerCalls := 0
	app, decodes := newSpyApp()
	oapi := New(app)
	bodyLimit := func(c fiber.Ctx) error {
		if len(c.Body()) > 16 {
			return c.Status(fiber.StatusRequestEntityTooLarge).SendString("too large")
		}
		return c.Next()
	}
	rateLimit := func(c fiber.Ctx) error {
		if c.Get("X-Over-Quota") != "" {
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return c.Next()
	}
	Post(oapi, "/items", func(c fiber.Ctx, in authFirstBody) (fiber.Map, struct{}) {
		handlerCalls++
		return fiber.Map{"name": in.Name}, struct{}{}
	}, OpenAPIOptions{Middlewares: []fiber.Handler{rateLimit, bodyLimit}})

	req := httptest.NewRequest("POST", "/items", strings.NewReader(`{"name": "a very long name indeed"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 413, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "too large", string(raw))

	req = httptest.NewRequest("POST", "/items", strings.NewReader(`{"name": "abc"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Over-Quota", "1")
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode)

	assert.Zero(t, handlerCalls)
	assert.Zero(t, decodes.Load(), "the input is not read when a middleware answers")

	assert.Equal(t, 200, postJSON(t, app, "/items", "", `{"name": "abc"}`))
	assert.Equal(t, 1, handlerCalls)
}

func TestRouteMiddlewares_RunAfterAuthenticateFirst(t *testing.T) {
	var trace []string
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthenticateFirst:   true,
		AuthService:         NewMockAuthService(),
	})
	Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{Middlewares: []fiber.Handler{traceMiddleware("mw", &trace)}})

	status, _ := doBearer(t, app, "/me", "")
	assert.Equal(t, 401, status)
	assert.Empty(t, trace, "an unauthenticated request never reaches the route middlewares")

	status, _ = doBearer(t, app, "/me", "valid-token")
	assert.Equal(t, 200, status)
	assert.Equal(t, []string{"mw"}, trace)
}
