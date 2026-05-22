package fiberoapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// Global validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// inputShape caches per-type metadata used at request time so we avoid
// re-introspecting the input struct on every request. Built once via sync.OnceValue.
type inputShape struct {
	// isStruct is true when the request input is a (possibly pointer-to) struct,
	// i.e. eligible for URI/Query/Header binding.
	isStruct bool
}

var shapeCache sync.Map // map[reflect.Type]*inputShape

// shapeFor returns the cached inputShape for T, computing it lazily once.
func shapeFor[TInput any]() *inputShape {
	var zero TInput
	t := reflect.TypeOf(zero)
	if cached, ok := shapeCache.Load(t); ok {
		return cached.(*inputShape)
	}
	s := &inputShape{
		isStruct: t != nil && dereferenceType(t).Kind() == reflect.Struct,
	}
	actual, _ := shapeCache.LoadOrStore(t, s)
	return actual.(*inputShape)
}

// parseInput parses the input from the request, delegating URI / Query / Header /
// Body extraction to Fiber's Bind (which caches its own per-type schema). Per-type
// shape metadata is cached locally to avoid re-running reflection on every request.
func parseInput[TInput any](app *OApiApp, c fiber.Ctx, path string, options *OpenAPIOptions) (TInput, error) {
	var input TInput

	shape := shapeFor[TInput]()

	if shape.isStruct {
		if err := c.Bind().URI(&input); err != nil {
			return input, err
		}
		if err := c.Bind().Query(&input); err != nil {
			return input, err
		}
	}

	// Parse body for POST/PUT/PATCH methods only if there's content.
	// Body is parsed before headers so that header values take priority over any
	// field that the JSON decoder may have populated (e.g. when a header-bound
	// field is also sent in the body without a json:"-" tag).
	method := c.Method()
	if method == "POST" || method == "PUT" || method == "PATCH" {
		bodyLength := len(c.Body())
		contentType := c.Get("Content-Type")

		if bodyLength > 0 || strings.Contains(contentType, "application/json") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
			if err := c.Bind().Body(&input); err != nil {
				// For POST without a body, tolerate the parsing failure
				if bodyLength == 0 && method == "POST" {
					// no-op
				} else if friendly := translateJSONError(err); friendly != nil {
					return input, friendly
				} else {
					return input, err
				}
			}
		}
	}

	if shape.isStruct {
		if err := c.Bind().Header(&input); err != nil {
			return input, err
		}
	}

	// Validate input if enabled in configuration
	if app.Config().EnableValidation {
		if err := validate.Struct(input); err != nil {
			return input, err
		}
	}

	// Validate authorization if enabled in configuration and not disabled for this route
	if app.Config().EnableAuthorization && options != nil {
		if securityValue, ok := options.Security.(string); ok && securityValue == "disabled" {
			// Skip authorization for this route
		} else {
			cfg := app.Config()
			if routeSecurity, ok := options.Security.([]map[string][]string); ok && len(routeSecurity) > 0 {
				cfg.DefaultSecurity = routeSecurity
			}
			if err := validateAuthorization(c, input, cfg.AuthService, &cfg, options.RequiredRoles, options.RequireAllRoles); err != nil {
				return input, err
			}
		}
	}

	return input, nil
}

// translateJSONError converts low-level json decoder errors into a stable,
// user-facing validation message. Returns nil if err is not a JSON type mismatch.
func translateJSONError(err error) error {
	ute, ok := errors.AsType[*json.UnmarshalTypeError](err)
	if !ok {
		return nil
	}
	fieldName := ute.Field
	if i := strings.LastIndex(fieldName, "."); i >= 0 {
		fieldName = fieldName[i+1:]
	}
	if fieldName == "" {
		return fmt.Errorf("invalid JSON: expected %s but got %s", ute.Type.String(), ute.Value)
	}
	return fmt.Errorf("invalid type for field '%s': expected %s but got %s",
		fieldName, ute.Type.String(), ute.Value)
}

// Function to handle custom errors
func handleCustomError(c fiber.Ctx, customErr interface{}) error {
	// Use reflection to extract error information
	errValue := reflect.ValueOf(customErr)

	// Handle pointers - get the element they point to
	if errValue.Kind() == reflect.Ptr {
		if errValue.IsNil() {
			return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
		}
		errValue = errValue.Elem()
	}

	// Assume your error struct has fields like StatusCode and Message
	statusCode := 500 // default
	if errValue.Kind() == reflect.Struct {
		if field := errValue.FieldByName("StatusCode"); field.IsValid() && field.CanInt() {
			statusCode = int(field.Int())
		} else if field := errValue.FieldByName("Code"); field.IsValid() && field.CanInt() {
			statusCode = int(field.Int())
		}
	}

	// Return the error as JSON
	if err := c.Status(statusCode).JSON(customErr); err != nil {
		if fallbackErr := c.Status(500).JSON(fiber.Map{"error": "Failed to serialize error response"}); fallbackErr != nil {
			// Both serializations failed, return original error to Fiber
			return err
		}
		return nil
	}
	return nil
}

