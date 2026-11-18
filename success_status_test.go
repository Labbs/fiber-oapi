package fiberoapi

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blobResult struct {
	ID      string `json:"id"`
	created bool
}

// HTTPStatus tells a new blob (201) from one already present (200).
func (r *blobResult) HTTPStatus() int {
	if r.created {
		return fiber.StatusCreated
	}
	return fiber.StatusOK
}

type blobInput struct {
	ID string `uri:"id"`
}

func doRequest(t *testing.T, app *fiber.App, method, path string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil))
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestSuccessStatus_Runtime(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/created", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"id": "1"}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})
	Post(oapi, "/logout", func(c fiber.Ctx, _ struct{}) (*struct{}, struct{}) {
		return nil, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 204})
	Put(oapi, "/blobs/:id", func(c fiber.Ctx, in blobInput) (*blobResult, struct{}) {
		return &blobResult{ID: in.ID, created: in.ID != "existing"}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})
	Post(oapi, "/accepted", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		c.Status(fiber.StatusAccepted)
		return fiber.Map{"queued": true}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})
	Get(oapi, "/plain", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ok": true}, struct{}{}
	}, OpenAPIOptions{})

	status, body := doRequest(t, app, "POST", "/created")
	assert.Equal(t, 201, status)
	assert.JSONEq(t, `{"id":"1"}`, body)

	status, body = doRequest(t, app, "POST", "/logout")
	assert.Equal(t, 204, status)
	assert.Empty(t, body, "a 204 has no body, not null")

	status, _ = doRequest(t, app, "PUT", "/blobs/new")
	assert.Equal(t, 201, status, "HTTPStatus() of the output")
	status, body = doRequest(t, app, "PUT", "/blobs/existing")
	assert.Equal(t, 200, status, "HTTPStatus() of the output wins over SuccessStatus")
	assert.JSONEq(t, `{"id":"existing"}`, body)

	status, _ = doRequest(t, app, "POST", "/accepted")
	assert.Equal(t, 202, status, "a status set by the handler wins")

	status, _ = doRequest(t, app, "GET", "/plain")
	assert.Equal(t, 200, status)
}

func TestSuccessStatus_Spec(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/created", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"id": "1"}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})
	Post(oapi, "/logout", func(c fiber.Ctx, _ struct{}) (*struct{}, struct{}) {
		return nil, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 204})
	Put(oapi, "/blobs/:id", func(c fiber.Ctx, in blobInput) (*blobResult, struct{}) {
		return &blobResult{ID: in.ID}, struct{}{}
	}, OpenAPIOptions{
		SuccessStatus: 201,
		Responses: map[string]any{
			"200": map[string]any{"description": "Same content already present"},
			"422": map[string]any{"description": "Replaced"},
		},
	})

	created := operationResponses(oapi, "/created", "post")
	assert.NotContains(t, created, "200")
	require.Contains(t, created, "201")
	assert.Equal(t, "Created", created["201"].(map[string]any)["description"])
	assert.Contains(t, created["201"].(map[string]any), "content")

	logout := operationResponses(oapi, "/logout", "post")
	require.Contains(t, logout, "204")
	assert.NotContains(t, logout["204"].(map[string]any), "content", "a 204 is documented without content")

	blobs := operationResponses(oapi, "/blobs/{id}", "put")
	assert.Contains(t, blobs, "201")
	assert.Equal(t, map[string]any{"description": "Same content already present"}, blobs["200"])
	assert.Equal(t, map[string]any{"description": "Replaced"}, blobs["422"], "Responses replace generated entries")
}

