package main

import (
	"log"

	fiberoapi "github.com/labbs/fiber-oapi"
	"github.com/gofiber/fiber/v2"
)

// CustomErrorResponse représente votre structure d'erreur personnalisée
type CustomErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// CreateUserInput représente l'entrée pour créer un utilisateur
type CreateUserInput struct {
	Name  string `json:"name" validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"required,min=18,max=100"`
}

// CreateUserOutput représente la sortie
type CreateUserOutput struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Age     int    `json:"age"`
	Message string `json:"message"`
}

func main() {
	app := fiber.New()

	// Configurer fiber-oapi avec un gestionnaire d'erreur de validation personnalisé
	oapi := fiberoapi.New(app, fiberoapi.Config{
		EnableValidation:  true,
		EnableOpenAPIDocs: true,
		// Définir votre handler personnalisé pour les erreurs de validation
		ValidationErrorHandler: func(c *fiber.Ctx, err error) error {
			// Vous pouvez parser l'erreur de validation pour extraire plus de détails
			// ou simplement retourner votre structure personnalisée
			return c.Status(fiber.StatusBadRequest).JSON(CustomErrorResponse{
				Success: false,
				Message: err.Error(),
				Code:    "VALIDATION_ERROR",
			})
		},
	})

	// Définir votre endpoint
	fiberoapi.Post[CreateUserInput, CreateUserOutput, struct{}](
		oapi,
		"/users",
		func(c *fiber.Ctx, input CreateUserInput) (CreateUserOutput, struct{}) {
			// Logique de création d'utilisateur
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