// Utility to check if a value is zero
func isZero(v interface{}) bool {
	return reflect.ValueOf(v).IsZero()
}

// Validate that struct parameters match the path
func validatePathParams[T any](path string) error {
	var zero T
	inputType := reflect.TypeOf(zero)

	// If the type is a pointer, get the element type
	if inputType != nil && isPointerType(inputType) {
		inputType = inputType.Elem()
	}

	// If inputType is nil or not a struct, skip validation
	if inputType == nil || inputType.Kind() != reflect.Struct {
		return nil
	}

	// Extract Fiber path parameters (:param format)
	pathParams := extractFiberPathParams(path)

	// Check that each field with "path" tag exists in the path
	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		if uriTag := field.Tag.Get("uri"); uriTag != "" {
			if !contains(pathParams, uriTag) {
				return fmt.Errorf("field %s has uri tag '%s' but parameter is not in path %s", field.Name, uriTag, path)
			}
		}
	}

	return nil
}

// Extract Fiber path parameters (:param or {param})
func extractFiberPathParams(path string) []string {
	var params []string
	parts := strings.Split(path, "/")

	for _, part := range parts {
		// Fiber uses :param or {param}
		if strings.HasPrefix(part, ":") {
			params = append(params, part[1:])
		} else if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			params = append(params, part[1:len(part)-1])
		}
	}

	return params
}

// Helper to check if a slice contains an element
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// extractParametersFromStruct extracts OpenAPI parameters from struct tags
func extractParametersFromStruct(inputType reflect.Type) []map[string]interface{} {
	var parameters []map[string]interface{}

	if inputType == nil {
		return parameters
	}

	// Handle pointer types
	inputType = dereferenceType(inputType)

	// Only process struct types
	if inputType.Kind() != reflect.Struct {
		return parameters
	}

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip fields hidden from OpenAPI documentation
		if field.Tag.Get("openapi") == "-" {
			continue
		}

		// Process path parameters (Fiber v3 binding tag is "uri")
		if uriTag := field.Tag.Get("uri"); uriTag != "" {
			// Path parameters are always required per OpenAPI 3.0.
			param := map[string]interface{}{
				"name":        uriTag,
				"in":          "path",
				"required":    true,
				"description": getFieldDescription(field, "Path parameter"),
				"schema":      getSchemaForType(field.Type),
			}
			parameters = append(parameters, param)
		}

		// Process query parameters
		if queryTag := field.Tag.Get("query"); queryTag != "" {
			// Query parameters use specialized logic based on type and validation tags
			required := isQueryFieldRequired(field)
			param := map[string]interface{}{
				"name":        queryTag,
				"in":          "query",
				"required":    required,
				"description": getFieldDescription(field, "Query parameter"),
				"schema":      getSchemaForType(field.Type),
			}
			parameters = append(parameters, param)
		}

		// Process header parameters
		if headerTag := field.Tag.Get("header"); headerTag != "" {
			// OpenAPI 3.0 specifies that header parameters named "Accept", "Content-Type",
			// or "Authorization" are ignored by tooling when in: header. Skip these reserved names.
			switch strings.ToLower(headerTag) {
			case "accept", "content-type", "authorization":
				continue
			}

			required := isQueryFieldRequired(field)
			param := map[string]interface{}{
				"name":        headerTag,
				"in":          "header",
				"required":    required,
				"description": getFieldDescription(field, "Header parameter"),
				"schema":      getSchemaForType(field.Type),
			}
			parameters = append(parameters, param)
		}
	}

	return parameters
}

// getFieldDescription extracts description from struct field
func getFieldDescription(field reflect.StructField, defaultDesc string) string {
	// Try to get description from json tag comment or other sources
	if desc := field.Tag.Get("description"); desc != "" {
		return desc
	}
	if desc := field.Tag.Get("doc"); desc != "" {
		return desc
	}
	// Use field name as fallback
	return fmt.Sprintf("%s: %s", defaultDesc, field.Name)
}

// isPointerType checks if a reflect.Type is a pointer type
func isPointerType(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr
}

// isPointerField checks if a reflect.StructField is a pointer type
func isPointerField(field reflect.StructField) bool {
	return isPointerType(field.Type)
}

// dereferenceType removes pointer indirection from a type
func dereferenceType(t reflect.Type) reflect.Type {
	if isPointerType(t) {
		return t.Elem()
	}
	return t
}

