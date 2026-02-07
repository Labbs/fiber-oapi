package main

import (
	"fmt"
	"time"

	fiberoapi "github.com/Labbs/fiber-oapi"
	"github.com/gofiber/fiber/v2"
)

// Authentication service with role management.
// Implements AuthorizationService, BasicAuthValidator, APIKeyValidator, and AWSSignatureValidator.
type ExampleAuthService struct{}

func (s *ExampleAuthService) ValidateToken(token string) (*fiberoapi.AuthContext, error) {
	// Simulation de différents utilisateurs selon le token
	switch token {
	case "admin-token":
		return &fiberoapi.AuthContext{
			UserID: "admin-123",
			Roles:  []string{"admin", "user"},
			Scopes: []string{"read", "write", "delete", "share"},
			Claims: map[string]interface{}{
				"sub": "admin-123",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		}, nil
	case "editor-token":
		return &fiberoapi.AuthContext{
			UserID: "editor-456",
			Roles:  []string{"editor", "user"},
			Scopes: []string{"read", "write", "share"},
			Claims: map[string]interface{}{
				"sub": "editor-456",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		}, nil
	case "user-token":
		return &fiberoapi.AuthContext{
			UserID: "user-789",
			Roles:  []string{"user"},
			Scopes: []string{"read", "write"},
			Claims: map[string]interface{}{
				"sub": "user-789",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		}, nil
	case "readonly-token":
		return &fiberoapi.AuthContext{
			UserID: "readonly-123",
			Roles:  []string{"user"},
			Scopes: []string{"read"},
			Claims: map[string]interface{}{
				"sub": "readonly-123",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid token: %s", token)
	}
}

func (s *ExampleAuthService) HasRole(ctx *fiberoapi.AuthContext, role string) bool {
	for _, r := range ctx.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (s *ExampleAuthService) HasScope(ctx *fiberoapi.AuthContext, scope string) bool {
	for _, sc := range ctx.Scopes {
		if sc == scope {
			return true
		}
	}
	return false
}

func (s *ExampleAuthService) CanAccessResource(ctx *fiberoapi.AuthContext, resourceType, resourceID, action string) (bool, error) {
	// Admins can do everything
	if s.HasRole(ctx, "admin") {
		return true, nil
	}

	// Simple logic for the example
	if resourceType == "document" {
		if action == "delete" {
			return s.HasRole(ctx, "admin"), nil
		}
		if action == "write" {
			return s.HasScope(ctx, "write"), nil
		}
		if action == "read" {
			return s.HasScope(ctx, "read"), nil
		}
	}

	return false, nil
}

func (s *ExampleAuthService) GetUserPermissions(ctx *fiberoapi.AuthContext, resourceType, resourceID string) (*fiberoapi.ResourcePermission, error) {
	actions := []string{}
	if s.HasRole(ctx, "admin") {
		actions = []string{"read", "write", "delete", "share"}
	} else if s.HasScope(ctx, "write") {
		actions = []string{"read", "write"}
	} else if s.HasScope(ctx, "read") {
		actions = []string{"read"}
	}

	return &fiberoapi.ResourcePermission{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Actions:      actions,
	}, nil
}

// ValidateBasicAuth implements BasicAuthValidator for HTTP Basic authentication (curl --user).
func (s *ExampleAuthService) ValidateBasicAuth(username, password string) (*fiberoapi.AuthContext, error) {
	// Example credentials
	users := map[string]string{
		"admin": "admin-pass",
		"user":  "user-pass",
	}

	expectedPassword, exists := users[username]
	if !exists || password != expectedPassword {
		return nil, fmt.Errorf("invalid credentials for user: %s", username)
	}

	roles := []string{"user"}
	scopes := []string{"read", "write"}
	if username == "admin" {
		roles = []string{"admin", "user"}
		scopes = []string{"read", "write", "delete", "share"}
	}

	return &fiberoapi.AuthContext{
		UserID: username,
		Roles:  roles,
		Scopes: scopes,
	}, nil
}

// ValidateAPIKey implements APIKeyValidator for API Key authentication.
func (s *ExampleAuthService) ValidateAPIKey(key string, location string, paramName string) (*fiberoapi.AuthContext, error) {
	validKeys := map[string]string{
		"my-secret-api-key": "apikey-user-1",
		"another-api-key":   "apikey-user-2",
	}

	userID, exists := validKeys[key]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	return &fiberoapi.AuthContext{
		UserID: userID,
		Roles:  []string{"user"},
		Scopes: []string{"read"},
	}, nil
}

// ValidateAWSSignature implements AWSSignatureValidator for AWS SigV4 authentication.
func (s *ExampleAuthService) ValidateAWSSignature(params *fiberoapi.AWSSignatureParams) (*fiberoapi.AuthContext, error) {
	// In a real implementation, you would verify the HMAC-SHA256 signature
	// using the secret key associated with the AccessKeyID.
	validKeys := map[string]bool{
		"AKIAIOSFODNN7EXAMPLE": true,
	}

	if !validKeys[params.AccessKeyID] {
		return nil, fmt.Errorf("invalid access key: %s", params.AccessKeyID)
	}

	return &fiberoapi.AuthContext{
		UserID: "aws-service-" + params.AccessKeyID,
		Roles:  []string{"service"},
		Scopes: []string{"read", "write"},
		Claims: map[string]interface{}{
			"region":  params.Region,
			"service": params.Service,
		},
	}, nil
}

type CreateUserRequest struct {
	Name string `json:"name" validate:"required,min=2,max=50"`
}

type CreateUserResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type DocumentRequest struct {
	DocumentID string `path:"documentId" validate:"required"`
}

type UpdateDocumentRequest struct {
	DocumentID string `path:"documentId" validate:"required"`
	Title      string `json:"title" validate:"required,min=1,max=100"`
	Content    string `json:"content" validate:"required,min=1"`
}

type DocumentResponse struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Author  string `json:"author"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type DocumentShareResponse struct {
	ShareLink string `json:"share_link"`
}

type DocumentDeleteResponse struct {
	Success bool `json:"success"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

func main() {
	app := fiber.New()

	authService := &ExampleAuthService{}

	config := fiberoapi.Config{
		EnableValidation:    true,
		EnableOpenAPIDocs:   true,
		EnableAuthorization: true,
		AuthService:         authService,
		SecuritySchemes: map[string]fiberoapi.SecurityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "JWT Bearer token authentication",
			},
			"basicAuth": {
				Type:        "http",
				Scheme:      "basic",
				Description: "HTTP Basic authentication (curl --user user:pass)",
			},
			"apiKeyAuth": {
				Type:        "apiKey",
				In:          "header",
				Name:        "X-API-Key",
				Description: "API Key authentication via header",
			},
			"awsSigV4": {
				Type:        "http",
				Scheme:      "AWS4-HMAC-SHA256",
				Description: "AWS Signature V4 authentication",
			},
		},
		// Any of these schemes can be used (OR semantics)
		DefaultSecurity: []map[string][]string{
			{"bearerAuth": {}},
			{"basicAuth": {}},
			{"apiKeyAuth": {}},
			{"awsSigV4": {}},
		},
	}

	oapi := fiberoapi.New(app, config)

	// ====== ROUTES PUBLIQUES ======
	fiberoapi.Get(oapi, "/health",
		func(c *fiber.Ctx, input struct{}) (map[string]string, *fiberoapi.ErrorResponse) {
			return map[string]string{
				"status":  "ok",
				"service": "fiber-oapi auth example",
				"version": "1.0.0",
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Health check",
			Description: "Endpoint public pour vérifier l'état du service - utilise map[string]string",
			Tags:        []string{"health"},
			Security:    "disabled",
		})

	// ====== ROUTES AVEC AUTHENTIFICATION SIMPLE ======
	fiberoapi.Get(oapi, "/me",
		func(c *fiber.Ctx, input struct{}) (CreateUserResponse, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)
			return CreateUserResponse{
				ID:   1,
				Name: "Current User",
				Role: fmt.Sprintf("roles: %v, scopes: %v", authCtx.Roles, authCtx.Scopes),
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Get current user info",
			Description: "Récupère les informations de l'utilisateur authentifié",
			Tags:        []string{"user"},
		})

	// Test endpoint with map[string]interface{} response
	fiberoapi.Get(oapi, "/status",
		func(c *fiber.Ctx, input struct{}) (map[string]interface{}, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)
			return map[string]interface{}{
				"user_id":   authCtx.UserID,
				"roles":     authCtx.Roles,
				"scopes":    authCtx.Scopes,
				"timestamp": time.Now().Unix(),
				"active":    true,
				"metadata": map[string]string{
					"source": "fiber-oapi",
					"env":    "development",
				},
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Get extended status",
			Description: "Récupère un statut étendu avec map[string]interface{} - test pour RedDoc",
			Tags:        []string{"user", "status"},
		})

	// ====== ROUTES AVEC CONTRÔLE DE RÔLES ======

	// Route pour les utilisateurs (rôle minimum)
	fiberoapi.Get(oapi, "/documents/:documentId",
		func(c *fiber.Ctx, input DocumentRequest) (DocumentResponse, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)

			// Vérification manuelle des rôles et scopes
			if !authService.HasRole(authCtx, "user") {
				return DocumentResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'user' role",
					Type:    "authorization_error",
				}
			}
			if !authService.HasScope(authCtx, "read") {
				return DocumentResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'read' scope",
					Type:    "authorization_error",
				}
			}

			fmt.Printf("📖 User %s (roles: %v) accessing document %s\n", authCtx.UserID, authCtx.Roles, input.DocumentID)

			return DocumentResponse{
				ID:      input.DocumentID,
				Title:   "Sample Document",
				Content: "This document content is protected",
				Author:  authCtx.UserID,
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Get document",
			Description: "Récupère un document. Nécessite le rôle 'user' et scope 'read'",
			Tags:        []string{"documents"},
		})

	// Route pour les éditeurs (peuvent modifier)
	fiberoapi.Put(oapi, "/documents/:documentId",
		func(c *fiber.Ctx, input UpdateDocumentRequest) (DocumentResponse, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)

			// Vérification manuelle des rôles et scopes
			if !authService.HasRole(authCtx, "user") {
				return DocumentResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'user' role",
					Type:    "authorization_error",
				}
			}
			if !authService.HasScope(authCtx, "write") {
				return DocumentResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'write' scope",
					Type:    "authorization_error",
				}
			}

			fmt.Printf("✏️  User %s (scopes: %v) updating document %s\n", authCtx.UserID, authCtx.Scopes, input.DocumentID)

			return DocumentResponse{
				ID:      input.DocumentID,
				Title:   input.Title,
				Content: input.Content,
				Author:  authCtx.UserID,
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Update document",
			Description: "Met à jour un document. Nécessite le rôle 'user' et scope 'write'",
			Tags:        []string{"documents"},
		})

	// Route pour partager (éditeurs et admins)
	fiberoapi.Post(oapi, "/documents/:documentId/share",
		func(c *fiber.Ctx, input DocumentRequest) (DocumentShareResponse, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)

			// Vérification du scope share
			if !authService.HasScope(authCtx, "share") {
				return DocumentShareResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'share' scope",
					Type:    "authorization_error",
				}
			}

			fmt.Printf("🔗 User %s sharing document %s\n", authCtx.UserID, input.DocumentID)

			return DocumentShareResponse{
				ShareLink: fmt.Sprintf("https://example.com/shared/%s", input.DocumentID),
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Share document",
			Description: "Partage un document. Nécessite le scope 'share'",
			Tags:        []string{"documents", "sharing"},
		})

	// Route réservée aux administrateurs
	fiberoapi.Delete(oapi, "/documents/:documentId",
		func(c *fiber.Ctx, input DocumentRequest) (DocumentDeleteResponse, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)

			// Vérification du rôle admin et scope delete
			if !authService.HasRole(authCtx, "admin") {
				return DocumentDeleteResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'admin' role",
					Type:    "authorization_error",
				}
			}
			if !authService.HasScope(authCtx, "delete") {
				return DocumentDeleteResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'delete' scope",
					Type:    "authorization_error",
				}
			}

			fmt.Printf("🗑️  Admin %s deleting document %s\n", authCtx.UserID, input.DocumentID)

			return DocumentDeleteResponse{
				Success: true,
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Delete document",
			Description: "Supprime un document. Réservé aux administrateurs",
			Tags:        []string{"documents", "admin"},
		})

	// Route de création d'utilisateur (admin seulement)
	fiberoapi.Post(oapi, "/users",
		func(c *fiber.Ctx, input CreateUserRequest) (CreateUserResponse, *fiberoapi.ErrorResponse) {
			authCtx, _ := fiberoapi.GetAuthContext(c)

			// Vérification du rôle admin et scope write
			if !authService.HasRole(authCtx, "admin") {
				return CreateUserResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'admin' role",
					Type:    "authorization_error",
				}
			}
			if !authService.HasScope(authCtx, "write") {
				return CreateUserResponse{}, &fiberoapi.ErrorResponse{
					Code:    403,
					Details: "Access denied: requires 'write' scope",
					Type:    "authorization_error",
				}
			}

			fmt.Printf("👤 Admin %s creating user: %s\n", authCtx.UserID, input.Name)

			return CreateUserResponse{
				ID:   123,
				Name: input.Name,
				Role: "user",
			}, nil
		},
		fiberoapi.OpenAPIOptions{
			Summary:     "Create user",
			Description: "Crée un nouvel utilisateur. Réservé aux administrateurs",
			Tags:        []string{"users", "admin"},
		})

	fmt.Println("🚀 Serveur avec authentification et rôles démarré sur port 3002")
	fmt.Println("📚 Documentation: http://localhost:3002/docs")
	fmt.Println("📄 OpenAPI JSON: http://localhost:3002/openapi.json")
	fmt.Println("")
	fmt.Println("🔑 Méthodes d'authentification supportées:")
	fmt.Println("   Bearer Token:  Authorization: Bearer <token>")
	fmt.Println("   Basic Auth:    Authorization: Basic base64(user:pass)  (curl --user user:pass)")
	fmt.Println("   API Key:       X-API-Key: <key>")
	fmt.Println("   AWS SigV4:     Authorization: AWS4-HMAC-SHA256 Credential=...")
	fmt.Println("")
	fmt.Println("🔑 Tokens de test disponibles:")
	fmt.Println("   admin-token     -> rôles: [admin, user], scopes: [read, write, delete, share]")
	fmt.Println("   editor-token    -> rôles: [editor, user], scopes: [read, write, share]")
	fmt.Println("   user-token      -> rôles: [user], scopes: [read, write]")
	fmt.Println("   readonly-token  -> rôles: [user], scopes: [read]")
	fmt.Println("")
	fmt.Println("🔑 Comptes Basic Auth:")
	fmt.Println("   admin:admin-pass  -> rôles: [admin, user]")
	fmt.Println("   user:user-pass    -> rôles: [user]")
	fmt.Println("")
	fmt.Println("🔑 API Keys:")
	fmt.Println("   my-secret-api-key  -> read only")
	fmt.Println("   another-api-key    -> read only")
	fmt.Println("")
	fmt.Println("🧪 Tests suggérés:")
	fmt.Println("   # Bearer Token")
	fmt.Println("   curl -H 'Authorization: Bearer admin-token' http://localhost:3002/me")
	fmt.Println("")
	fmt.Println("   # Basic Auth (curl --user)")
	fmt.Println("   curl --user admin:admin-pass http://localhost:3002/me")
	fmt.Println("")
	fmt.Println("   # API Key")
	fmt.Println("   curl -H 'X-API-Key: my-secret-api-key' http://localhost:3002/documents/doc-1")
	fmt.Println("")
	fmt.Println("   # AWS SigV4")
	fmt.Println("   curl -H 'Authorization: AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20250101/us-east-1/execute-api/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123' http://localhost:3002/me")
	fmt.Println("")
	fmt.Println("   # Public endpoint")
	fmt.Println("   curl http://localhost:3002/health")

	app.Listen(":3002")
}
