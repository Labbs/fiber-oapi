# Fiber OpenAPI

A Go library that extends Fiber to add automatic OpenAPI documentation generation with built-in validation and group support.

## Features

- ✅ **Complete HTTP methods** (GET, POST, PUT, DELETE) with automatic validation
- ✅ **Group support** with OpenAPI methods available on both app and groups
- ✅ **Unified API** with interface-based approach for seamless app/group usage
- ✅ **Powerful validation** via `github.com/go-playground/validator/v10`
- ✅ **Authentication & Authorization** with JWT/Bearer token support and role-based access control
- ✅ **OpenAPI Security** documentation with automatic security scheme generation
- ✅ **Type safety** with Go generics
- ✅ **Custom error handling**
- ✅ **OpenAPI documentation generation** with automatic schema generation
- ✅ **Redoc documentation UI** for modern, responsive API documentation
- ✅ **Support for path, query, and body parameters**
- ✅ **Automatic documentation setup** with configurable paths

## Installation

```bash
go get github.com/labbs/fiber-oapi
```

## Quick Start

### Basic Usage with Default Configuration

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    fiberoapi "github.com/labbs/fiber-oapi"
)

func main() {
    app := fiber.New()
    
    // Create OApi app with default configuration
    // Documentation will be available at /documentation (Redoc UI) and /openapi.json
    oapi := fiberoapi.New(app)

    // Your routes here...

    oapi.Listen(":3000")
}
```

### Using Groups

```go
func main() {
    app := fiber.New()
    oapi := fiberoapi.New(app)

    // Create groups with OpenAPI support
    v1 := fiberoapi.Group(oapi, "/api/v1")
    v2 := fiberoapi.Group(oapi, "/api/v2")
    
    // Nested groups
    users := fiberoapi.Group(v1, "/users")
    admin := fiberoapi.Group(v1, "/admin")

    // Routes work the same on app, groups, and nested groups
    fiberoapi.Get(oapi, "/health", handler, options)     // On app
    fiberoapi.Get(v1, "/status", handler, options)       // On group
    fiberoapi.Post(users, "/", handler, options)         // On nested group

    oapi.Listen(":3000")
}
```

### Custom Configuration

```go
func main() {
    app := fiber.New()
    
    // Custom configuration
    config := fiberoapi.Config{
        EnableValidation:    true,               // Enable input validation
        EnableOpenAPIDocs:   true,               // Enable automatic docs setup
        EnableAuthorization: false,              // Enable authentication (default: false)
        OpenAPIDocsPath:     "/docs",            // Custom docs path (default: /docs)
        OpenAPIJSONPath:     "/openapi.json",    // Custom spec path (default: /openapi.json)
    }
    oapi := fiberoapi.New(app, config)

    // Your routes here...

    oapi.Listen(":3000")
}
```

## Usage Examples

### GET with path parameters and validation

```go
type GetInput struct {
    Name string `path:"name" validate:"required,min=2"`
}

type GetOutput struct {
    Message string `json:"message"`
}

type GetError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

// Works on app
fiberoapi.Get(oapi, "/greeting/:name", 
    func(c *fiber.Ctx, input GetInput) (GetOutput, GetError) {
        return GetOutput{Message: "Hello " + input.Name}, GetError{}
    }, 
    fiberoapi.OpenAPIOptions{
        OperationID: "get-greeting",
        Tags:        []string{"greeting"},
        Summary:     "Get a personalized greeting",
    })

// Works on groups too
v1 := fiberoapi.Group(oapi, "/api/v1")
fiberoapi.Get(v1, "/greeting/:name", handler, options)
```

### POST with JSON body and validation

```go
type CreateUserInput struct {
    Username string `json:"username" validate:"required,min=3,max=20,alphanum"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"required,min=13,max=120"`
}

type CreateUserOutput struct {
    ID      string `json:"id"`
    Message string `json:"message"`
}

type CreateUserError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

