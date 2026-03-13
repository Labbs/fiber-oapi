# Fiber OpenAPI

A Go library that extends Fiber to add automatic OpenAPI documentation generation with built-in validation, authentication, and role-based access control.

## Features

- **Complete HTTP methods** (GET, POST, PUT, PATCH, DELETE, HEAD) with automatic validation
- **Group support** with OpenAPI methods available on both app and groups
- **Unified API** with interface-based approach for seamless app/group usage
- **Powerful validation** via `github.com/go-playground/validator/v10`
- **Multiple authentication schemes**: Bearer, Basic Auth, API Key, AWS SigV4
- **Declarative role-based access control** with OR/AND semantics
- **Custom error handlers** for validation and authentication errors
- **Per-route security overrides** and public routes
- **Type safety** with Go generics
- **OpenAPI 3.0 documentation** in JSON and YAML formats
- **Redoc documentation UI** for modern, responsive API documentation
- **OpenAPI extensions** (`x-required-roles`, `x-required-roles-mode`)
- **Conditional auth middleware** for flexible authentication strategies

## Installation

```bash
go get github.com/labbs/fiber-oapi
```

## Quick Start

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    fiberoapi "github.com/labbs/fiber-oapi"
)

func main() {
    app := fiber.New()
    oapi := fiberoapi.New(app)

    fiberoapi.Get(oapi, "/hello/:name",
        func(c *fiber.Ctx, input struct {
            Name string `path:"name" validate:"required,min=2"`
        }) (fiber.Map, *fiberoapi.ErrorResponse) {
            return fiber.Map{"message": "Hello " + input.Name}, nil
        },
        fiberoapi.OpenAPIOptions{
            Summary: "Say hello",
            Tags:    []string{"greeting"},
        })

    // Docs at /docs, spec at /openapi.json and /openapi.yaml
    app.Listen(":3000")
}
```

## Configuration

```go
type Config struct {
    EnableValidation       bool                      // Enable input validation (default: true)
    EnableOpenAPIDocs      bool                      // Enable automatic docs setup (default: true)
    EnableAuthorization    bool                      // Enable auth validation (default: false)
    OpenAPIDocsPath        string                    // Path for docs UI (default: "/docs")
    OpenAPIJSONPath        string                    // Path for JSON spec (default: "/openapi.json")
    OpenAPIYamlPath        string                    // Path for YAML spec (default: "/openapi.yaml")
    AuthService            AuthorizationService      // Service for handling auth
    SecuritySchemes        map[string]SecurityScheme // OpenAPI security schemes
    DefaultSecurity        []map[string][]string     // Default security requirements
    ValidationErrorHandler ValidationErrorHandler    // Custom handler for validation errors
    AuthErrorHandler       AuthErrorHandler          // Custom handler for auth errors (401/403/5xx)
}
```

Default config when none is provided:
- Validation: **enabled**
- Documentation: **enabled**
- Authorization: **disabled**
- Docs path: `/docs`
- JSON spec path: `/openapi.json`
- YAML spec path: `/openapi.yaml`

## HTTP Methods

All methods work with both the main app and groups:

```go
fiberoapi.Get(router, path, handler, options)
fiberoapi.Post(router, path, handler, options)
fiberoapi.Put(router, path, handler, options)
fiberoapi.Patch(router, path, handler, options)
fiberoapi.Delete(router, path, handler, options)
fiberoapi.Head(router, path, handler, options)
fiberoapi.Method(method, router, path, handler, options) // Custom HTTP method
```

## Parameter Types

```go
type MyInput struct {
    ID     string `path:"id" validate:"required"`           // Path parameter
    Filter string `query:"filter" validate:"omitempty"`      // Query parameter
    Auth   string `header:"Authorization"`                   // Header parameter
    Title  string `json:"title" validate:"required,min=1"`   // JSON body field
}
```

Special tags:
- `openapi:"-"` — Exclude a field from the OpenAPI schema (the field still works in the handler)
- `description:"text"` — Add a description to the field in the spec
- `resource:"document"` — Mark field as a resource identifier for dynamic authorization
- `action:"write"` — Specify the action for resource access checks

## Groups

```go
app := fiber.New()
oapi := fiberoapi.New(app)

v1 := fiberoapi.Group(oapi, "/api/v1")
users := fiberoapi.Group(v1, "/users")

// OpenAPI methods on groups
fiberoapi.Get(users, "/:id", getUser, options)   // Registers as GET /api/v1/users/{id}
fiberoapi.Post(users, "/", createUser, options)

