package fiberoapi

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type putBlobInput struct {
	ID          string `uri:"id" validate:"required"`
	IfNoneMatch string `header:"If-None-Match" validate:"required,eq=*"`
	Body        []byte `body:"raw" validate:"required,max=2097152"`
}

func putBlob(t *testing.T, app *fiber.App, path string, body []byte) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest("PUT", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("If-None-Match", "*")
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func TestRawBody_ReceivedIntactAndCopied(t *testing.T) {
	payload := make([]byte, 1<<20)
	_, err := rand.Read(payload)
	require.NoError(t, err)

	var received []byte
	var aliased bool
	app := fiber.New()
	oapi := New(app)
	Put(oapi, "/blobs/:id", func(c fiber.Ctx, in putBlobInput) (fiber.Map, struct{}) {
		received = in.Body
		aliased = &in.Body[0] == &c.Body()[0]
		return fiber.Map{"id": in.ID, "size": len(in.Body)}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})

	status, body := putBlob(t, app, "/blobs/b-1", payload)
	require.Equal(t, 201, status, "%s", body)
	assert.JSONEq(t, `{"id":"b-1","size":1048576}`, string(body))
	assert.True(t, bytes.Equal(payload, received), "the body must be received intact")
	assert.False(t, aliased, "the field must not share memory with Fiber's buffer")
}

func TestRawBody_Validation(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Put(oapi, "/small/:id", func(c fiber.Ctx, in struct {
		ID   string `uri:"id"`
		Body []byte `body:"raw" validate:"required,max=4"`
	}) (fiber.Map, struct{}) {
		return fiber.Map{"size": len(in.Body)}, struct{}{}
	}, OpenAPIOptions{})

	status, _ := putBlob(t, app, "/small/1", []byte("1234"))
	assert.Equal(t, 200, status)

	status, raw := putBlob(t, app, "/small/1", []byte("12345"))
	assert.Equal(t, 422, status)
	var envelope ErrorEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	assert.Equal(t, []any{"body"}, envelope.Errors[0].Loc)
	assert.Equal(t, "max=4", envelope.Errors[0].Constraint)

	status, raw = putBlob(t, app, "/small/1", nil)
	assert.Equal(t, 422, status, "an empty body fails required")
	require.NoError(t, json.Unmarshal(raw, &envelope))
	assert.Equal(t, "required", envelope.Errors[0].Constraint)
}

func TestRawBody_InvalidDeclarationsPanic(t *testing.T) {
	assert.PanicsWithValue(t, `Raw body validation failed for /twice: fields A and B both have body:"raw"`, func() {
		Post(New(fiber.New()), "/twice", func(c fiber.Ctx, _ struct {
			A []byte `body:"raw"`
			B []byte `body:"raw"`
		}) (fiber.Map, struct{}) {
			return nil, struct{}{}
		}, OpenAPIOptions{})
	})
	assert.Panics(t, func() {
		Post(New(fiber.New()), "/string", func(c fiber.Ctx, _ struct {
			Body string `body:"raw"`
		}) (fiber.Map, struct{}) {
			return nil, struct{}{}
		}, OpenAPIOptions{})
	})
}

func TestRawBody_Spec(t *testing.T) {
	oapi := New(fiber.New())
	handler := func(c fiber.Ctx, in putBlobInput) (fiber.Map, struct{}) { return nil, struct{}{} }
	Put(oapi, "/blobs/:id", handler, OpenAPIOptions{})
	Post(oapi, "/images/:id", handler, OpenAPIOptions{RequestContentType: "image/png"})

	spec := oapi.GenerateOpenAPISpec()
	operation := func(path, method string) map[string]any {
		return spec["paths"].(map[string]any)[path].(map[string]any)[method].(map[string]any)
	}
	binary := map[string]any{"type": "string", "format": "binary"}

	blobs := operation("/blobs/{id}", "put")
	content := blobs["requestBody"].(map[string]any)["content"].(map[string]any)
	assert.Equal(t, map[string]any{fiber.MIMEOctetStream: map[string]any{"schema": binary}}, content)

	images := operation("/images/{id}", "post")
	content = images["requestBody"].(map[string]any)["content"].(map[string]any)
	assert.Equal(t, map[string]any{"image/png": map[string]any{"schema": binary}}, content)

	var names []string
	for _, param := range blobs["parameters"].([]map[string]any) {
		names = append(names, param["in"].(string)+":"+param["name"].(string))
	}
	assert.ElementsMatch(t, []string{"path:id", "header:If-None-Match"}, names, "uri and header fields stay parameters")

	schema := spec["components"].(map[string]any)["schemas"].(map[string]any)["putBlobInput"].(map[string]any)
	assert.Empty(t, schema["properties"], "the raw field is not part of a JSON schema")
}