fiberoapi.Post(oapi, "/users", 
    func(c *fiber.Ctx, input CreateUserInput) (CreateUserOutput, CreateUserError) {
        if input.Username == "admin" {
            return CreateUserOutput{}, CreateUserError{
                Code:    403,
                Message: "Username 'admin' is reserved",
            }
        }
        
        return CreateUserOutput{
            ID:      "user_" + input.Username,
            Message: "User created successfully",
        }, CreateUserError{}
    }, 
    fiberoapi.OpenAPIOptions{
        OperationID: "create-user",
        Tags:        []string{"users"},
        Summary:     "Create a new user",
    })
```

### PUT with path parameters and JSON body

```go
type UpdateUserInput struct {
    ID       string `path:"id" validate:"required"`
    Username string `json:"username" validate:"omitempty,min=3,max=20,alphanum"`
    Email    string `json:"email" validate:"omitempty,email"`
    Age      int    `json:"age" validate:"omitempty,min=13,max=120"`
}

type UpdateUserOutput struct {
    ID      string `json:"id"`
    Message string `json:"message"`
    Updated bool   `json:"updated"`
}

fiberoapi.Put(oapi, "/users/:id", 
    func(c *fiber.Ctx, input UpdateUserInput) (UpdateUserOutput, CreateUserError) {
        if input.ID == "notfound" {
            return UpdateUserOutput{}, CreateUserError{
                Code:    404,
                Message: "User not found",
            }
        }
        
        return UpdateUserOutput{
            ID:      input.ID,
            Message: "User updated successfully",
            Updated: true,
        }, CreateUserError{}
    }, 
    fiberoapi.OpenAPIOptions{
        OperationID: "update-user",
        Tags:        []string{"users"},
        Summary:     "Update an existing user",
    })
```

### DELETE with path parameters

```go
type DeleteUserInput struct {
    ID string `path:"id" validate:"required"`
}

type DeleteUserOutput struct {
    ID      string `json:"id"`
    Message string `json:"message"`
    Deleted bool   `json:"deleted"`
}

fiberoapi.Delete(oapi, "/users/:id", 
    func(c *fiber.Ctx, input DeleteUserInput) (DeleteUserOutput, CreateUserError) {
        if input.ID == "protected" {
            return DeleteUserOutput{}, CreateUserError{
                Code:    403,
                Message: "User is protected and cannot be deleted",
            }
        }
        
        return DeleteUserOutput{
            ID:      input.ID,
            Message: "User deleted successfully",
            Deleted: true,
        }, CreateUserError{}
    }, 
    fiberoapi.OpenAPIOptions{
        OperationID: "delete-user",
        Tags:        []string{"users"},
        Summary:     "Delete a user",
    })
```

## Authentication & Authorization

Fiber-oapi provides comprehensive authentication and authorization support with JWT/Bearer tokens, role-based access control, and automatic OpenAPI security documentation.

### Basic Authentication Setup

First, implement the `AuthorizationService` interface:

```go
package main

import (
    "fmt"
    "time"
    fiberoapi "github.com/labbs/fiber-oapi"
    "github.com/gofiber/fiber/v2"
)

// Implement the AuthorizationService interface
type MyAuthService struct{}

func (s *MyAuthService) ValidateToken(token string) (*fiberoapi.AuthContext, error) {
    // Validate your JWT token here
    switch token {
    case "admin-token":
        return &fiberoapi.AuthContext{
            UserID: "admin-123",
            Roles:  []string{"admin", "user"},
            Scopes: []string{"read", "write", "delete"},
            Claims: map[string]interface{}{
                "sub": "admin-123",
                "exp": time.Now().Add(time.Hour).Unix(),
            },
        }, nil
    case "user-token":
        return &fiberoapi.AuthContext{
            UserID: "user-789",
            Roles:  []string{"user"},
            Scopes: []string{"read", "write"},
            Claims: map[string]interface{}{
                "sub": "user-789",
                "exp": time.Now().Add(time.Hour).Unix(),
            },
        }, nil
    default:
        return nil, fmt.Errorf("invalid token")
    }
}

func (s *MyAuthService) HasRole(ctx *fiberoapi.AuthContext, role string) bool {
    for _, r := range ctx.Roles {
        if r == role {
            return true
        }
    }
    return false
}