// Standard Fiber middleware still works
v1.Use(authMiddleware)
```

## Validation

Uses `validator/v10`. Common tags:

```go
type Input struct {
    Name   string `json:"name" validate:"required,min=3,max=50"`
    Email  string `json:"email" validate:"required,email"`
    Age    int    `json:"age" validate:"min=13,max=120"`
    Role   string `json:"role" validate:"oneof=admin user guest"`
    Tags   []string `json:"tags" validate:"dive,min=1"`
}
```

## Authentication & Authorization

### Supported Security Schemes

| Scheme | Config | Validator Interface |
|--------|--------|-------------------|
| Bearer Token | `Type: "http", Scheme: "bearer"` | `AuthorizationService` (built-in) |
| HTTP Basic | `Type: "http", Scheme: "basic"` | `BasicAuthValidator` |
| API Key | `Type: "apiKey", In: "header"/"query"/"cookie"` | `APIKeyValidator` |
| AWS SigV4 | `Type: "http", Scheme: "AWS4-HMAC-SHA256"` | `AWSSignatureValidator` |

### Setup

Implement `AuthorizationService` (required) and any additional validator interfaces:

```go
type MyAuthService struct{}

// Required: AuthorizationService
func (s *MyAuthService) ValidateToken(token string) (*fiberoapi.AuthContext, error) { ... }
func (s *MyAuthService) HasRole(ctx *fiberoapi.AuthContext, role string) bool { ... }
func (s *MyAuthService) HasScope(ctx *fiberoapi.AuthContext, scope string) bool { ... }
func (s *MyAuthService) CanAccessResource(ctx *fiberoapi.AuthContext, resourceType, resourceID, action string) (bool, error) { ... }
func (s *MyAuthService) GetUserPermissions(ctx *fiberoapi.AuthContext, resourceType, resourceID string) (*fiberoapi.ResourcePermission, error) { ... }

// Optional: BasicAuthValidator
func (s *MyAuthService) ValidateBasicAuth(username, password string) (*fiberoapi.AuthContext, error) { ... }

// Optional: APIKeyValidator
func (s *MyAuthService) ValidateAPIKey(key, location, paramName string) (*fiberoapi.AuthContext, error) { ... }

// Optional: AWSSignatureValidator
func (s *MyAuthService) ValidateAWSSignature(params *fiberoapi.AWSSignatureParams) (*fiberoapi.AuthContext, error) { ... }
```

Configure multiple security schemes (OR semantics between them):

```go
config := fiberoapi.Config{
    EnableAuthorization: true,
    AuthService:         &MyAuthService{},
    SecuritySchemes: map[string]fiberoapi.SecurityScheme{
        "bearerAuth": {
            Type:         "http",
            Scheme:       "bearer",
            BearerFormat: "JWT",
            Description:  "JWT Bearer token",
        },
        "basicAuth": {
            Type:        "http",
            Scheme:      "basic",
            Description: "HTTP Basic authentication",
        },
        "apiKeyAuth": {
            Type:        "apiKey",
            In:          "header",
            Name:        "X-API-Key",
            Description: "API Key via header",
        },
    },
    // Any of these schemes can authenticate a request (OR semantics)
    DefaultSecurity: []map[string][]string{
        {"bearerAuth": {}},
        {"basicAuth": {}},
        {"apiKeyAuth": {}},
    },
}
oapi := fiberoapi.New(app, config)
```

### Public vs Protected Routes

```go
// Public route — no authentication
fiberoapi.Get(oapi, "/health", handler,
    fiberoapi.OpenAPIOptions{
        Summary:  "Health check",
        Security: "disabled",
    })

// Protected route — uses default security
fiberoapi.Get(oapi, "/profile", handler,
    fiberoapi.OpenAPIOptions{
        Summary: "Get profile",
    })

// Per-route security override
fiberoapi.Get(oapi, "/admin", handler,
    fiberoapi.WithSecurity(
        fiberoapi.OpenAPIOptions{Summary: "Admin endpoint"},
        []map[string][]string{{"bearerAuth": {}}}, // Only bearer, not API key
    ))
```

### Declarative Role-Based Access Control

Roles are checked automatically before your handler runs. No manual checks needed.

```go
// OR semantics: user needs at least ONE of the listed roles
fiberoapi.Get(oapi, "/documents/:id", handler,
    fiberoapi.WithRoles(
        fiberoapi.OpenAPIOptions{Summary: "Get document", Tags: []string{"documents"}},
        "admin", "editor",  // admin OR editor can access
    ))

// AND semantics: user needs ALL of the listed roles
fiberoapi.Delete(oapi, "/documents/:id", handler,
    fiberoapi.WithAllRoles(
        fiberoapi.OpenAPIOptions{Summary: "Delete document", Tags: []string{"documents"}},
        "admin", "moderator",  // must be admin AND moderator
    ))

// Inline via OpenAPIOptions
fiberoapi.Get(oapi, "/settings", handler,
    fiberoapi.OpenAPIOptions{
        Summary:         "Settings",
        RequiredRoles:   []string{"admin", "superadmin"},
        RequireAllRoles: false,  // OR semantics (default)
    })
```

Roles appear in the OpenAPI spec as extensions:

```json
{
    "x-required-roles": ["admin", "editor"],
    "x-required-roles-mode": "any"
}
```

### Permissions and Resource Access

```go
// RequiredPermissions are documented in the OpenAPI spec description
fiberoapi.Put(oapi, "/documents/:id", handler,
    fiberoapi.OpenAPIOptions{
        Summary:             "Update document",
        RequiredRoles:       []string{"editor"},
        RequiredPermissions: []string{"document:write"},
    })