// Without SuccessStatus, a route keeps its 200 JSON response, in the spec and
// at runtime.
func TestSuccessStatus_DefaultIs200(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/items", func(c fiber.Ctx, _ struct{}) (customErrOutput, struct{}) {
		return customErrOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{})

	responses := operationResponses(oapi, "/items", "post")
	require.Contains(t, responses, "200")
	ok := responses["200"].(map[string]any)
	assert.Equal(t, "Successful response", ok["description"])
	schema := ok["content"].(map[string]any)["application/json"].(map[string]any)["schema"]
	assert.Equal(t, map[string]any{"$ref": "#/components/schemas/customErrOutput"}, schema)

	resp, err := app.Test(httptest.NewRequest("POST", "/items", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	raw, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"message":"ok"}`, string(raw))
}

// A 201 POST documents the output schema under "201", and only there.
func TestSuccessStatus_201KeepsOutputSchema(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/items", func(c fiber.Ctx, _ struct{}) (customErrOutput, struct{}) {
		return customErrOutput{Message: "ok"}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})

	responses := operationResponses(oapi, "/items", "post")
	assert.NotContains(t, responses, "200")
	require.Contains(t, responses, "201")
	schema := responses["201"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"]
	assert.Equal(t, map[string]any{"$ref": "#/components/schemas/customErrOutput"}, schema)
}

// A 204 DELETE is documented without content and answers an empty body even
// when the handler returns a value.
func TestSuccessStatus_204DeleteDropsBody(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Delete(oapi, "/items/:name", func(c fiber.Ctx, in customErrInput) (customErrOutput, struct{}) {
		return customErrOutput{Message: "deleted " + in.Name}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 204})

	responses := operationResponses(oapi, "/items/{name}", "delete")
	assert.NotContains(t, responses, "200")
	require.Contains(t, responses, "204")
	assert.Equal(t, map[string]any{"description": "No Content"}, responses["204"])

	status, body := doRequest(t, app, "DELETE", "/items/abc")
	assert.Equal(t, 204, status)
	assert.Empty(t, body)
}

func TestSuccessStatus_205HasNoBody(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/form", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"ignored": true}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 205})

	assert.Equal(t, map[string]any{"description": "Reset Content"}, operationResponses(oapi, "/form", "post")["205"])
	status, body := doRequest(t, app, "POST", "/form")
	assert.Equal(t, 205, status)
	assert.Empty(t, body)
}

// An error returned by a 201 route keeps its own status.
func TestSuccessStatus_ErrorWins(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Post(oapi, "/items/:name", func(c fiber.Ctx, in customErrInput) (customErrOutput, *AppError) {
		if in.Name == "taken" {
			return customErrOutput{}, appConflict("already exists")
		}
		return customErrOutput{Message: "ok"}, nil
	}, OpenAPIOptions{SuccessStatus: 201, Errors: []any{appConflict("already exists")}})

	responses := operationResponses(oapi, "/items/{name}", "post")
	assert.Contains(t, responses, "201")
	assert.Contains(t, responses, "409")

	status, _ := doRequest(t, app, "POST", "/items/fresh")
	assert.Equal(t, 201, status)
	status, body := doRequest(t, app, "POST", "/items/taken")
	assert.Equal(t, 409, status)
	assert.Contains(t, body, "already exists")
	status, _ = doRequest(t, app, "POST", "/items/x")
	assert.Equal(t, 422, status, "a validation error keeps its status too")
}

func TestSuccessStatus_RejectsNon2xx(t *testing.T) {
	for _, status := range []int{404, 302, 199, 300, 500, -1} {
		app := fiber.New()
		oapi := New(app)
		assert.Panics(t, func() {
			Post(oapi, "/items", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
				return fiber.Map{}, struct{}{}
			}, OpenAPIOptions{SuccessStatus: status})
		}, "SuccessStatus %d", status)
	}

	app := fiber.New()
	oapi := New(app)
	assert.PanicsWithValue(t, "Success status validation failed for DELETE /v1/items: SuccessStatus 404 is not a 2xx status code", func() {
		Delete(Group(oapi, "/v1"), "/items", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
			return fiber.Map{}, struct{}{}
		}, OpenAPIOptions{SuccessStatus: 404})
	})

	for _, status := range []int{0, 200, 201, 204, 299} {
		assert.NotPanics(t, func() {
			Post(New(fiber.New()), "/items", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
				return fiber.Map{}, struct{}{}
			}, OpenAPIOptions{SuccessStatus: status})
		}, "SuccessStatus %d", status)
	}
}

func TestSuccessStatus_Group(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	v1 := Group(oapi, "/v1")
	admin := Group(v1, "/admin")
	Post(v1, "/items", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"id": "1"}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})
	Delete(admin, "/items/:name", func(c fiber.Ctx, _ customErrInput) (fiber.Map, struct{}) {
		return fiber.Map{"ignored": true}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 204})

	assert.Contains(t, operationResponses(oapi, "/v1/items", "post"), "201")
	assert.Equal(t, map[string]any{"description": "No Content"}, operationResponses(oapi, "/v1/admin/items/{name}", "delete")["204"])

	status, body := doRequest(t, app, "POST", "/v1/items")
	assert.Equal(t, 201, status)
	assert.JSONEq(t, `{"id":"1"}`, body)
	status, body = doRequest(t, app, "DELETE", "/v1/admin/items/abc")
	assert.Equal(t, 204, status)
	assert.Empty(t, body)
}
