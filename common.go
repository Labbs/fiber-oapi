package fiberoapi

import (
	"bytes"
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
	// rawBody is the index of the []byte field tagged body:"raw", which
	// receives the request body as-is, or -1.
	rawBody int
	// paramOnly lists the fields bound from the path, the query or the
	// headers that declare no body tag (see paramOnlyFields).
	paramOnly []int
}

var shapeCache sync.Map // map[reflect.Type]*inputShape

// shapeFor returns the cached inputShape for T, computing it lazily once.
func shapeFor[TInput any]() *inputShape {
	var zero TInput
	t := reflect.TypeOf(zero)
	if cached, ok := shapeCache.Load(t); ok {
		return cached.(*inputShape)
	}
	rawBody, _ := rawBodyField(t) // an invalid declaration panics in Method
	s := &inputShape{
		isStruct:  t != nil && dereferenceType(t).Kind() == reflect.Struct,
		rawBody:   rawBody,
		paramOnly: paramOnlyFields(t),
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

	// Parse body for POST/PUT/PATCH methods only if there's content.
	// The body is parsed first so that path, query and header values take
	// priority over any field the JSON decoder may have populated. The decoder
	// matches keys case-insensitively, so a field tagged uri:"id" without
	// json:"-" would otherwise be overwritten by {"id": "..."} in the body.
	method := c.Method()
	if (method == "POST" || method == "PUT" || method == "PATCH") && shape.rawBody >= 0 {
		// The body is taken as-is, whatever its Content-Type. Fiber reuses its
		// buffers, so it is copied. An empty body leaves the field nil, so that
		// validate:"required" rejects it.
		if body := c.Body(); len(body) > 0 {
			setRawBody(&input, shape.rawBody, bytes.Clone(body))
		}
	} else if method == "POST" || method == "PUT" || method == "PATCH" {
		bodyLength := len(c.Body())
		contentType := c.Get("Content-Type")

		if bodyLength > 0 || strings.Contains(contentType, "application/json") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
			if err := c.Bind().Body(&input); err != nil {
				// For POST without a body, tolerate the parsing failure.
				if bodyLength == 0 && method == "POST" {
					// no-op
				} else if wrapped := wrapJSONTypeError(err); wrapped != nil {
					// Type-mismatch errors get a friendly Error() while staying
					// errors.As-recoverable down to the original *json.UnmarshalTypeError.
					return input, wrapped
				} else {
					return input, err
				}
			}
		}
		// The decoder may have filled a path, query or header field from a
		// body key with the same name. Binding below only overwrites the
		// parameters present in the request, so reset those fields first: a
		// header absent from the request must not keep a value from the body.
		resetFields(&input, shape.paramOnly)
	}

	if shape.isStruct {
		if err := c.Bind().URI(&input); err != nil {
			return input, err
		}
		if err := c.Bind().Query(&input); err != nil {
			return input, err
		}
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
	if app.config.EnableAuthorization && options != nil && !securityDisabled(options) {
		// The request was authenticated before being read (see
		// authenticateHandler); only roles and resource permissions, which
		// need the input, are left.
		authCtx, _ := c.Locals("auth").(*AuthContext)
		if err := authorize(c, authCtx, input, app.config.AuthService, options.RequiredRoles, options.RequireAllRoles); err != nil {
			return input, err
		}
	}

	return input, nil
}

// securityDisabled reports whether the route opted out of authentication
// with Security: "disabled".
func securityDisabled(options *OpenAPIOptions) bool {
	securityValue, ok := options.Security.(string)
	return ok && securityValue == "disabled"
}

// rawBodyField returns the index of the field tagged body:"raw" in the input
// struct t, or -1 when there is none. The field must be a []byte, and there
// can be only one.
func rawBodyField(t reflect.Type) (int, error) {
	if t == nil {
		return -1, nil
	}
	t = dereferenceType(t)
	if t.Kind() != reflect.Struct {
		return -1, nil
	}
	index := -1
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("body") != "raw" {
			continue
		}
		if !field.IsExported() || field.Type != bytesType {
			return -1, fmt.Errorf("field %s has body:\"raw\" but is not an exported []byte", field.Name)
		}
		if index >= 0 {
			return -1, fmt.Errorf("fields %s and %s both have body:\"raw\"", t.Field(index).Name, field.Name)
		}
		index = i
	}
	return index, nil
}

