package fiberoapi

import (
	"reflect"

	"github.com/gofiber/fiber/v2"
)

// PutOApi defines a PUT operation for the OpenAPI documentation
func PutOApi[TInput any, TOutput any, TError any](
	o *OApiApp,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
) {
	// Get type information for schema generation
	var inputZero TInput
	var outputZero TOutput
	var errorZero TError

	inputType := reflect.TypeOf(inputZero)
	outputType := reflect.TypeOf(outputZero)
	errorType := reflect.TypeOf(errorZero)

	// Register the operation for OpenAPI documentation
	o.operations = append(o.operations, OpenAPIOperation{
		Method:     "PUT",
		Path:       path,
		Options:    options,
		InputType:  inputType,
		OutputType: outputType,
		ErrorType:  errorType,
	})

	// Wrapper
	fiberHandler := func(c *fiber.Ctx) error {
		input, err := parseInput[TInput](o, c, path)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Validation failed",
				"details": err.Error(),
			})
		}

		output, customErr := handler(c, input)

		if !isZero(customErr) {
			return handleCustomError(c, customErr)
		}

		return c.JSON(output)
	}

	o.Put(path, fiberHandler)
}
