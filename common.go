package fiberoapi

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Global validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Function to parse input from the request
// parseInput parses the input from the request
func parseInput[TInput any](app *OApiApp, c *fiber.Ctx, path string, options *OpenAPIOptions) (TInput, error) {
	var input TInput

	// Parse path parameters if needed
	err := parsePathParams(c, &input)
	if err != nil {
		return input, err
	}

	// Parse query parameters
	err = parseQueryParams(c, &input)
	if err != nil {
		return input, err
	}

	// Parse header parameters
	err = parseHeaderParams(c, &input)
	if err != nil {
		return input, err
	}

	// Parse body for POST/PUT methods only if there's content
	method := c.Method()
	if method == "POST" || method == "PUT" || method == "PATCH" {
		// Check if there's content in the body
		bodyLength := len(c.Body())
		contentType := c.Get("Content-Type")

		// Parse the body if there's content OR if it's a POST/PUT/PATCH with specified Content-Type
		if bodyLength > 0 || strings.Contains(contentType, "application/json") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
			err = c.BodyParser(&input)
			if err != nil {
				// For POST requests without a body, ignore the parsing error
				if bodyLength == 0 && method == "POST" {
					// It's OK, the POST has no body - ignore the error
				} else {
					// Transform JSON unmarshal type errors into readable validation errors
					errMsg := err.Error()

					// Check if error message contains unmarshal type error pattern
					if strings.Contains(errMsg, "json: cannot unmarshal") && strings.Contains(errMsg, "into Go struct field") {
						// Parse the error message to extract field name and type info
						// Format: "json: cannot unmarshal <type> into Go struct field <StructName>.<Field> of type <GoType>"
						parts := strings.Split(errMsg, "into Go struct field ")
						if len(parts) == 2 {
							afterField := parts[1]
							fieldParts := strings.Split(afterField, " of type ")
							if len(fieldParts) == 2 {
								// Extract field name (after the last dot)
								fullFieldName := fieldParts[0]
								fieldNameParts := strings.Split(fullFieldName, ".")
								fieldName := fieldNameParts[len(fieldNameParts)-1]

								// Extract expected type and trim whitespace
								expectedType := strings.TrimSpace(fieldParts[1])

								// Extract actual type from the first part and trim whitespace
								typePart := strings.TrimSpace(strings.TrimPrefix(parts[0], "json: cannot unmarshal "))

								return input, fmt.Errorf("invalid type for field '%s': expected %s but got %s",
									fieldName, expectedType, typePart)
							}
						}
					} else if strings.Contains(errMsg, "json: slice") || strings.Contains(errMsg, "json: map") {
						// Handle "json: slice unexpected end of JSON input" and similar errors
						// This happens when sending wrong type for slice/map fields
						// Try to identify which field caused the error by parsing the request body
						fieldName, expectedType, actualType := detectTypeMismatchFromBody(c.Body(), input)
						if fieldName != "" {
							return input, fmt.Errorf("invalid type for field '%s': expected %s but got %s",
								fieldName, expectedType, actualType)
						}
						// Fallback to generic message if we can't identify the field
						return input, fmt.Errorf("invalid JSON: expected array or object but got incompatible type")
					}

					// Return original error if no pattern matched
					return input, err
				}
			}
		}
	}

	// Validate input if enabled in configuration
	if app.Config().EnableValidation {
		err = validate.Struct(input)
		if err != nil {
			return input, err
		}
	}

	// Validate authorization if enabled in configuration and not disabled for this route
	if app.Config().EnableAuthorization && options != nil {
		// Check if security is explicitly disabled for this route
		if securityValue, ok := options.Security.(string); ok && securityValue == "disabled" {
			// Skip authorization for this route
		} else {
			err = validateAuthorization(c, input, app.Config().AuthService)
			if err != nil {
				return input, err
			}
		}
	}

	return input, nil
}

// Function to handle custom errors
func handleCustomError(c *fiber.Ctx, customErr interface{}) error {
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

// Parse path parameters
func parsePathParams(c *fiber.Ctx, input interface{}) error {
	inputValue := reflect.ValueOf(input).Elem()
	inputType := reflect.TypeOf(input).Elem()

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		if pathTag := field.Tag.Get("path"); pathTag != "" {
			paramValue := c.Params(pathTag)
			if paramValue != "" {
				fieldValue := inputValue.Field(i)
				if fieldValue.CanSet() && fieldValue.Kind() == reflect.String {
					fieldValue.SetString(paramValue)
				}
			}
		}
	}

	return nil
}

// Parse query parameters
func parseQueryParams(c *fiber.Ctx, input interface{}) error {
	inputValue := reflect.ValueOf(input).Elem()
	inputType := reflect.TypeOf(input).Elem()

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		if queryTag := field.Tag.Get("query"); queryTag != "" {
			queryValue := c.Query(queryTag)
			if queryValue != "" {
				fieldValue := inputValue.Field(i)
				if fieldValue.CanSet() {
					if err := setFieldValue(fieldValue, queryValue); err != nil {
						return fmt.Errorf("failed to parse query param %s: %w", queryTag, err)
					}
				}
			}
		}
	}

	return nil
}

// Parse header parameters
func parseHeaderParams(c *fiber.Ctx, input interface{}) error {
	inputValue := reflect.ValueOf(input).Elem()
	inputType := reflect.TypeOf(input).Elem()

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		if headerTag := field.Tag.Get("header"); headerTag != "" {
			headerValue := c.Get(headerTag)
			if headerValue != "" {
				fieldValue := inputValue.Field(i)
				if fieldValue.CanSet() {
					if err := setFieldValue(fieldValue, headerValue); err != nil {
						return fmt.Errorf("failed to parse header param %s: %w", headerTag, err)
					}
				}
			}
		}
	}

	return nil
}

// Helper function to set field values with type conversion
func setFieldValue(fieldValue reflect.Value, value string) error {
	// Handle pointer types: allocate and recurse into the pointed-to value
	if fieldValue.Kind() == reflect.Ptr {
		if fieldValue.IsNil() {
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
		}
		return setFieldValue(fieldValue.Elem(), value)
	}

	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intVal, err := strconv.ParseInt(value, 10, 64); err != nil {
			return err
		} else {
			fieldValue.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintVal, err := strconv.ParseUint(value, 10, 64); err != nil {
			return err
		} else {
			fieldValue.SetUint(uintVal)
		}
	case reflect.Float32, reflect.Float64:
		if floatVal, err := strconv.ParseFloat(value, fieldValue.Type().Bits()); err != nil {
			return err
		} else {
			fieldValue.SetFloat(floatVal)
		}
	case reflect.Bool:
		if boolVal, err := strconv.ParseBool(value); err != nil {
			return err
		} else {
			fieldValue.SetBool(boolVal)
		}
	default:
		return fmt.Errorf("unsupported field type: %s", fieldValue.Kind())
	}
	return nil
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

		if pathTag := field.Tag.Get("path"); pathTag != "" {
			if !contains(pathParams, pathTag) {
				return fmt.Errorf("field %s has path tag '%s' but parameter is not in path %s", field.Name, pathTag, path)
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

		// Process path parameters
		if pathTag := field.Tag.Get("path"); pathTag != "" {
			// Path parameters are always required regardless of type or validation tags.
			// This follows OpenAPI 3.0 specification where path parameters must be required,
			// and is enforced here by explicitly setting "required": true at line 289.
			param := map[string]interface{}{
				"name":        pathTag,
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
