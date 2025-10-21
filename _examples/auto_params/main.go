package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	fiberoapi "github.com/labbs/fiber-oapi"
)

type SearchInput struct {
	Name     string  `path:"name" validate:"required"`
	Email    string  `query:"email" validate:"omitempty,email"`
	Age      int     `query:"age" validate:"omitempty,min=0,max=120"`
	Active   bool    `query:"active"`
	MinPrice float64 `query:"minPrice" validate:"omitempty,min=0"`
	
	// Pointer types - automatically optional and nullable
	Category    *string  `query:"category"`
	MaxResults  *int     `query:"maxResults"`
	IncludeInactive *bool `query:"includeInactive"`
}

type SearchOutput struct {
	Results []User `json:"results"`
	Total   int    `json:"total"`
}

type User struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Age      int     `json:"age"`
	Active   bool    `json:"active"`
	Price    float64 `json:"price"`
	Category *string `json:"category,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	app := fiber.New()
	oapi := fiberoapi.New(app)

	// Endpoint avec génération automatique des paramètres
	fiberoapi.Get(oapi, "/users/:name", func(c *fiber.Ctx, input SearchInput) (SearchOutput, ErrorResponse) {
		// Simulate search results
		user := User{
			ID:       1,
			Name:     input.Name,
			Email:    input.Email,
			Age:      input.Age,
			Active:   input.Active,
			Price:    input.MinPrice,
			Category: input.Category, // Pointer field - can be nil
		}

		return SearchOutput{
			Results: []User{user},
			Total:   1,
		}, ErrorResponse{}
	}, fiberoapi.OpenAPIOptions{
		OperationID: "searchUsers",
		Summary:     "Search users with auto-generated parameters",
		Description: "This endpoint demonstrates automatic parameter generation from struct tags",
		Tags:        []string{"users"},
	})

	// Setup documentation
	oapi.SetupDocs()

	// Generate and print OpenAPI spec
	spec := oapi.GenerateOpenAPISpec()

	// Extract and print parameters for our endpoint
	if paths, ok := spec["paths"].(map[string]interface{}); ok {
		if userPath, ok := paths["/users/{name}"].(map[string]interface{}); ok {
			if getOp, ok := userPath["get"].(map[string]interface{}); ok {
				if parameters, ok := getOp["parameters"].([]map[string]interface{}); ok {
					fmt.Println("🎉 Paramètres auto-générés :")
					for _, param := range parameters {
						paramJSON, _ := json.MarshalIndent(param, "  ", "  ")
						fmt.Printf("  %s\n", string(paramJSON))
					}
				}
			}
		}
	}

	fmt.Println("\n📖 Documentation disponible sur http://localhost:3000/docs")
	fmt.Println("📊 Spec OpenAPI JSON sur http://localhost:3000/openapi.json")
	fmt.Println("🧪 Test de l'endpoint : http://localhost:3000/users/john?email=john@example.com&age=25&active=true&minPrice=10.5&category=electronics&maxResults=10")
	fmt.Println("🔧 Paramètres optionnels (pointeurs) : category, maxResults, includeInactive")

	log.Fatal(app.Listen(":3000"))
}
