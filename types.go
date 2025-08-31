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

// HandlerFunc represents a handler function with typed input and output
type HandlerFunc[TInput any, TOutput any, TError any] func(c *fiber.Ctx, input TInput) (TOutput, TError)

// PathInfo represents information about a path parameter
type PathInfo struct {
	Name   string
	IsPath bool
	Index  int // Position in the path for validation
}

// Config represents configuration for the OApi wrapper
type Config struct {
	EnableValidation  bool   // Enable request validation (default: true)
	EnableOpenAPIDocs bool   // Enable automatic docs setup (default: true)
	OpenAPIDocsPath   string // Path for documentation UI (default: "/docs")
	OpenAPIJSONPath   string // Path for OpenAPI JSON spec (default: "/openapi.json")
}

// OpenAPIOptions represents options for OpenAPI operations
type OpenAPIOptions struct {
	OperationID string                   `json:"operationId,omitempty"`
	Tags        []string                 `json:"tags,omitempty"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	Parameters  []map[string]interface{} `json:"parameters,omitempty"`
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