func (s *MyAuthService) HasScope(ctx *fiberoapi.AuthContext, scope string) bool {
    for _, sc := range ctx.Scopes {
        if sc == scope {
            return true
        }
    }
    return false
}

func (s *MyAuthService) CanAccessResource(ctx *fiberoapi.AuthContext, resourceType, resourceID, action string) (bool, error) {
    // Admins can do everything
    if s.HasRole(ctx, "admin") {
        return true, nil
    }
    
    // Custom resource-based logic
    if resourceType == "document" && action == "delete" {
        return false, nil // Only admins can delete
    }
    
    return s.HasScope(ctx, action), nil
}

func (s *MyAuthService) GetUserPermissions(ctx *fiberoapi.AuthContext, resourceType, resourceID string) (*fiberoapi.ResourcePermission, error) {
    actions := []string{}
    if s.HasScope(ctx, "read") {
        actions = append(actions, "read")
    }
    if s.HasScope(ctx, "write") {
        actions = append(actions, "write")
    }
    if s.HasRole(ctx, "admin") {
        actions = append(actions, "delete")
    }
    
    return &fiberoapi.ResourcePermission{
        ResourceType: resourceType,
        ResourceID:   resourceID,
        Actions:      actions,
    }, nil
}

func main() {
    app := fiber.New()
    authService := &MyAuthService{}

    // Configure with authentication
    config := fiberoapi.Config{
        EnableValidation:    true,
        EnableOpenAPIDocs:   true,
        EnableAuthorization: true,
        AuthService:         authService,
        SecuritySchemes: map[string]fiberoapi.SecurityScheme{
            "bearerAuth": {
                Type:         "http",
                Scheme:       "bearer",
                BearerFormat: "JWT",
                Description:  "JWT Bearer token",
            },
        },
        DefaultSecurity: []map[string][]string{
            {"bearerAuth": {}},
        },
    }

    oapi := fiberoapi.New(app, config)

    // Your authenticated routes here...

    oapi.Listen(":3000")
}
```

### Public vs Protected Routes

```go
// Public route (no authentication required)
fiberoapi.Get(oapi, "/health",
    func(c *fiber.Ctx, input struct{}) (map[string]string, *fiberoapi.ErrorResponse) {
        return map[string]string{"status": "ok"}, nil
    },
    fiberoapi.OpenAPIOptions{
        Summary:  "Health check",
        Security: "disabled", // Explicitly disable auth for this route
    })

// Protected route (authentication required by default)
fiberoapi.Get(oapi, "/profile",
    func(c *fiber.Ctx, input struct{}) (UserProfile, *fiberoapi.ErrorResponse) {
        // Get authenticated user context
        authCtx, err := fiberoapi.GetAuthContext(c)
        if err != nil {
            return UserProfile{}, &fiberoapi.ErrorResponse{
                Code:    401,
                Details: "Authentication required",
                Type:    "auth_error",
            }
        }

        return UserProfile{
            UserID: authCtx.UserID,
            Roles:  authCtx.Roles,
        }, nil
    },
    fiberoapi.OpenAPIOptions{
        Summary: "Get user profile",
        Tags:    []string{"user"},
    })
```

### Role-Based Access Control

```go
type DocumentRequest struct {
    DocumentID string `path:"documentId" validate:"required"`
}

type DocumentResponse struct {
    ID      string `json:"id"`
    Title   string `json:"title"`
    Content string `json:"content"`
}

// Route requiring specific role
fiberoapi.Get(oapi, "/documents/:documentId",
    func(c *fiber.Ctx, input DocumentRequest) (DocumentResponse, *fiberoapi.ErrorResponse) {
        authCtx, _ := fiberoapi.GetAuthContext(c)

        // Check if user has required role
        if !authService.HasRole(authCtx, "user") {
            return DocumentResponse{}, &fiberoapi.ErrorResponse{
                Code:    403,
                Details: "User role required",
                Type:    "authorization_error",
            }
        }

        // Check if user has required scope
        if !authService.HasScope(authCtx, "read") {
            return DocumentResponse{}, &fiberoapi.ErrorResponse{
                Code:    403,
                Details: "Read permission required",
                Type:    "authorization_error",
            }
        }

        return DocumentResponse{
            ID:      input.DocumentID,
            Title:   "Document Title",
            Content: "Document content",
        }, nil
    },
    fiberoapi.OpenAPIOptions{
        Summary:     "Get document",
        Description: "Requires 'user' role and 'read' scope",
        Tags:        []string{"documents"},
    })

