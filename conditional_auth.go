package fiberoapi

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ConditionalAuthMiddleware crée un middleware qui s'applique seulement aux routes spécifiées
func ConditionalAuthMiddleware(authMiddleware fiber.Handler, excludePaths ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Debug: afficher le chemin testé
		fmt.Printf("🔍 Checking path: %s\n", path)

		// Vérifier si le chemin est dans la liste d'exclusion
		for _, excludePath := range excludePaths {
			if path == excludePath || strings.HasPrefix(path, excludePath) {
				fmt.Printf("✅ Path %s excluded from auth\n", path)
				return c.Next() // Passer au suivant sans authentification
			}
		}

		fmt.Printf("🔒 Path %s requires auth\n", path)
		// Appliquer l'authentification pour tous les autres chemins
		return authMiddleware(c)
	}
}

// SmartAuthMiddleware crée un middleware qui exclut automatiquement les routes de documentation
func SmartAuthMiddleware(authService AuthorizationService, config Config) fiber.Handler {
	authMiddleware := BearerTokenMiddleware(authService)

	// Chemins à exclure de l'authentification
	excludePaths := []string{
		config.OpenAPIDocsPath, // /docs
		config.OpenAPIJSONPath, // /openapi.json
	}

	return ConditionalAuthMiddleware(authMiddleware, excludePaths...)
}
