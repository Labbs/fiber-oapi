package main

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	fiberoapi "github.com/labbs/fiber-oapi"
)

type ContextRequest struct {
	RequestId string `path:"requestId" validate:"required,min=2"`
}

type GetInput struct {
	Name string `path:"name" validate:"required,min=2"`
}

type GetOutput struct {
	Message string `json:"message"`
}

type GetError struct {
	Code    int    `json:"code"`
	Details string `json:"details"`
	Type    string `json:"type"`
}

// Structures pour POST et PUT
type CreateUserInput struct {
	Username string `json:"username" validate:"required,min=3,max=20,alphanum"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"required,min=13,max=120"`
	Bio      string `json:"bio" validate:"omitempty,max=500"`

	RequestContext ContextRequest `json:"requestContext" validate:"required,dive"`
}

type CreateUserOutput struct {
	ID       string `json:"id"`
	Message  string `json:"message"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserError struct {
	Code    int    `json:"code"`
	Details string `json:"details"`
	Type    string `json:"type"`
}

// Structures pour PUT
type UpdateUserInput struct {
	ID       string `path:"id" validate:"required"`
	Username string `json:"username" validate:"omitempty,min=3,max=20,alphanum"`
	Email    string `json:"email" validate:"omitempty,email"`
	Age      int    `json:"age" validate:"omitempty,min=13,max=120"`
	Bio      string `json:"bio" validate:"omitempty,max=500"`
}

type UpdateUserOutput struct {
	ID       string `json:"id"`
	Message  string `json:"message"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Updated  bool   `json:"updated"`
}

// Structures pour DELETE
type DeleteUserInput struct {
	ID string `path:"id" validate:"required"`
}

type DeleteUserOutput struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Deleted bool   `json:"deleted"`
}

func main() {
	app := fiber.New()

	// Example 1: Using custom configuration
	customConfig := fiberoapi.Config{
		EnableValidation:  true,
		EnableOpenAPIDocs: true,
		OpenAPIDocsPath:   "/documentation", // Custom docs path
		OpenAPIJSONPath:   "/api-spec.json", // Custom spec path
		OpenAPIYamlPath:   "/api-spec.yaml", // Custom YAML spec path
	}
	appOApi := fiberoapi.New(app, customConfig)
	v1 := fiberoapi.Group(appOApi, "/api/v1")

	// Example 2: Using default configuration (commented out)
	// appOApi := fiberoapi.New(app) // Will use defaults: /docs and /openapi.json

	// Route GET avec validation
	fiberoapi.Get(appOApi, "/greeting/:name", func(c *fiber.Ctx, input GetInput) (GetOutput, GetError) {
		name := input.Name
		return GetOutput{Message: "Hello " + name}, GetError{}
	}, fiberoapi.OpenAPIOptions{
		OperationID: "get-greeting",
		Tags:        []string{"greeting"},
		Summary:     "Get a personalized greeting",
		Description: "Returns a greeting message for the provided name",
	})

	// POST route with complex validation
	fiberoapi.Post(appOApi, "/users", func(c *fiber.Ctx, input CreateUserInput) (CreateUserOutput, CreateUserError) {
		// Simulate user creation
		if input.Username == "admin" {
			return CreateUserOutput{}, CreateUserError{
				Code:    403,
				Details: "Username 'admin' is reserved",
				Type:    "forbidden_username",
			}
		}

		// Success
		return CreateUserOutput{
			ID:       "user_" + input.Username,
			Message:  "User created successfully",
			Username: input.Username,
			Email:    input.Email,
		}, CreateUserError{}
	}, fiberoapi.OpenAPIOptions{
		OperationID: "create-user",
		Tags:        []string{"users"},
		Summary:     "Create a new user",
		Description: "Creates a new user with validation for username, email, and age",
	})

	// PUT route for updating user
	fiberoapi.Put(appOApi, "/users/:id", func(c *fiber.Ctx, input UpdateUserInput) (UpdateUserOutput, CreateUserError) {
		// Simulate user update
		if input.ID == "nonexistent" {
			return UpdateUserOutput{}, CreateUserError{
				Code:    404,
				Details: "User not found",
				Type:    "not_found",
			}
		}

		// Success
		return UpdateUserOutput{
			ID:       input.ID,
			Message:  "User updated successfully",
			Username: input.Username,
			Email:    input.Email,
			Updated:  true,
		}, CreateUserError{}
	}, fiberoapi.OpenAPIOptions{
		OperationID: "update-user",
		Tags:        []string{"users"},
		Summary:     "Update an existing user",
		Description: "Updates an existing user with new information",
	})

	// DELETE route for removing user
	fiberoapi.Delete(appOApi, "/users/:id", func(c *fiber.Ctx, input DeleteUserInput) (DeleteUserOutput, CreateUserError) {
		// Simulate user deletion
		if input.ID == "protected" {
			return DeleteUserOutput{}, CreateUserError{
				Code:    403,
				Details: "Cannot delete protected user",
				Type:    "forbidden",
			}
		}

		// Success
		return DeleteUserOutput{
			ID:      input.ID,
			Message: "User deleted successfully",
			Deleted: true,
		}, CreateUserError{}
	}, fiberoapi.OpenAPIOptions{
		OperationID: "delete-user",
		Tags:        []string{"users"},
		Summary:     "Delete a user",
		Description: "Removes a user from the system",
	})

	// GET route with group
	fiberoapi.Get(v1, "/greeting/:name", func(c *fiber.Ctx, input GetInput) (GetOutput, GetError) {
		name := input.Name
		return GetOutput{Message: "Hello " + name}, GetError{}
	}, fiberoapi.OpenAPIOptions{
		OperationID: "get-greeting-group",
		Tags:        []string{"greeting", "group"},
		Summary:     "Get a personalized greeting with group",
		Description: "Returns a greeting message for the provided name",
	})

	// No need to manually call SetupDocs() - it's automatic when EnableOpenAPIDocs is true!

	fmt.Println("🚀 Server starting on :3000")
	fmt.Println("📚 API Documentation available at: http://localhost:3000/documentation")
	fmt.Println("📄 OpenAPI JSON available at: http://localhost:3000/api-spec.json")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  - Validation enabled: %t\n", appOApi.Config().EnableValidation)
	fmt.Printf("  - OpenAPI docs enabled: %t\n", appOApi.Config().EnableOpenAPIDocs)
	fmt.Printf("  - Docs path: %s\n", appOApi.Config().OpenAPIDocsPath)
	fmt.Printf("  - JSON spec path: %s\n", appOApi.Config().OpenAPIJSONPath)

	app.Listen(":3000")
}
