package fiberoapi

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// Tests pour les cas limites et edge cases
func TestAutoParamsEdgeCases(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	t.Run("Empty struct with tags", func(t *testing.T) {
		type EmptyInput struct{}

		type EmptyOutput struct {
			Message string `json:"message"`
		}

		type EmptyError struct {
			Code int `json:"code"`
		}

		Get(oapi, "/empty", func(c *fiber.Ctx, input EmptyInput) (EmptyOutput, EmptyError) {
			return EmptyOutput{Message: "empty"}, EmptyError{}
		}, OpenAPIOptions{
			OperationID: "emptyTest",
		})

		spec := oapi.GenerateOpenAPISpec()
		paths := spec["paths"].(map[string]interface{})
		emptyPath := paths["/empty"].(map[string]interface{})
		getOp := emptyPath["get"].(map[string]interface{})

		// Should not have parameters
		_, hasParams := getOp["parameters"]
		assert.False(t, hasParams, "Empty struct should not generate parameters")
	})

	t.Run("Struct with unexported fields", func(t *testing.T) {
		type UnexportedFieldsInput struct {
			PublicField string `path:"public" validate:"required"`
			_           string `path:"private" validate:"required"` // This should be ignored
		}

		type UnexportedOutput struct {
			Message string `json:"message"`
		}

		type UnexportedError struct {
			Code int `json:"code"`
		}

		Get(oapi, "/unexported/:public", func(c *fiber.Ctx, input UnexportedFieldsInput) (UnexportedOutput, UnexportedError) {
			return UnexportedOutput{Message: input.PublicField}, UnexportedError{}
		}, OpenAPIOptions{
			OperationID: "unexportedTest",
		})

		spec := oapi.GenerateOpenAPISpec()
		paths := spec["paths"].(map[string]interface{})
		unexportedPath := paths["/unexported/{public}"].(map[string]interface{})
		getOp := unexportedPath["get"].(map[string]interface{})

		parameters := getOp["parameters"].([]map[string]interface{})
		// Should only have 1 parameter (the exported one)
		assert.Len(t, parameters, 1, "Should only have 1 parameter from exported field")

		param := parameters[0]
		assert.Equal(t, "public", param["name"], "Should only have the public parameter")
	})

	t.Run("Fields with both path and query tags", func(t *testing.T) {
		type BothTagsInput struct {
			// This is an invalid case but we should handle it gracefully
			Field string `path:"field" query:"field" validate:"required"`
		}

		type BothTagsOutput struct {
			Message string `json:"message"`
		}

		type BothTagsError struct {
			Code int `json:"code"`
		}

		Get(oapi, "/both/:field", func(c *fiber.Ctx, input BothTagsInput) (BothTagsOutput, BothTagsError) {
			return BothTagsOutput{Message: input.Field}, BothTagsError{}
		}, OpenAPIOptions{
			OperationID: "bothTagsTest",
		})

		spec := oapi.GenerateOpenAPISpec()
		paths := spec["paths"].(map[string]interface{})
		bothPath := paths["/both/{field}"].(map[string]interface{})
		getOp := bothPath["get"].(map[string]interface{})

		parameters := getOp["parameters"].([]map[string]interface{})
		// Should have 2 parameters (one path, one query) - this is expected behavior
		assert.Len(t, parameters, 2, "Should have 2 parameters when field has both tags")

		// Log the parameters for debugging
		for i, param := range parameters {
			t.Logf("Parameter %d: %+v", i, param)
		}

		paramMap := make(map[string]string)
		for _, param := range parameters {
			if name, ok := param["name"].(string); ok {
				if in, ok := param["in"].(string); ok {
					paramMap[name] = in
				}
			}
		}

		// Check that both types are present (they have the same name but different "in" values)
		hasPath := false
		hasQuery := false
		for _, param := range parameters {
			if name, ok := param["name"].(string); ok && name == "field" {
				if in, ok := param["in"].(string); ok {
					if in == "path" {
						hasPath = true
					} else if in == "query" {
						hasQuery = true
					}
				}
			}
		}

		assert.True(t, hasPath, "Should have path parameter")
		assert.True(t, hasQuery, "Should have query parameter")
	})

	t.Run("Complex validation tags", func(t *testing.T) {
		type ComplexValidationInput struct {
			Email    string `query:"email" validate:"required,email,max=100"`
			Age      int    `query:"age" validate:"required,min=18,max=99"`
			Optional string `query:"optional" validate:"omitempty,alphanum,min=3"`
		}

		type ComplexValidationOutput struct {
			Message string `json:"message"`
		}

		type ComplexValidationError struct {
			Code int `json:"code"`
		}

		Get(oapi, "/complex", func(c *fiber.Ctx, input ComplexValidationInput) (ComplexValidationOutput, ComplexValidationError) {
			return ComplexValidationOutput{Message: "valid"}, ComplexValidationError{}
		}, OpenAPIOptions{
			OperationID: "complexValidationTest",
		})

		spec := oapi.GenerateOpenAPISpec()
		paths := spec["paths"].(map[string]interface{})
		complexPath := paths["/complex"].(map[string]interface{})
		getOp := complexPath["get"].(map[string]interface{})

		parameters := getOp["parameters"].([]map[string]interface{})
		assert.Len(t, parameters, 3, "Should have 3 parameters")

		paramMap := make(map[string]map[string]interface{})
		for _, param := range parameters {
			if name, ok := param["name"].(string); ok {
				paramMap[name] = param
			}
		}

		// Check that required fields are correctly identified
		assert.Equal(t, true, paramMap["email"]["required"], "Email should be required")
		assert.Equal(t, true, paramMap["age"]["required"], "Age should be required")
		assert.Equal(t, false, paramMap["optional"]["required"], "Optional should not be required (has omitempty)")
	})

	t.Run("Pointer types", func(t *testing.T) {
		type PointerInput struct {
			Name  *string `query:"name"`
			Count *int    `query:"count"`
		}

		type PointerOutput struct {
			Message string `json:"message"`
		}

		type PointerError struct {
			Code int `json:"code"`
		}

		Get(oapi, "/pointers", func(c *fiber.Ctx, input PointerInput) (PointerOutput, PointerError) {
			return PointerOutput{Message: "pointer"}, PointerError{}
		}, OpenAPIOptions{
			OperationID: "pointerTest",
		})

		spec := oapi.GenerateOpenAPISpec()
		paths := spec["paths"].(map[string]interface{})
		pointerPath := paths["/pointers"].(map[string]interface{})
		getOp := pointerPath["get"].(map[string]interface{})

		parameters := getOp["parameters"].([]map[string]interface{})
		assert.Len(t, parameters, 2, "Should have 2 parameters")

		paramMap := make(map[string]map[string]interface{})
		for _, param := range parameters {
			if name, ok := param["name"].(string); ok {
				paramMap[name] = param
			}
		}

		// Check that pointer types are handled correctly
		nameSchema := paramMap["name"]["schema"].(map[string]interface{})
		countSchema := paramMap["count"]["schema"].(map[string]interface{})

		assert.Equal(t, "string", nameSchema["type"], "Pointer to string should be string type")
		assert.Equal(t, "integer", countSchema["type"], "Pointer to int should be integer type")
	})
}
