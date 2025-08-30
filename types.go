package fiberoapi

import "github.com/gofiber/fiber/v2"

// OApiApp wraps fiber.App with OpenAPI capabilities
type OApiApp struct {
	*fiber.App
	operations []OpenAPIOperation
	config     Config
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
