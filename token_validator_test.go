package fiberoapi

import (
	"errors"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTokenValidatorApp(t *testing.T, svc AuthorizationService) (*fiber.App, *OApiApp) {
	t.Helper()
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         svc,
	})
	return app, oapi
}

var staticTokens = TokenValidatorFunc(func(token string) (*AuthContext, error) {
	switch token {
	case "admin":
		return &AuthContext{UserID: "u-admin", Roles: []string{"admin"}, Scopes: []string{"write"}}, nil
	case "user":
		return &AuthContext{UserID: "u-user", Roles: []string{"user"}}, nil
	}
	return nil, errTokenExpired
})

func TestTokenValidatorFunc_AuthenticatesRequests(t *testing.T) {
	app, oapi := newTokenValidatorApp(t, staticTokens)
	Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		authCtx, err := GetAuthContext(c)
		if err != nil {
			return fiber.Map{"user": ""}, struct{}{}
		}
		return fiber.Map{"user": authCtx.UserID}, struct{}{}
	}, OpenAPIOptions{})

	status, body := doBearer(t, app, "/me", "user")
	assert.Equal(t, 200, status)
	assert.Equal(t, "u-user", body["user"])

	status, _ = doBearer(t, app, "/me", "nope")
	assert.Equal(t, 401, status)
}

func TestTokenValidatorFunc_RolesComeFromAuthContext(t *testing.T) {
	app, oapi := newTokenValidatorApp(t, staticTokens)
	Get(oapi, "/admin", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, WithRoles(OpenAPIOptions{}, "admin"))

	status, _ := doBearer(t, app, "/admin", "admin")
	assert.Equal(t, 200, status)
	status, _ = doBearer(t, app, "/admin", "user")
	assert.Equal(t, 403, status)

	assert.True(t, staticTokens.HasScope(&AuthContext{Scopes: []string{"write"}}, "write"))
	assert.False(t, staticTokens.HasScope(nil, "write"))
}

type tokenValidatorDocInput struct {
	DocID string `uri:"docId" resource:"document" action:"read"`
}

func TestTokenValidatorFunc_ResourceRouteIsAConfigurationError(t *testing.T) {
	app, oapi := newTokenValidatorApp(t, staticTokens)
	Get(oapi, "/documents/:docId", func(c fiber.Ctx, in tokenValidatorDocInput) (fiber.Map, struct{}) {
		return fiber.Map{"id": in.DocID}, struct{}{}
	}, OpenAPIOptions{})

	status, _ := doBearer(t, app, "/documents/doc-1", "admin")
	assert.Equal(t, 500, status, "a resource check with a token-only validator must never grant access")

	_, err := staticTokens.GetUserPermissions(nil, "document", "doc-1")
	assert.ErrorIs(t, err, ErrResourceAuthorizationNotConfigured)
}

// ctxTokenService rejects every token through ValidateToken; only
// ValidateTokenCtx, which reads the request, can accept one.
type ctxTokenService struct{ causeAuthService }

func (ctxTokenService) ValidateToken(string) (*AuthContext, error) {
	return nil, errors.New("ValidateToken must not be called")
}

func (ctxTokenService) ValidateTokenCtx(c fiber.Ctx, token string) (*AuthContext, error) {
	if token == "ok" && c.Get("X-Device") == "d-1" {
		return &AuthContext{UserID: "u-1"}, nil
	}
	return nil, errTokenExpired
}

func TestContextTokenValidator_IsPreferred(t *testing.T) {
	for name, schemes := range map[string]map[string]SecurityScheme{
		"bearer only":      nil,
		"security schemes": {"bearerAuth": {Type: "http", Scheme: "bearer"}},
	} {
		t.Run(name, func(t *testing.T) {
			app := fiber.New()
			oapi := New(app, Config{
				EnableValidation:    true,
				EnableAuthorization: true,
				AuthService:         ctxTokenService{},
				SecuritySchemes:     schemes,
			})
			Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
				return fiber.Map{"ok": true}, struct{}{}
			}, OpenAPIOptions{})

			req := newBearerRequest("GET", "/me", "ok")
			req.Header.Set("X-Device", "d-1")
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		})
	}

	app := fiber.New()
	app.Use(BearerTokenMiddleware(ctxTokenService{}))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendString("ok") })
	req := newBearerRequest("GET", "/me", "ok")
	req.Header.Set("X-Device", "d-1")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "BearerTokenMiddleware prefers ValidateTokenCtx too")
}

// nilContextService accepts every credential without returning a context.
type nilContextService struct{ causeAuthService }

func (nilContextService) ValidateToken(string) (*AuthContext, error)             { return nil, nil }
func (nilContextService) ValidateBasicAuth(string, string) (*AuthContext, error) { return nil, nil }

func TestNilAuthContext_IsAnAuthenticationFailure(t *testing.T) {
	for name, schemes := range map[string]map[string]SecurityScheme{
		"bearer only":      nil,
		"security schemes": {"bearerAuth": {Type: "http", Scheme: "bearer"}},
	} {
		t.Run(name, func(t *testing.T) {
			app, decodes := newSpyApp()
			oapi := New(app, Config{
				EnableValidation:    true,
				EnableAuthorization: true,
				AuthService:         nilContextService{},
				SecuritySchemes:     schemes,
			})
			registerAuthFirstRoute(oapi)

			assert.Equal(t, 401, postJSON(t, app, "/items", "any", `{"name": "widget"}`))
			assert.Zero(t, decodes.Load())
		})
	}

	basic := SecurityScheme{Type: "http", Scheme: "basic"}
	for name, mw := range map[string]fiber.Handler{
		"BearerTokenMiddleware": BearerTokenMiddleware(nilContextService{}),
		"SmartAuthMiddleware": SmartAuthMiddleware(nilContextService{}, Config{
			SecuritySchemes: map[string]SecurityScheme{"bearerAuth": {Type: "http", Scheme: "bearer"}},
		}),
		"BasicAuthMiddleware": BasicAuthMiddleware(nilContextService{}),
		"MultiSchemeAuthMiddleware basic": MultiSchemeAuthMiddleware(nilContextService{}, Config{
			SecuritySchemes: map[string]SecurityScheme{"basicAuth": basic},
		}),
	} {
		t.Run(name, func(t *testing.T) {
			app := fiber.New()
			app.Use(mw)
			app.Get("/me", func(c fiber.Ctx) error { return c.SendString("ok") })

			req := newBearerRequest("GET", "/me", "any")
			if strings.Contains(name, "Basic") || strings.Contains(name, "basic") {
				req.SetBasicAuth("user", "pass")
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, 401, resp.StatusCode)
		})
	}
}
