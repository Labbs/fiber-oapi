package fiberoapi

import (
	"reflect"

	"github.com/gofiber/fiber/v2"
)

// PostOApi defines a POST operation for the OpenAPI documentation
func PostOApi[TInput any, TOutput any, TError any](
	o *OApiApp,
	path string,
	handler HandlerFunc[TInput, TOutput, TError],
	options OpenAPIOptions,
) {
	// Register the operation for OpenAPI documentation with type information
	var inputZero TInput
	var outputZero TOutput
	var errorZero TError

	o.operations = append(o.operations, OpenAPIOperation{
		Method:     "POST",
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

	o.Post(path, fiberHandler)
}
