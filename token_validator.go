package fiberoapi

import (
	"errors"
	"slices"

	"github.com/gofiber/fiber/v3"
)

// ErrResourceAuthorizationNotConfigured is returned by TokenValidatorFunc when
// a route asks for resource permissions: the adapter only validates tokens, so
// such a route is a configuration error (500), never an access granted by default.
var ErrResourceAuthorizationNotConfigured = errors.New("resource authorization is not configured: the AuthService only validates tokens")

// TokenValidatorFunc adapts a plain token validation function into an
// AuthorizationService, for applications that only need to authenticate.
//
// Roles and scopes are read from the AuthContext the function returns.
// Resource permissions are not supported: a route that needs them answers 500.
type TokenValidatorFunc func(token string) (*AuthContext, error)

// ValidateToken calls f.
func (f TokenValidatorFunc) ValidateToken(token string) (*AuthContext, error) { return f(token) }

// HasRole reports whether the AuthContext returned by the validator carries role.
func (TokenValidatorFunc) HasRole(ctx *AuthContext, role string) bool {
	return ctx != nil && slices.Contains(ctx.Roles, role)
}

// HasScope reports whether the AuthContext returned by the validator carries scope.
func (TokenValidatorFunc) HasScope(ctx *AuthContext, scope string) bool {
	return ctx != nil && slices.Contains(ctx.Scopes, scope)
}

// CanAccessResource always fails with ErrResourceAuthorizationNotConfigured.
func (TokenValidatorFunc) CanAccessResource(*AuthContext, string, string, string) (bool, error) {
	return false, ErrResourceAuthorizationNotConfigured
}

// GetUserPermissions always fails with ErrResourceAuthorizationNotConfigured.
func (TokenValidatorFunc) GetUserPermissions(*AuthContext, string, string) (*ResourcePermission, error) {
	return nil, ErrResourceAuthorizationNotConfigured
}

// ContextTokenValidator is an optional interface for an AuthorizationService
// that needs the request to validate a Bearer token (client IP, request
// context, ...). When implemented, it is used instead of ValidateToken.
type ContextTokenValidator interface {
	ValidateTokenCtx(c fiber.Ctx, token string) (*AuthContext, error)
}

// validateToken validates a Bearer token, preferring ValidateTokenCtx when the
// service implements ContextTokenValidator.
func validateToken(c fiber.Ctx, authService AuthorizationService, token string) (*AuthContext, error) {
	if ctxValidator, ok := authService.(ContextTokenValidator); ok {
		return ctxValidator.ValidateTokenCtx(c, token)
	}
	return authService.ValidateToken(token)
}