var bytesType = reflect.TypeFor[[]byte]()

// bodyTags are the struct tags through which a body decoder fills a field.
var bodyTags = []string{"json", "form", "xml", "msgpack", "cbor"}

// paramOnlyFields returns the indexes of the fields of the input struct t
// that are bound from the path, the query or the headers and declare no body
// tag (a "-" tag counts as none). Body decoders still fill such a field when a
// key matches its name, so parseInput resets them after decoding the body. A
// field that declares both, e.g. query:"name" json:"name", accepts either.
func paramOnlyFields(t reflect.Type) []int {
	if t == nil {
		return nil
	}
	t = dereferenceType(t)
	if t.Kind() != reflect.Struct {
		return nil
	}
	var fields []int
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Tag.Get("uri") == "" && field.Tag.Get("query") == "" && field.Tag.Get("header") == "" {
			continue
		}
		fromBody := false
		for _, tag := range bodyTags {
			if name, _, _ := strings.Cut(field.Tag.Get(tag), ","); name != "" && name != "-" {
				fromBody = true
				break
			}
		}
		if !fromBody {
			fields = append(fields, i)
		}
	}
	return fields
}

// resetFields sets the given fields of input to their zero value.
func resetFields[TInput any](input *TInput, fields []int) {
	if len(fields) == 0 {
		return
	}
	v := reflect.ValueOf(input).Elem()
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	for _, i := range fields {
		v.Field(i).SetZero()
	}
}

// setRawBody stores body in the raw body field of input, allocating the
// struct when the input is a nil pointer.
func setRawBody[TInput any](input *TInput, field int, body []byte) {
	v := reflect.ValueOf(input).Elem()
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	v.Field(field).SetBytes(body)
}

// handleCustomError emits the error instance a handler returned through its
// TError, with the status that instance carries.
//
// The status resolution is extractErrorStatusCode — the very same one the spec
// generator uses for the entries of OpenAPIOptions.Errors. It used to be
// duplicated here as a StatusCode/Code field lookup, which silently left out the
// HTTPStatus() method: an error type that carries its code somewhere the
// reflection cannot reach (nested in a slice of per-field errors, say) and
// exposes it through the interface was documented under its real status and then
// served as 500. Runtime and spec must not disagree about the same instance.
//
// The Content-Type is application/json, unless the instance implements
// ContentTyper.
func handleCustomError(c fiber.Ctx, cfg *Config, customErr interface{}) error {
	// A typed nil pointer carries nothing to serialize, so it cannot say anything
	// about its own status either.
	if v := reflect.ValueOf(customErr); v.Kind() == reflect.Ptr && v.IsNil() {
		if cfg.ProblemDetails {
			return writeProblem(c, cfg.problemFor(errorCategory{Code: 500, Type: errTypeInternal, Message: "Internal server error"}, nil))
		}
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	statusCode := extractErrorStatusCode(customErr)

	// Return the error as JSON
	if err := c.Status(statusCode).JSON(customErr, contentTypeOf(customErr)); err != nil {
		if cfg.ProblemDetails {
			if fallbackErr := writeProblem(c, cfg.problemFor(errorCategory{Code: 500, Type: errTypeSerialization, Message: "Failed to serialize error response"}, nil)); fallbackErr != nil {
				return err
			}
			return nil
		}
		if fallbackErr := c.Status(500).JSON(fiber.Map{"error": "Failed to serialize error response"}); fallbackErr != nil {
			// Both serializations failed, return original error to Fiber
			return err
		}
		return nil
	}
	return nil
}

// Utility to check if a value is zero. Handles three edge cases beyond the
// straightforward reflect.ValueOf().IsZero():
//   - untyped nil (e.g. the handler signature has TError = error and the handler
//     returned nil) — reflect.ValueOf(nil) returns an invalid Value whose
//     IsZero() would panic;
//   - typed nil pointer (zero value of *Foo) — IsZero correctly reports true;
//   - zero struct (Foo{}) — IsZero reports true.
func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return true
	}
	return rv.IsZero()
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