// Admin-only route
fiberoapi.Delete(oapi, "/documents/:documentId",
    func(c *fiber.Ctx, input DocumentRequest) (map[string]bool, *fiberoapi.ErrorResponse) {
        authCtx, _ := fiberoapi.GetAuthContext(c)

        // Only admins can delete
        if !authService.HasRole(authCtx, "admin") {
            return nil, &fiberoapi.ErrorResponse{
                Code:    403,
                Details: "Admin role required",
                Type:    "authorization_error",
            }
        }

        return map[string]bool{"deleted": true}, nil
    },
    fiberoapi.OpenAPIOptions{
        Summary:     "Delete document",
        Description: "Admin only - requires 'admin' role",
        Tags:        []string{"documents", "admin"},
    })
```

### Security Helpers

Use the helper functions to simplify security configuration:

```go
// Add security to existing options
options := fiberoapi.OpenAPIOptions{Summary: "Protected endpoint"}
options = fiberoapi.WithSecurity(options, map[string][]string{
    "bearerAuth": {},
})

// Disable security for specific route
options = fiberoapi.WithSecurityDisabled(options)

// Add required permissions for documentation
options = fiberoapi.WithPermissions(options, "document:read", "workspace:admin")

// Set resource type for documentation
options = fiberoapi.WithResourceType(options, "document")
```

### Authentication Context

Access the authenticated user's information in your handlers:

```go
fiberoapi.Post(oapi, "/posts",
    func(c *fiber.Ctx, input CreatePostInput) (PostResponse, *fiberoapi.ErrorResponse) {
        // Get authenticated user context
        authCtx, err := fiberoapi.GetAuthContext(c)
        if err != nil {
            return PostResponse{}, &fiberoapi.ErrorResponse{
                Code:    401,
                Details: "Authentication required",
                Type:    "auth_error",
            }
        }

        // Use auth context
        fmt.Printf("User %s with roles %v creating post\n", authCtx.UserID, authCtx.Roles)

        // Access JWT claims if needed
        if exp, ok := authCtx.Claims["exp"].(int64); ok {
            if time.Now().Unix() > exp {
                return PostResponse{}, &fiberoapi.ErrorResponse{
                    Code:    401,
                    Details: "Token expired",
                    Type:    "auth_error",
                }
            }
        }

        return PostResponse{
            ID:     "post-123",
            Author: authCtx.UserID,
            Title:  input.Title,
        }, nil
    },
    fiberoapi.OpenAPIOptions{
        Summary: "Create a post",
        Tags:    []string{"posts"},
    })
```

### OpenAPI Security Documentation

When authentication is enabled, the OpenAPI specification automatically includes:

- **Security Schemes**: JWT Bearer token configuration
- **Security Requirements**: Applied to protected endpoints
- **Security Overrides**: Public endpoints marked with `security: []`

```json
{
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT",
        "description": "JWT Bearer token"
      }
    }
  },
  "security": [
    {"bearerAuth": []}
  ],
  "paths": {
    "/health": {
      "get": {
        "security": [],
        "summary": "Health check"
      }
    },
    "/profile": {
      "get": {
        "security": [{"bearerAuth": []}],
        "summary": "Get user profile"
      }
    }
  }
}
```

### Testing with Authentication

```bash
# Public endpoint
curl http://localhost:3000/health

# Protected endpoint
curl -H "Authorization: Bearer your-jwt-token" http://localhost:3000/profile

