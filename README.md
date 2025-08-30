# Fiber OpenAPI

A Go library that extends Fiber to add automatic OpenAPI documentation generation with built-in validation.

## Features

- ✅ **Complete HTTP methods** (GET, POST, PUT, DELETE) with automatic validation
- ✅ **Powerful validation** via `github.com/go-playground/validator/v10`
- ✅ **Type safety** with Go generics
- ✅ **Custom error handling**
- ✅ **OpenAPI documentation generation** with automatic schema generation
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
    // Documentation will be available at /docs and /openapi.json
    oapi := fiberoapi.New(app)

    // Your routes here...

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

fiberoapi.GetOApi(oapi, "/greeting/:name", 
    func(c *fiber.Ctx, input GetInput) (GetOutput, GetError) {
        return GetOutput{Message: "Hello " + input.Name}, GetError{}
    }, 
    fiberoapi.OpenAPIOptions{
        OperationID: "get-greeting",
        Tags:        []string{"greeting"},
        Summary:     "Get a personalized greeting",
    })
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

fiberoapi.PostOApi(oapi, "/users", 
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

fiberoapi.PutOApi(oapi, "/users/:id", 
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

fiberoapi.DeleteOApi(oapi, "/users/:id", 
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

- **GET**: `fiberoapi.GetOApi()` - Retrieve resources with path/query parameters
- **POST**: `fiberoapi.PostOApi()` - Create resources with JSON body + optional path parameters
- **PUT**: `fiberoapi.PutOApi()` - Update resources with path parameters + JSON body
- **DELETE**: `fiberoapi.DeleteOApi()` - Delete resources with path parameters + optional query parameters

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

## Example

See the `_examples/` folder for a complete usage example.

## Documentation

When `EnableOpenAPIDocs` is set to `true` (default), the library automatically sets up:

- **Swagger UI**: Available at the configured docs path (default: `/docs`)
- **OpenAPI JSON**: Available at the configured JSON path (default: `/openapi.json`)
- **Automatic Schema Generation**: Input and output types are automatically converted to OpenAPI schemas
- **Components Section**: All schemas are properly organized in the `components/schemas` section

No manual setup required! Just visit `http://localhost:3000/docs` to see your API documentation.

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

If you're migrating from a previous version that used `SetupDocs()`, you can:

1. **Recommended**: Use the new configuration system:
```go
// Old way
oapi := fiberoapi.New(app)
oapi.SetupDocs()

// New way
oapi := fiberoapi.New(app) // Uses defaults, docs auto-configured
```

2. **Backward compatibility**: `SetupDocs()` is still available but deprecated.

### New HTTP Methods

Version 2.0+ includes full CRUD support:

```go
// All methods now available:
fiberoapi.GetOApi(oapi, "/users/:id", handler, options)      // ✅ Available
fiberoapi.PostOApi(oapi, "/users", handler, options)        // ✅ Available  
fiberoapi.PutOApi(oapi, "/users/:id", handler, options)     // ✅ New in v2.0
fiberoapi.DeleteOApi(oapi, "/users/:id", handler, options)  // ✅ New in v2.0
```
