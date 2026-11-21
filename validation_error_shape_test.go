package fiberoapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envErr is the envelope a custom ValidationErrorHandler sends in the tests:
// a list of entries and a context, unlike the library's ErrorEnvelope.
type envErr struct {
	Errors  []envErrEntry `json:"errors"`
	Context envErrContext `json:"context"`
}

type envErrEntry struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Details string `json:"details"`
}

type envErrContext struct {
	RequestID string `json:"request_id,omitempty"`
}

// envErrHandler answers validation errors with envErr and status.
func envErrHandler(status int) ValidationErrorHandler {
	return func(c fiber.Ctx, err error) error {
		return c.Status(status).JSON(envErr{Errors: []envErrEntry{{Code: status, Type: "validation_error", Details: err.Error()}}})
	}
}

// validationResponse returns the response documented under status for the
// createUser operation of spec.
func validationResponse(t *testing.T, spec map[string]any, status string) map[string]any {
	t.Helper()
	post := spec["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)
	resp, ok := post["responses"].(map[string]any)[status].(map[string]any)
	require.True(t, ok, "no %s response documented", status)
	return resp["content"].(map[string]any)["application/json"].(map[string]any)
}

func registerShapeRoute(t *testing.T, cfg Config) (*fiber.App, *OApiApp) {
	t.Helper()
	app := fiber.New()
	oapi := New(app, cfg)
	Post(oapi, "/users/:name", func(c fiber.Ctx, in uniInput) (uniOutput, error) {
		return uniOutput{Message: "ok"}, nil
	}, OpenAPIOptions{OperationID: "createUser"})
	return app, oapi
}

func TestValidationErrorShape_specDocumentsTheShape(t *testing.T) {
	example := envErr{Errors: []envErrEntry{{Code: 422, Type: "validation_error", Details: "field 'name' must be at least 2"}}}
	cases := []struct {
		name        string
		shape       any
		wantExample any
	}{
		{"filled instance is the example", example, example},
		{"filled pointer is the example", &example, &example},
		{"zero instance is filled by reflection", envErr{}, envErr{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, oapi := registerShapeRoute(t, Config{
				ValidationErrorHandler: envErrHandler(422),
				ValidationErrorShape:   tc.shape,
			})
			spec := oapi.GenerateOpenAPISpec()

			media := validationResponse(t, spec, "422")
			assert.Equal(t, "#/components/schemas/envErr", media["schema"].(map[string]any)["$ref"])
			assert.Equal(t, tc.wantExample, media["example"])

			schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
			for _, name := range []string{"envErr", "envErrEntry", "envErrContext"} {
				assert.Contains(t, schemas, name, "the $ref must not dangle")
			}
		})
	}
}

func TestValidationErrorShape_zeroFlatShapeGetsTheValidationFields(t *testing.T) {
	_, oapi := registerShapeRoute(t, Config{ValidationErrorShape: &UniErr{}})

	media := validationResponse(t, oapi.GenerateOpenAPISpec(), "422")
	assert.Equal(t, "#/components/schemas/UniErr", media["schema"].(map[string]any)["$ref"])
	got, ok := media["example"].(*UniErr)
	require.True(t, ok, "example: %#v", media["example"])
	assert.Equal(t, 422, got.Code)
	assert.Equal(t, "validation_error", got.Type)
	assert.NotEmpty(t, got.Message)
}

func TestValidationErrorShape_followsValidationStatusCode(t *testing.T) {
	// Validation moved to 400 with the same shape as the other library errors:
	// one schema under 400, no 422 left.
	_, oapi := registerShapeRoute(t, Config{
		ValidationErrorHandler: envErrHandler(400),
		ValidationErrorShape:   envErr{},
		DefaultErrorShape:      envErr{},
		ValidationStatusCode:   400,
	})
	spec := oapi.GenerateOpenAPISpec()

	media := validationResponse(t, spec, "400")
	assert.Equal(t, "#/components/schemas/envErr", media["schema"].(map[string]any)["$ref"])
	post := spec["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)
	assert.NotContains(t, post["responses"], "422")
}

func TestValidationErrorShape_handlerKeepsPriorityAtRuntime(t *testing.T) {
	app, _ := registerShapeRoute(t, Config{
		ValidationErrorHandler: envErrHandler(422),
		ValidationErrorShape:   envErr{},
	})

	resp, err := app.Test(httptest.NewRequest("POST", "/users/a", strings.NewReader("")))
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var got envErr
	require.NoError(t, json.Unmarshal(raw, &got), "body: %s", raw)
	require.Len(t, got.Errors, 1)
	assert.Contains(t, got.Errors[0].Details, "Name", "the handler wrote the body")
}

func TestValidationErrorShape_libraryFillsTheShapeWithoutHandler(t *testing.T) {
	app, _ := registerShapeRoute(t, Config{ValidationErrorShape: &UniErr{}})

	resp, err := app.Test(httptest.NewRequest("POST", "/users/a", strings.NewReader("")))
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var got UniErr
	require.NoError(t, json.Unmarshal(raw, &got), "body: %s", raw)
	assert.Equal(t, 422, got.Code)
	assert.Equal(t, "validation_error", got.Type)
	assert.NotEmpty(t, got.Message)
}

func TestValidationErrorShape_parseErrorsKeepTheirShape(t *testing.T) {
	// Only validation errors take the shape: a type mismatch keeps
	// DefaultErrorShape at runtime and in the spec.
	app, oapi := registerShapeRoute(t, Config{
		ValidationErrorShape: envErr{},
		DefaultErrorShape:    &UniErr{},
	})

	media := validationResponse(t, oapi.GenerateOpenAPISpec(), "400")
	assert.Equal(t, "#/components/schemas/UniErr", media["schema"].(map[string]any)["$ref"])

	req := httptest.NewRequest("POST", "/users/alice", strings.NewReader(`{"age": "not a number"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var got UniErr
	require.NoError(t, json.Unmarshal(raw, &got), "body: %s", raw)
	assert.Equal(t, "type_error", got.Type)
}

func TestValidationErrorShape_problemDetailsTakesPrecedence(t *testing.T) {
	app, oapi := registerShapeRoute(t, Config{
		ValidationErrorShape: envErr{},
		ProblemDetails:       true,
	})

	post := oapi.GenerateOpenAPISpec()["paths"].(map[string]any)["/users/{name}"].(map[string]any)["post"].(map[string]any)
	content := post["responses"].(map[string]any)["422"].(map[string]any)["content"].(map[string]any)
	assert.Contains(t, content, MIMEApplicationProblemJSON)

	resp, err := app.Test(httptest.NewRequest("POST", "/users/a", strings.NewReader("")))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, MIMEApplicationProblemJSON, resp.Header.Get(fiber.HeaderContentType))
}