# Admin endpoint
curl -H "Authorization: Bearer admin-token" -X DELETE http://localhost:3000/documents/123
```

### Error Responses

Authentication errors are automatically formatted:

```json
{
  "code": 401,
  "details": "Invalid token",
  "type": "auth_error"
}
```

```json
{
  "code": 403,
  "details": "Admin role required",
  "type": "authorization_error"
}
```

### Complete Authentication Example

See the complete working example in `_examples/auth/main.go` which demonstrates:
- Multiple user types with different roles and scopes
- Role-based access control
- Scope-based permissions
- Resource-level authorization
- OpenAPI security documentation
- Public and protected endpoints

```go
// Clone the repository and run the auth example
go run _examples/auth/main.go

// Visit http://localhost:3002/docs to see the documentation
// Test with different tokens: admin-token, user-token, readonly-token
```
```

## Configuration

The library supports flexible configuration through the `Config` struct:

```go
type Config struct {
    EnableValidation    bool                      // Enable/disable input validation (default: true)
    EnableOpenAPIDocs   bool                      // Enable automatic docs setup (default: true)
    EnableAuthorization bool                      // Enable authentication/authorization (default: false)
    OpenAPIDocsPath     string                    // Path for documentation UI (default: "/docs")
    OpenAPIJSONPath     string                    // Path for OpenAPI JSON spec (default: "/openapi.json")
    AuthService         AuthorizationService      // Service for handling auth
    SecuritySchemes     map[string]SecurityScheme // OpenAPI security schemes
    DefaultSecurity     []map[string][]string     // Default security requirements
}
```

### Default Configuration

If no configuration is provided, the library uses these defaults:
- Validation: **enabled**
- Documentation: **enabled** 
- Authorization: **disabled**
- Docs path: `/docs`
- JSON spec path: `/openapi.json`

### Disabling Features

```go
// Disable documentation but keep validation
config := fiberoapi.Config{
    EnableValidation:  true,
    EnableOpenAPIDocs: false,
}

// Or disable validation but keep docs
config := fiberoapi.Config{
    EnableValidation:  false,
    EnableOpenAPIDocs: true,
    OpenAPIDocsPath:   "/api-docs",
    OpenAPIJSONPath:   "/openapi.json",
}

// Enable authentication with custom paths
config := fiberoapi.Config{
    EnableValidation:    true,
    EnableOpenAPIDocs:   true,
    EnableAuthorization: true,
    AuthService:         &MyAuthService{},
    OpenAPIDocsPath:     "/documentation",
    OpenAPIJSONPath:     "/openapi.json",
}
```

## Validation

This library uses `validator/v10` for validation. You can use all supported validation tags:

- `required` - Required field
- `min=3,max=20` - Min/max length
- `email` - Valid email format
- `alphanum` - Alphanumeric characters only
- `uuid4` - UUID version 4
- `url` - Valid URL
- `oneof=admin user guest` - Value from a list
- `dive` - Validation for slice elements
- `gtfield=MinPrice` - Greater than another field

## Supported Parameter Types

- **Path parameters**: `path:"paramName"` (GET, POST, PUT, DELETE)
- **Query parameters**: `query:"paramName"` (GET, DELETE)
- **JSON body**: `json:"fieldName"` (POST, PUT)

## Supported HTTP Methods

All methods work with both the main app and groups through the unified API:

- **GET**: `fiberoapi.Get()` - Retrieve resources with path/query parameters
- **POST**: `fiberoapi.Post()` - Create resources with JSON body + optional path parameters  
- **PUT**: `fiberoapi.Put()` - Update resources with path parameters + JSON body
- **DELETE**: `fiberoapi.Delete()` - Delete resources with path parameters + optional query parameters

### Legacy Method Names (Still Supported)

For backward compatibility, the old method names are still available:
- `fiberoapi.GetOApi()` 
- `fiberoapi.PostOApi()`
- `fiberoapi.PutOApi()`
- `fiberoapi.DeleteOApi()`

## Groups

Fiber-oapi provides full support for Fiber groups while maintaining access to OpenAPI methods:

