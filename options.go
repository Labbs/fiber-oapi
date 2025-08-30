package fiberoapi

import "reflect"

type OpenAPIOperation struct {
	Method     string         // HTTP method: "GET", "POST", etc.
	Path       string         // Route path: "/greeting/{name}"
	Options    OpenAPIOptions // OpenAPI options and metadata
	InputType  reflect.Type   // Type of the input struct for schema generation
	OutputType reflect.Type   // Type of the output struct for schema generation
	ErrorType  reflect.Type   // Type of the error struct for schema generation
}

type OpenAPIOptions struct {
	OperationID string                     `json:"operationId,omitempty"`
	Summary     string                     `json:"summary,omitempty"`
	Description string                     `json:"description,omitempty"`
	Tags        []string                   `json:"tags,omitempty"`
	Security    []map[string][]string      `json:"security,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
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
