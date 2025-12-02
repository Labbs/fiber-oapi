package fiberoapi

import (
	"reflect"

	"github.com/gofiber/fiber/v2"
)

// OApiRouter interface that both OApiApp and OApiGroup implement
type OApiRouter interface {
	GetApp() *OApiApp
	GetPrefix() string
}

// OApiApp wraps fiber.App with OpenAPI capabilities
type OApiApp struct {
	f          *fiber.App
	operations []OpenAPIOperation
	config     Config
}

// Implement OApiRouter interface for OApiApp
func (o *OApiApp) GetApp() *OApiApp {
	return o
}

func (o *OApiApp) GetPrefix() string {
	return ""
}

// Config returns the current configuration
func (o *OApiApp) Config() Config {
	return o.config
}

// Use adds middleware to the OApiApp
func (o *OApiApp) Use(middleware fiber.Handler) {
	o.f.Use(middleware)
}

// Listen starts the server on the given address
func (o *OApiApp) Listen(addr string) error {
	return o.f.Listen(addr)
}

// HandlerFunc represents a handler function with typed input and output
type HandlerFunc[TInput any, TOutput any, TError any] func(c *fiber.Ctx, input TInput) (TOutput, TError)

// PathInfo represents information about a path parameter
type PathInfo struct {
	Name   string
	IsPath bool
	Index  int // Position in the path for validation
}

// ValidationErrorHandler is a function type for handling validation errors
// It receives the fiber context and the validation error, and returns a fiber error response
type ValidationErrorHandler func(c *fiber.Ctx, err error) error

// Config represents configuration for the OApi wrapper
type Config struct {
	EnableValidation       bool                      // Enable request validation (default: true)
	EnableOpenAPIDocs      bool                      // Enable automatic docs setup (default: true)
	EnableAuthorization    bool                      // Enable authorization validation (default: false)
	OpenAPIDocsPath        string                    // Path for documentation UI (default: "/docs")
	OpenAPIJSONPath        string                    // Path for OpenAPI JSON spec (default: "/openapi.json")
	OpenAPIYamlPath        string                    // Path for OpenAPI YAML spec (default: "/openapi.yaml")
	AuthService            AuthorizationService      // Service for handling authentication and authorization
	SecuritySchemes        map[string]SecurityScheme // OpenAPI security schemes
	DefaultSecurity        []map[string][]string     // Default security requirements
	ValidationErrorHandler ValidationErrorHandler    // Custom handler for validation errors
}

// OpenAPIOptions represents options for OpenAPI operations
type OpenAPIOptions struct {
	OperationID         string                   `json:"operationId,omitempty"`
	Tags                []string                 `json:"tags,omitempty"`
	Summary             string                   `json:"summary,omitempty"`
	Description         string                   `json:"description,omitempty"`
	Parameters          []map[string]interface{} `json:"parameters,omitempty"`
	Security            interface{}              `json:"security,omitempty"` // Can be []map[string][]string or "disabled"
	RequiredPermissions []string                 `json:"-"`                  // Ex: ["document:read", "workspace:admin"]
	ResourceType        string                   `json:"-"`                  // Type de ressource concernée
}

// OpenAPIOperation represents a registered operation
type OpenAPIOperation struct {
	Method     string
	Path       string
	Options    OpenAPIOptions
	InputType  reflect.Type
	OutputType reflect.Type
	ErrorType  reflect.Type
}

type OpenAPIParameter struct {
	Name        string                 `json:"name"`
	In          string                 `json:"in"` // "path", "query", "header", "cookie"
	Required    bool                   `json:"required,omitempty"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
}

type OpenAPIResponse struct {
	Description string                 `json:"description"`
	Content     map[string]interface{} `json:"content,omitempty"`
}

type OpenAPIRequestBody struct {
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required,omitempty"`
	Content     map[string]interface{} `json:"content"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Details string `json:"details"`
	Type    string `json:"type"`
}