```go
// Create the main app
app := fiber.New()
oapi := fiberoapi.New(app)

// Create groups - they have access to all Fiber Router methods AND OpenAPI methods
v1 := fiberoapi.Group(oapi, "/api/v1")
v2 := fiberoapi.Group(oapi, "/api/v2")

// Nested groups work too
users := fiberoapi.Group(v1, "/users")
admin := fiberoapi.Group(v1, "/admin")

// Use OpenAPI methods on any router (app or group)
fiberoapi.Get(oapi, "/health", healthHandler, options)        // Main app
fiberoapi.Get(v1, "/status", statusHandler, options)          // Group
fiberoapi.Post(users, "/", createUserHandler, options)        // Nested group
fiberoapi.Put(users, "/:id", updateUserHandler, options)      // Nested group

// Use standard Fiber Router methods on groups (inherited via embedding)
v1.Use("/protected", authMiddleware)                          // Middleware
admin.Get("/stats", func(c *fiber.Ctx) error {              // Regular Fiber handler
    return c.JSON(fiber.Map{"stats": "data"})
})

// For static files, use the main Fiber app
app.Static("/files", "./uploads")  // Static files via main app

// Groups preserve full path context for OpenAPI documentation
// fiberoapi.Get(users, "/:id", ...) registers as GET /api/v1/users/{id}
```

### Group Features

- **Fiber Router compatibility**: Groups embed `fiber.Router` so standard Router methods work (Use, Get, Post, etc.)
- **OpenAPI method support**: Use `fiberoapi.Get()`, `fiberoapi.Post()`, etc. on groups
- **Nested groups**: Create groups within groups with proper path handling
- **Path prefix handling**: OpenAPI paths are automatically constructed with full prefixes
- **Unified API**: Same function names work on both app and groups through interface polymorphism

**Note**: For features like static file serving, use the main Fiber app: `app.Static("/path", "./dir")`

## Error Handling

Validation errors are automatically formatted and returned with HTTP status 400:

```json
{
  "error": "Validation failed",
  "details": "Key: 'CreateUserInput.Username' Error:Field validation for 'Username' failed on the 'min' tag"
}
```

Custom errors use the `StatusCode` from your error struct.

## Testing

Run tests:

```bash
go test -v
```

## Complete Example with Groups

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    fiberoapi "github.com/labbs/fiber-oapi"
)

type UserInput struct {
    ID int `path:"id" validate:"required,min=1"`
}

type UserOutput struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type UserError struct {
    Message string `json:"message"`
}

func main() {
    app := fiber.New()
    oapi := fiberoapi.New(app)

    // Global routes
    fiberoapi.Get(oapi, "/health", func(c *fiber.Ctx, input struct{}) (map[string]string, struct{}) {
        return map[string]string{"status": "ok"}, struct{}{}
    }, fiberoapi.OpenAPIOptions{
        Summary: "Health check",
        Tags:    []string{"health"},
    })

    // API v1 group
    v1 := fiberoapi.Group(oapi, "/api/v1")
    
    fiberoapi.Get(v1, "/users/:id", func(c *fiber.Ctx, input UserInput) (UserOutput, UserError) {
        return UserOutput{ID: input.ID, Name: "User " + string(rune(input.ID))}, UserError{}
    }, fiberoapi.OpenAPIOptions{
        Summary: "Get user by ID",
        Tags:    []string{"users"},
    })

    fiberoapi.Post(v1, "/users", func(c *fiber.Ctx, input UserOutput) (UserOutput, UserError) {
        return UserOutput{ID: 99, Name: input.Name}, UserError{}
    }, fiberoapi.OpenAPIOptions{
        Summary: "Create a new user",
        Tags:    []string{"users"},
    })

    // API v2 with nested groups
    v2 := fiberoapi.Group(oapi, "/api/v2")
    usersV2 := fiberoapi.Group(v2, "/users")

    fiberoapi.Get(usersV2, "/:id", func(c *fiber.Ctx, input UserInput) (UserOutput, UserError) {
        return UserOutput{ID: input.ID, Name: "User v2 " + string(rune(input.ID))}, UserError{}
    }, fiberoapi.OpenAPIOptions{
        Summary: "Get user by ID (v2)",
        Tags:    []string{"users", "v2"},
    })

    // Mix with standard Fiber methods
    v1.Use("/admin", func(c *fiber.Ctx) error {
        return c.Next() // Auth middleware
    })

    app.Listen(":3000")
    // Visit http://localhost:3000/docs to see the Redoc documentation
}
```

## Advanced Usage

### Custom Documentation Configuration

```go
config := fiberoapi.DocConfig{
    Title:       "My API",
    Description: "My API description",
    Version:     "2.0.0",
    DocsPath:    "/documentation",
    JSONPath:    "/api-spec.json",
}
oapi.SetupDocs(config) // Optional - docs are auto-configured by default
```

### OApiRouter Interface

The library uses an `OApiRouter` interface that allows the same functions to work seamlessly with both apps and groups:

```go
// This interface is implemented by both *OApiApp and *OApiGroup
type OApiRouter interface {
    GetApp() *OApiApp
    GetPrefix() string
}

