package fiberoapi

import (
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
func parseInput[TInput any](app *OApiApp, c *fiber.Ctx, path string) (TInput, error) {
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

	// Parse body for POST/PUT methods
	method := c.Method()
	if method == "POST" || method == "PUT" || method == "PATCH" {
		err = c.BodyParser(&input)
		if err != nil {
			return input, err
		}
	}

	// Validate input if enabled in configuration
	if app.Config().EnableValidation {
		err = validate.Struct(input)
		if err != nil {
			return input, err
		}
	}

	return input, nil
}

// Function to handle custom errors
func handleCustomError(c *fiber.Ctx, customErr interface{}) error {
	// Use reflection to extract error information
	errValue := reflect.ValueOf(customErr)
	// errType := reflect.TypeOf(customErr)

	// Assume your error struct has fields like StatusCode and Message
	statusCode := 500 // default
	if field := errValue.FieldByName("StatusCode"); field.IsValid() && field.CanInt() {
		statusCode = int(field.Int())
	}

	// Return the error as JSON
	return c.Status(statusCode).JSON(customErr)
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

// Helper function to set field values with type conversion
func setFieldValue(fieldValue reflect.Value, value string) error {
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
	if inputType != nil && inputType.Kind() == reflect.Ptr {
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
