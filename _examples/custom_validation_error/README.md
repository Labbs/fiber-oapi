# Custom Validation Error Handler Example

This example demonstrates how to use a custom validation error handler in fiber-oapi to return your own error structure when validation fails.

## Problem

By default, fiber-oapi returns a standard `ErrorResponse` structure when validation fails:

```json
{
  "code": 400,
  "details": "validation error message",
  "type": "validation_error"
}
```

However, you might want all errors in your API to follow the same structure, including validation errors.

## Solution

Use the `ValidationErrorHandler` field in the `Config` to provide a custom function that handles validation errors:

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    EnableValidation: true,
    ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
        // Return your custom error structure
        return c.Status(fiber.StatusBadRequest).JSON(CustomErrorResponse{
            Success: false,
            Message: err.Error(),
            Code:    "VALIDATION_ERROR",
        })
    },
})
```

## Running the Example

```bash
go run main.go
```

## Testing

Try sending an invalid request:

```bash
# Missing required fields
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{}'

# Response:
# {
#   "success": false,
#   "message": "Key: 'CreateUserInput.Name' Error:Field validation for 'Name' failed on the 'required' tag...",
#   "code": "VALIDATION_ERROR"
# }
```

```bash
# Invalid email format
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John",
    "email": "invalid-email",
    "age": 25
  }'

# Response:
# {
#   "success": false,
#   "message": "Key: 'CreateUserInput.Email' Error:Field validation for 'Email' failed on the 'email' tag",
#   "code": "VALIDATION_ERROR"
# }
```

```bash
# Valid request
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe",
    "email": "john@example.com",
    "age": 25
  }'

# Response:
# {
#   "id": 1,
#   "name": "John Doe",
#   "email": "john@example.com",
#   "age": 25,
#   "message": "User created successfully"
# }
```

## Advanced Usage

You can also parse the validation error to extract detailed information:

```go
ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
    // Parse validator errors for more details
    if validationErrs, ok := err.(validator.ValidationErrors); ok {
        errors := make([]map[string]string, 0)
        for _, fieldErr := range validationErrs {
            errors = append(errors, map[string]string{
                "field":   fieldErr.Field(),
                "tag":     fieldErr.Tag(),
                "value":   fmt.Sprintf("%v", fieldErr.Value()),
                "message": fieldErr.Error(),
            })
        }
        return c.Status(fiber.StatusBadRequest).JSON(map[string]interface{}{
            "success": false,
            "errors":  errors,
        })
    }

    // Fallback for other errors
    return c.Status(fiber.StatusBadRequest).JSON(CustomErrorResponse{
        Success: false,
        Message: err.Error(),
        Code:    "VALIDATION_ERROR",
    })
},
```
