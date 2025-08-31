# Fiber OpenAPI

A Go library that extends Fiber to add automatic OpenAPI documentation generation with built-in validation and group support.

## Features

- ✅ **Complete HTTP methods** (GET, POST, PUT, DELETE) with automatic validation
- ✅ **Group support** with OpenAPI methods available on both app and groups
- ✅ **Unified API** with interface-based approach for seamless app/group usage
- ✅ **Powerful validation** via `github.com/go-playground/validator/v10`
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
        EnableValidation:  true,                // Enable input validation
        EnableOpenAPIDocs: true,                // Enable automatic docs setup
        OpenAPIDocsPath:   "/documentation",    // Custom docs path
        OpenAPIJSONPath:   "/api-spec.json",    // Custom spec path
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

## Configuration

The library supports flexible configuration through the `Config` struct:

```go
type Config struct {
    EnableValidation  bool   // Enable/disable input validation (default: true)
    EnableOpenAPIDocs bool   // Enable automatic docs setup (default: true)
    OpenAPIDocsPath   string // Path for documentation UI (default: "/docs")
    OpenAPIJSONPath   string // Path for OpenAPI JSON spec (default: "/openapi.json")
}
```

### Default Configuration

If no configuration is provided, the library uses these defaults:
- Validation: **enabled**
- Documentation: **enabled**
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