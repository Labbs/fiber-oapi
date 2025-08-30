package fiberoapi

type Config struct {
	EnableValidation  bool   // Enable or disable input validation
	EnableOpenAPIDocs bool   // Enable or disable OpenAPI documentation generation
	OpenAPIDocsPath   string // Path to serve OpenAPI documentation (e.g., "/docs")
	OpenAPIJSONPath   string // Path to serve OpenAPI JSON spec (e.g., "/openapi.json")
}
