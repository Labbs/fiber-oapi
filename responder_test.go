package fiberoapi

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawResponse(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/default", func(c fiber.Ctx, _ struct{}) (*RawResponse, struct{}) {
		return &RawResponse{Body: []byte{0x00, 0xff, 0x10}}, struct{}{}
	}, OpenAPIOptions{})
	Get(oapi, "/custom", func(c fiber.Ctx, _ struct{}) (*RawResponse, struct{}) {
		return &RawResponse{Status: 206, ContentType: "image/png", Body: []byte("png")}, struct{}{}
	}, OpenAPIOptions{})
	Post(oapi, "/created", func(c fiber.Ctx, _ struct{}) (*RawResponse, struct{}) {
		return &RawResponse{Body: []byte("new")}, struct{}{}
	}, OpenAPIOptions{SuccessStatus: 201})
	Get(oapi, "/empty", func(c fiber.Ctx, _ struct{}) (*RawResponse, struct{}) {
		return &RawResponse{Status: 204, Body: []byte("ignored")}, struct{}{}
	}, OpenAPIOptions{})

	check := func(method, path string, status int, contentType string, body []byte) {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest(method, path, nil))
		require.NoError(t, err)
		raw, _ := io.ReadAll(resp.Body)
		assert.Equal(t, status, resp.StatusCode, path)
		if contentType != "" {
			assert.Equal(t, contentType, resp.Header.Get("Content-Type"), path)
		}
		assert.Equal(t, body, raw, path)
	}
	check("GET", "/default", 200, fiber.MIMEOctetStream, []byte{0x00, 0xff, 0x10})
	check("GET", "/custom", 206, "image/png", []byte("png"))
	check("POST", "/created", 201, fiber.MIMEOctetStream, []byte("new"))
	check("GET", "/empty", 204, "", []byte{})
}

// eventStream is the Server-Sent Events Responder of the README. It sends
// one event, waits until the client asks for the next one, sends it, then
// closes the stream.
type eventStream struct {
	next <-chan struct{}
}

func (s *eventStream) Respond(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		for i := range 2 {
			if i > 0 {
				select {
				case <-s.next:
				case <-time.After(5 * time.Second):
					return
				}
			}
			fmt.Fprintf(w, "id: %d\nevent: change\ndata: {\"seq\":%d}\n\n", i, i)
			if err := w.Flush(); err != nil {
				return // client gone
			}
		}
	})
}

func TestResponder_ServerSentEventsOnARealListener(t *testing.T) {
	next := make(chan struct{})
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/events", func(c fiber.Ctx, _ struct{}) (*eventStream, struct{}) {
		return &eventStream{next: next}, struct{}{}
	}, OpenAPIOptions{ResponseContentType: "text/event-stream"})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://" + ln.Addr().String() + "/events")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))

	reader := bufio.NewReader(resp.Body)
	readEvent := func() []string {
		var lines []string
		for {
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			line = strings.TrimSuffix(line, "\n")
			if line == "" {
				return lines
			}
			lines = append(lines, line)
		}
	}

	// The first event arrives while the stream is still open.
	assert.Equal(t, []string{"id: 0", "event: change", `data: {"seq":0}`}, readEvent())
	close(next)
	assert.Equal(t, []string{"id: 1", "event: change", `data: {"seq":1}`}, readEvent())

	// Then the server closes the stream.
	_, err = reader.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

func TestResponder_Spec(t *testing.T) {
	oapi := New(fiber.New())
	Get(oapi, "/blobs/:id", func(c fiber.Ctx, _ blobInput) (*RawResponse, struct{}) {
		return nil, struct{}{}
	}, OpenAPIOptions{})
	Get(oapi, "/events", func(c fiber.Ctx, _ struct{}) (*eventStream, struct{}) {
		return nil, struct{}{}
	}, OpenAPIOptions{ResponseContentType: "text/event-stream"})
	Get(oapi, "/any", func(c fiber.Ctx, _ struct{}) (Responder, struct{}) {
		return nil, struct{}{}
	}, OpenAPIOptions{ResponseContentType: "application/pdf", SuccessStatus: 201})

	success := func(path, code string) map[string]any {
		return operationResponses(oapi, path, "get")[code].(map[string]any)["content"].(map[string]any)
	}
	assert.Equal(t, map[string]any{fiber.MIMEOctetStream: map[string]any{
		"schema": map[string]any{"type": "string", "format": "binary"},
	}}, success("/blobs/{id}", "200"))
	assert.Equal(t, map[string]any{"text/event-stream": map[string]any{
		"schema": map[string]any{"type": "string"},
	}}, success("/events", "200"))
	assert.Contains(t, success("/any", "201"), "application/pdf", "an interface output is detected too")

	schemas := oapi.GenerateOpenAPISpec()["components"].(map[string]any)["schemas"].(map[string]any)
	assert.NotContains(t, schemas, "RawResponse")
	assert.NotContains(t, schemas, "eventStream")
}

func TestResponseContentType_JSONOutput(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/doc", func(c fiber.Ctx, _ struct{}) (fiber.Map, struct{}) {
		return fiber.Map{"data": 1}, struct{}{}
	}, OpenAPIOptions{ResponseContentType: "application/vnd.api+json"})

	resp, err := app.Test(httptest.NewRequest("GET", "/doc", nil))
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.api+json", resp.Header.Get("Content-Type"))
	assert.Contains(t, operationResponses(oapi, "/doc", "get")["200"].(map[string]any)["content"], "application/vnd.api+json")
}

func TestRawResponse_ByValue(t *testing.T) {
	app := fiber.New()
	oapi := New(app)
	Get(oapi, "/value", func(c fiber.Ctx, _ struct{}) (RawResponse, struct{}) {
		return RawResponse{ContentType: "image/png", Body: []byte("png")}, struct{}{}
	}, OpenAPIOptions{})

	resp, err := app.Test(httptest.NewRequest("GET", "/value", nil))
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	assert.Equal(t, "png", string(raw), "a RawResponse by value is written as-is, not encoded as JSON")
	assert.Contains(t, operationResponses(oapi, "/value", "get")["200"].(map[string]any)["content"], fiber.MIMEOctetStream)
}
