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
