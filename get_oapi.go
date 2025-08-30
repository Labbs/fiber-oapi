package fiberoapi

import (
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v2"
)

// GetOApi defines a GET operation for the OpenAPI documentation
func GetOApi[TInput any, TOutput any, TError any](
	o *OApiApp,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
) {

	// Validate path parameters with the input struct
	if err := validatePathParams[TInput](path); err != nil {
		panic(fmt.Sprintf("Path validation failed for %s: %v", path, err))
	}

	// Register the operation for OpenAPI documentation with type information
	var inputZero TInput
	var outputZero TOutput
	var errorZero TError

	o.operations = append(o.operations, OpenAPIOperation{
		Method:     "GET",
		Path:       path,
		Options:    options,
		InputType:  reflect.TypeOf(inputZero),
		OutputType: reflect.TypeOf(outputZero),
		ErrorType:  reflect.TypeOf(errorZero),
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

	o.Get(path, fiberHandler)
}
