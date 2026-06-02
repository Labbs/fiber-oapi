package main

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	fiberoapi "github.com/labbs/fiber-oapi/v3"
)

// ErrorResponse is the shared shape every custom error in this app emits.
// Declaring it once means the OpenAPI spec gets a single component schema
// shared by every status code, and every response is consistent for clients.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Type    string `json:"type"`
}

// Error implements the standard error interface so handlers can return
// *ErrorResponse via a (Output, error) signature. It is optional — handlers
// can also be declared as func(...) (Output, *ErrorResponse) and skip this.
func (e *ErrorResponse) Error() string { return e.Message }

// Factory helpers — declare each error once, reuse across routes. The exact
// instance passed to OpenAPIOptions.Errors becomes the example shown in the
// spec, so write a representative payload.
func UserAlreadyExists(name string) *ErrorResponse {
	return &ErrorResponse{
		Code:    409,
		Message: fmt.Sprintf("user %q already exists", name),
		Type:    "Conflict",
		Details: "Pick a different username.",
	}
}

func UserNotFound(name string) *ErrorResponse {
	return &ErrorResponse{
		Code:    404,
		Message: fmt.Sprintf("user %q not found", name),
		Type:    "NotFound",
	}
}

type CreateUserInput struct {
	Name      string `uri:"name" validate:"required,min=2"`
	RequestID string `header:"x-request-id" validate:"omitempty"`
}

type CreateUserOutput struct {
	Message string `json:"message"`
}

func main() {
	app := fiber.New()

	oapi := fiberoapi.New(app, fiberoapi.Config{
		EnableValidation:   true,
		EnableOpenAPIDocs:  true,
		OpenAPIDocsPath:    "/documentation",                           // Custom docs path
		OpenAPIJSONPath:    "/api-spec.json",                           // Custom spec path
		OpenAPIYamlPath:    "/api-spec.yaml",                           // Custom YAML spec path
		OpenAPITitle:       "Simple Example API",                       // Spec title
		OpenAPIDescription: "A minimal fiber-oapi demo for the README", // Spec description
		OpenAPIVersion:     "0.1.0",
		// Tell the library to use *our* ErrorResponse shape for every error it
		// emits internally (validation, parse, auth, 404 / 405). Pass an empty
		// instance — the lib fills the Code / Message / Type / Details fields
		// from each error category. With this set the spec stops showing the
		// envelope-shaped 400/422/404 and instead documents ErrorResponse
		// everywhere, matching the per-route errors we declare below.
		DefaultErrorShape: &ErrorResponse{},
	})

	// The handler returns (Output, error). Returning a *ErrorResponse picks the
	// matching status code; returning nil emits the 200 response.
	fiberoapi.Post(oapi, "/users/:name", func(c fiber.Ctx, input CreateUserInput) (CreateUserOutput, error) {
		switch input.Name {
		case "admin":
			return CreateUserOutput{}, UserAlreadyExists(input.Name)
		case "ghost":
			return CreateUserOutput{}, UserNotFound(input.Name)
		}
		return CreateUserOutput{Message: fmt.Sprintf("user %q created", input.Name)}, nil
	}, fiberoapi.OpenAPIOptions{
		OperationID: "create-user",
		Tags:        []string{"users"},
		Summary:     "Create a new user",
		Description: "Demonstrates two custom error paths declared in the spec.",
		// Each entry below produces its own response in the generated spec:
		// 409 with this example payload, 404 with that one, etc. The schema
		// is shared because both factories return *ErrorResponse.
		Errors: []any{
			UserAlreadyExists("admin"),
			UserNotFound("ghost"),
		},
	})

	oapi.UseNotFoundHandler()

	fmt.Println("🚀 :3000  docs: /docs  spec: /openapi.json")
	app.Listen(":3000")
}