// Resource-based access via struct tags
type UpdateDocInput struct {
    DocumentID string `path:"documentId" validate:"required" resource:"document" action:"write"`
    Title      string `json:"title" validate:"required"`
}

// Dynamic resource access check in handler
fiberoapi.RequireResourceAccess(c, authService, "document", docID, "delete")
```

### Authentication Context

Access the authenticated user in handlers:

```go
fiberoapi.Get(oapi, "/me", func(c *fiber.Ctx, input struct{}) (fiber.Map, *fiberoapi.ErrorResponse) {
    authCtx, err := fiberoapi.GetAuthContext(c)
    if err != nil {
        return nil, &fiberoapi.ErrorResponse{Code: 401, Details: "Not authenticated"}
    }
    return fiber.Map{
        "user_id": authCtx.UserID,
        "roles":   authCtx.Roles,
        "scopes":  authCtx.Scopes,
        "claims":  authCtx.Claims,
    }, nil
}, fiberoapi.OpenAPIOptions{Summary: "Current user"})
```

## Custom Error Handlers

### Validation Errors

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
        return c.Status(400).JSON(fiber.Map{
            "success": false,
            "error":   err.Error(),
        })
    },
})
```

### Authentication/Authorization Errors

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    EnableAuthorization: true,
    AuthService:         authService,
    AuthErrorHandler: func(c *fiber.Ctx, err *fiberoapi.AuthError) error {
        // err.StatusCode: 401, 403, or 5xx
        // err.Message: human-readable error message
        return c.Status(err.StatusCode).JSON(fiber.Map{
            "error":   err.Message,
            "status":  err.StatusCode,
        })
    },
})
```

Without custom handlers, default error responses are returned:

```json
// 401 - Authentication failure
{"code": 401, "details": "invalid token", "type": "authentication_error"}

// 403 - Authorization failure
{"code": 403, "details": "requires one of: admin, editor", "type": "authorization_error"}

// 400 - Validation failure
{"code": 400, "details": "...", "type": "validation_error"}
```

## Conditional Auth Middleware

Standalone middleware functions for use outside the declarative route system:

```go
// Smart middleware that auto-detects security schemes and excludes doc routes
app.Use(fiberoapi.SmartAuthMiddleware(authService, config))

// Skip auth for specific paths
app.Use(fiberoapi.ConditionalAuthMiddleware(
    fiberoapi.BearerTokenMiddleware(authService),
    "/health", "/docs", "/openapi.json",
))

// Individual scheme middleware
app.Use(fiberoapi.BearerTokenMiddleware(authService))
app.Use(fiberoapi.BasicAuthMiddleware(authService))
app.Use(fiberoapi.APIKeyMiddleware(authService, scheme))
app.Use(fiberoapi.AWSSignatureMiddleware(authService))

// Role guard middleware
app.Use(fiberoapi.RoleGuard(authService, "admin"))
```

## Security Helpers

```go
opts := fiberoapi.OpenAPIOptions{Summary: "My endpoint"}

// Security
opts = fiberoapi.WithSecurity(opts, []map[string][]string{{"bearerAuth": {}}})
opts = fiberoapi.WithSecurityDisabled(opts)

// Roles
opts = fiberoapi.WithRoles(opts, "admin", "editor")       // OR semantics
opts = fiberoapi.WithAllRoles(opts, "admin", "moderator")  // AND semantics

// Documentation
opts = fiberoapi.WithPermissions(opts, "document:read", "document:write")
opts = fiberoapi.WithResourceType(opts, "document")
```

## OpenAPI Spec Generation

The spec is available in both JSON and YAML:

```go
// Automatic endpoints
// GET /openapi.json
// GET /openapi.yaml
// GET /docs (Redoc UI)

// Programmatic access
spec := oapi.GenerateOpenAPISpec()           // map[string]interface{}
yamlSpec, err := oapi.GenerateOpenAPISpecYAML() // string
```

### Custom Documentation

```go
oapi.SetupDocs(fiberoapi.DocConfig{
    Title:       "My API",
    Description: "My API description",
    Version:     "2.0.0",
    DocsPath:    "/documentation",
    JSONPath:    "/api-spec.json",
})
```

## Testing

```bash
# Run all tests
go test -v ./...

# Run the auth example
go run _examples/auth/main.go
# Visit http://localhost:3002/docs
```

Testing with authentication:

```bash
# Bearer token
curl -H "Authorization: Bearer admin-token" http://localhost:3002/me

# Basic auth
curl --user admin:admin-pass http://localhost:3002/me

# API key
curl -H "X-API-Key: my-secret-api-key" http://localhost:3002/documents/doc-1

# Public endpoint
curl http://localhost:3002/health
```

## Complete Example

See `_examples/auth/main.go` for a full working example with:
- Multiple security schemes (Bearer, Basic, API Key, AWS SigV4)
- Declarative role-based access control
- Custom auth error handler
- Public and protected routes
- Resource-level authorization
- OpenAPI documentation with security schemes
