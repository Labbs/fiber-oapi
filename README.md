# Fiber OpenAPI

A Go library that extends Fiber to add automatic OpenAPI documentation generation with built-in validation.

## Features

- ✅ **GET and POST methods** with automatic validation
- ✅ **Powerful validation** via `github.com/go-playground/validator/v10`
- ✅ **Type safety** with Go generics
- ✅ **Custom error handling**
- ✅ **OpenAPI documentation generation**
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

- **Path parameters**: `path:"paramName"`
- **Query parameters**: `query:"paramName"`
- **JSON body**: `json:"fieldName"` (for POST/PUT)

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

No manual setup required! Just visit `http://localhost:3000/docs` to see your API documentation.