// isQueryFieldRequired checks if a query parameter field is required
// Query parameters have different logic than path parameters:
// - Path parameters are always required (handled separately)
// - Pointer types (*string, *int, etc.) are optional by default
// - Non-pointer types are optional by default unless explicitly marked as required
// - Fields with "omitempty" are optional
// - Fields with "required" are required
func isQueryFieldRequired(field reflect.StructField) bool {
	validateTag := field.Tag.Get("validate")

	// If it's a pointer type, it's optional by default (unless explicitly required)
	if isPointerField(field) {
		return strings.Contains(validateTag, "required")
	}

	// For non-pointer types in query parameters:
	// - If has omitempty, it's optional
	if strings.Contains(validateTag, "omitempty") {
		return false
	}

	// Check for explicit required validation
	return strings.Contains(validateTag, "required")
}

// getSchemaForType returns OpenAPI schema for a Go type
func getSchemaForType(t reflect.Type) map[string]interface{} {
	schema := make(map[string]interface{})

	// Handle pointer types - preserve original to detect nullability, then dereference for type checking
	originalType := t
	t = dereferenceType(t)

	switch t.Kind() {
	case reflect.String:
		schema["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema["type"] = "integer"
		if t.Kind() == reflect.Int64 {
			schema["format"] = "int64"
		} else {
			schema["format"] = "int32"
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema["type"] = "integer"
		schema["minimum"] = 0
		if t.Kind() == reflect.Uint64 {
			schema["format"] = "int64"
		} else {
			schema["format"] = "int32"
		}
	case reflect.Float32:
		schema["type"] = "number"
		schema["format"] = "float"
	case reflect.Float64:
		schema["type"] = "number"
		schema["format"] = "double"
	case reflect.Bool:
		schema["type"] = "boolean"
	default:
		schema["type"] = "string"
	}

	// If the original type was a pointer, indicate it's nullable
	if isPointerType(originalType) {
		schema["nullable"] = true
	}

	return schema
}

// detectTypeMismatchFromBody attempts to identify which field caused a JSON type mismatch
// by parsing the request body and comparing against the expected struct type
func detectTypeMismatchFromBody(body []byte, input interface{}) (fieldName, expectedType, actualType string) {
	// Parse the JSON body into a map to see what was actually sent
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return "", "", ""
	}

	// Get the struct type using reflection
	inputValue := reflect.ValueOf(input)
	if inputValue.Kind() == reflect.Ptr {
		inputValue = inputValue.Elem()
	}
	inputType := inputValue.Type()

	if inputType.Kind() != reflect.Struct {
		return "", "", ""
	}

	// Iterate through struct fields to find the mismatch
	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)

		// Get the JSON tag name (default to field name if no tag)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = field.Name
		} else {
			// Remove omitempty and other options from the tag
			jsonTag = strings.Split(jsonTag, ",")[0]
		}

		// Check if this field is in the body map
		if actualValue, exists := bodyMap[jsonTag]; exists {
			expectedFieldType := dereferenceType(field.Type)
			actualValueType := getJSONValueType(actualValue)

			// Check for type mismatch
			mismatch := false
			expectedTypeName := ""

			switch expectedFieldType.Kind() {
			case reflect.String:
				expectedTypeName = "string"
				if actualValueType != "string" {
					mismatch = true
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				expectedTypeName = "integer"
				if actualValueType != "number" {
					mismatch = true
				}
			case reflect.Float32, reflect.Float64:
				expectedTypeName = "number"
				if actualValueType != "number" {
					mismatch = true
				}
			case reflect.Bool:
				expectedTypeName = "boolean"
				if actualValueType != "boolean" {
					mismatch = true
				}
			case reflect.Slice, reflect.Array:
				expectedTypeName = fmt.Sprintf("[]%s", dereferenceType(expectedFieldType.Elem()).Kind())
				if actualValueType != "array" {
					mismatch = true
				}
			case reflect.Map:
				expectedTypeName = "map"
				if actualValueType != "object" {
					mismatch = true
				}
			case reflect.Struct:
				expectedTypeName = "object"
				if actualValueType != "object" {
					mismatch = true
				}
			}

			if mismatch {
				return field.Name, expectedTypeName, actualValueType
			}
		}
	}

	return "", "", ""
}

// getJSONValueType returns the JSON type name for a value parsed from JSON
func getJSONValueType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// mergeParameters merges auto-generated parameters with manually defined ones
// Manual parameters take precedence over auto-generated ones with the same name
func mergeParameters(autoParams []map[string]interface{}, manualParams []map[string]interface{}) []map[string]interface{} {
	// Create a map to track manual parameter names
	manualParamNames := make(map[string]bool)
	for _, param := range manualParams {
		if name, ok := param["name"].(string); ok {
			manualParamNames[name] = true
		}
	}

	// Start with manual parameters (they have precedence)
	result := make([]map[string]interface{}, len(manualParams))
	copy(result, manualParams)

	// Add auto-generated parameters that don't conflict with manual ones
	for _, autoParam := range autoParams {
		if name, ok := autoParam["name"].(string); ok {
			if !manualParamNames[name] {
				result = append(result, autoParam)
			}
		}
	}

	return result
}
