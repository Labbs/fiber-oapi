package fiberoapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTokenExpired = errors.New("token expired")

// deviceRevokedError carries data the AuthErrorHandler can extract with
// errors.As, and its own HTTP status.
type deviceRevokedError struct{ DeviceID string }

func (e *deviceRevokedError) Error() string   { return "device " + e.DeviceID + " revoked" }
func (e *deviceRevokedError) HTTPStatus() int { return fiber.StatusForbidden }

// causeAuthService maps a few well-known tokens onto typed failures.
type causeAuthService struct{}

func (causeAuthService) ValidateToken(token string) (*AuthContext, error) {
	switch token {
	case "expired":
		return nil, errTokenExpired
	case "revoked":
		return nil, &deviceRevokedError{DeviceID: "d-42"}
	case "ok":
		return &AuthContext{UserID: "u-1"}, nil
	}
	return nil, errors.New("unknown token")
}
func (causeAuthService) HasRole(*AuthContext, string) bool  { return false }
func (causeAuthService) HasScope(*AuthContext, string) bool { return false }
func (causeAuthService) CanAccessResource(*AuthContext, string, string, string) (bool, error) {
	return false, nil
}
func (causeAuthService) GetUserPermissions(*AuthContext, string, string) (*ResourcePermission, error) {
	return nil, nil
}

// classifyingAuthErrorHandler answers with a stable code derived from the cause.
func classifyingAuthErrorHandler(c fiber.Ctx, err *AuthError) error {
	code := "unauthorized"
	var revoked *deviceRevokedError
	switch {
	case errors.Is(err, errTokenExpired):
		code = "token_expired"
	case errors.As(err, &revoked):
		code = "device_revoked:" + revoked.DeviceID
	}
	return c.Status(err.StatusCode).JSON(fiber.Map{"code": code})
}

func newBearerRequest(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func doBearer(t *testing.T, app *fiber.App, path, token string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(newBearerRequest("GET", path, token))
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	return resp.StatusCode, body
}

func TestAuthErrorCause_ReachableFromAuthErrorHandler(t *testing.T) {
	for _, tc := range []struct {
		name    string
		schemes map[string]SecurityScheme
	}{
		{name: "bearer only", schemes: nil},
		{name: "security schemes", schemes: map[string]SecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			oapi := New(app, Config{
				EnableValidation:    true,
				EnableAuthorization: true,
				AuthService:         causeAuthService{},
				SecuritySchemes:     tc.schemes,
				AuthErrorHandler:    classifyingAuthErrorHandler,
			})
			Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
				return fiber.Map{"ok": true}, struct{}{}
			}, OpenAPIOptions{})

			status, body := doBearer(t, app, "/me", "expired")
			assert.Equal(t, 401, status)
			assert.Equal(t, "token_expired", body["code"])

			status, body = doBearer(t, app, "/me", "revoked")
			assert.Equal(t, 403, status, "the status comes from the cause's HTTPStatus()")
			assert.Equal(t, "device_revoked:d-42", body["code"])

			status, _ = doBearer(t, app, "/me", "ok")
			assert.Equal(t, 200, status)
		})
	}
}

func TestAuthErrorCause_StatusFromHTTPStatusInDefaultEnvelope(t *testing.T) {
	app := fiber.New()
	oapi := New(app, Config{
		EnableValidation:    true,
		EnableAuthorization: true,
		AuthService:         causeAuthService{},
	})
	Get(oapi, "/me", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{})

	status, body := doBearer(t, app, "/me", "revoked")
	assert.Equal(t, 403, status)
	entry := body["errors"].([]any)[0].(map[string]any)
	assert.Equal(t, errTypeAuthZ, entry["type"])
	assert.Equal(t, "device d-42 revoked", entry["msg"])
}

func TestAuthErrorFrom(t *testing.T) {
	authErr := authErrorFrom(errTokenExpired)
	assert.Equal(t, 401, authErr.StatusCode)
	assert.ErrorIs(t, authErr, errTokenExpired)

	authErr = authErrorFrom(&ScopeError{Scope: "write"})
	assert.Equal(t, 403, authErr.StatusCode)
	_, ok := errors.AsType[*ScopeError](authErr)
	assert.True(t, ok)

	typed := &AuthError{StatusCode: 500, Message: "misconfigured"}
	assert.Same(t, typed, authErrorFrom(typed), "a typed AuthError is returned as-is")
}

func TestAuthMiddlewares_HonourAuthErrorHandler(t *testing.T) {
	multi := Config{
		SecuritySchemes:  map[string]SecurityScheme{"bearerAuth": {Type: "http", Scheme: "bearer"}},
		AuthErrorHandler: classifyingAuthErrorHandler,
	}
	bearerOnly := Config{AuthErrorHandler: classifyingAuthErrorHandler}

	for name, mw := range map[string]fiber.Handler{
		"MultiSchemeAuthMiddleware":        MultiSchemeAuthMiddleware(causeAuthService{}, multi),
		"SmartAuthMiddleware multi-scheme": SmartAuthMiddleware(causeAuthService{}, multi),
		"SmartAuthMiddleware bearer only":  SmartAuthMiddleware(causeAuthService{}, bearerOnly),
	} {
		t.Run(name, func(t *testing.T) {
			app := fiber.New()
			app.Use(mw)
			app.Get("/me", func(c fiber.Ctx) error { return c.SendString("ok") })

			status, body := doBearer(t, app, "/me", "expired")
			assert.Equal(t, 401, status)
			assert.Equal(t, "token_expired", body["code"])

			status, body = doBearer(t, app, "/me", "revoked")
			assert.Equal(t, 403, status)
			assert.Equal(t, "device_revoked:d-42", body["code"])

			status, body = doBearer(t, app, "/me", "")
			assert.Equal(t, 401, status)
			assert.Equal(t, "unauthorized", body["code"])
		})
	}
}

func TestAuthMiddlewares_LegacyBodyWithoutHandler(t *testing.T) {
	app := fiber.New()
	app.Use(SmartAuthMiddleware(causeAuthService{}, Config{}))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendString("ok") })

	status, body := doBearer(t, app, "/me", "revoked")
	assert.Equal(t, 401, status, "without a handler the legacy status is kept")
	assert.Equal(t, "Invalid token", body["error"])
}