// So these functions work with both:
func Get[T any, U any, E any](router OApiRouter, path string, handler HandlerFunc[T, U, E], options OpenAPIOptions)
func Post[T any, U any, E any](router OApiRouter, path string, handler HandlerFunc[T, U, E], options OpenAPIOptions)
func Put[T any, U any, E any](router OApiRouter, path string, handler HandlerFunc[T, U, E], options OpenAPIOptions)
func Delete[T any, U any, E any](router OApiRouter, path string, handler HandlerFunc[T, U, E], options OpenAPIOptions)
func Group(router OApiRouter, prefix string, handlers ...fiber.Handler) *OApiGroup
```

## Documentation

When `EnableOpenAPIDocs` is set to `true` (default), the library automatically sets up:

- **Redoc UI**: Modern, responsive documentation interface available at the configured docs path (default: `/docs`)
- **OpenAPI JSON**: Complete OpenAPI 3.0 specification available at the configured JSON path (default: `/openapi.json`)
- **Automatic Schema Generation**: Input and output types are automatically converted to OpenAPI schemas
- **Components Section**: All schemas are properly organized in the `components/schemas` section

No manual setup required! Just visit `http://localhost:3000/docs` to see your API documentation with Redoc.

### Redoc vs Swagger UI

This library uses **Redoc** for documentation UI instead of Swagger UI because:
- **Better performance** with large APIs
- **Responsive design** that works great on mobile
- **Clean, modern interface**
- **Better OpenAPI 3.0 support**
- **No JavaScript framework dependencies**

### OpenAPI Schema Generation

The library automatically generates OpenAPI 3.0 schemas from your Go types:

```go
// This struct automatically becomes an OpenAPI schema
type User struct {
    ID       string `json:"id"`
    Username string `json:"username" validate:"required,min=3"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"min=13,max=120"`
}
```

Generated OpenAPI spec will include:
- Complete path definitions with parameters
- Request/response schemas
- Validation rules as schema constraints
- Proper HTTP status codes
- Operation IDs, tags, and descriptions

## Migration from v1

If you're migrating from a previous version, here are the key changes:

### 1. New Unified API (Recommended)

```go
fiberoapi.Get(oapi, "/users/:id", handler, options)      // Works on app
fiberoapi.Post(oapi, "/users", handler, options)         // Works on app

// And seamlessly on groups
v1 := fiberoapi.Group(oapi, "/api/v1")
fiberoapi.Get(v1, "/users/:id", handler, options)        // Works on groups
fiberoapi.Post(v1, "/users", handler, options)           // Works on groups
```

### 2. Group Support

```go
// New group functionality
v1 := fiberoapi.Group(oapi, "/api/v1")
users := fiberoapi.Group(v1, "/users")

// All OpenAPI methods work on groups
fiberoapi.Get(users, "/:id", getUserHandler, options)
fiberoapi.Post(users, "/", createUserHandler, options)

// Standard Fiber Router methods work too (inherited via embedding)
users.Use(authMiddleware)                                     // Middleware
users.Get("/legacy", func(c *fiber.Ctx) error {             // Regular Fiber handler
    return c.SendString("legacy endpoint")
})

// For static files, use the main Fiber app
app.Static("/avatars", "./uploads")  // Static files via main app
```

### 3. Documentation UI

- **Changed from Swagger UI to Redoc** for better performance and modern UI
- Same paths: `/docs` for UI, `/openapi.json` for spec
- No code changes required for existing documentation setup