package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	fiberoapi "github.com/labbs/fiber-oapi"
)

// CustomErrorResponse is the structure for custom validation error responses
type CustomErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// CreateUserInput is the input structure for creating a user
type CreateUserInput struct {
	Name  string `json:"name" validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"required,min=18,max=100"`
}

// CreateUserOutput is the output structure for creating a user
type CreateUserOutput struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Age     int    `json:"age"`
	Message string `json:"message"`
}

func main() {
	app := fiber.New()

	// Configure fiber-oapi with a custom validation error handler
	oapi := fiberoapi.New(app, fiberoapi.Config{
		EnableValidation:  true,
		EnableOpenAPIDocs: true,
		// Define your custom handler for validation errors
		ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
			// You can parse the validation error to extract more details
			// or simply return your custom structure
			return c.Status(fiber.StatusBadRequest).JSON(CustomErrorResponse{
				Success: false,
				Message: err.Error(),
				Code:    "VALIDATION_ERROR",
			})
		},
	})

	// Define your endpoint
	fiberoapi.Post[CreateUserInput, CreateUserOutput, struct{}](
		oapi,
		"/users",
		func(c *fiber.Ctx, input CreateUserInput) (CreateUserOutput, struct{}) {
			// User creation logic goes here
			return CreateUserOutput{
				ID:      1,
				Name:    input.Name,
				Email:   input.Email,
				Age:     input.Age,
				Message: "User created successfully",
			}, struct{}{}
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Create a new user",
			Description: "Creates a new user with validation",
			Tags:        []string{"users"},
		},
	)

	log.Println("Server starting on :3000")
	log.Println("OpenAPI docs available at http://localhost:3000/docs")
	log.Fatal(oapi.Listen(":3000"))
}
