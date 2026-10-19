package fiberoapi

import (
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Responder is an output that writes its own response (status, headers and
// body): binary content, a stream, Server-Sent Events... When a handler
// returns a non-nil Responder, the library calls Respond instead of encoding
// the output as JSON. The success status (see OpenAPIOptions.SuccessStatus) is
// set beforehand, and Respond may override it.
//
// The spec documents the success response under
// OpenAPIOptions.ResponseContentType (application/octet-stream by default).
type Responder interface {
	Respond(c fiber.Ctx) error
}

var responderType = reflect.TypeFor[Responder]()

// RawResponse is a Responder for a binary body.
type RawResponse struct {
	Status      int    // 0 keeps the success status of the route (200 by default)
	ContentType string // default application/octet-stream
	Body        []byte
}

// Respond writes the status, the Content-Type and the body.
func (r *RawResponse) Respond(c fiber.Ctx) error {
	if r.Status != 0 {
		c.Status(r.Status)
	}
	if status := c.Response().StatusCode(); status == fiber.StatusNoContent || status == fiber.StatusNotModified {
		return c.SendStatus(status)
	}
	contentType := r.ContentType
	if contentType == "" {
		contentType = fiber.MIMEOctetStream
	}
	c.Set(fiber.HeaderContentType, contentType)
	return c.Send(r.Body)
}

// responderSchema is the schema of a body written by a Responder: a string
// for a text media type, binary data otherwise.
func responderSchema(mediaType string) map[string]interface{} {
	if strings.HasPrefix(mediaType, "text/") {
		return map[string]interface{}{"type": "string"}
	}
	return map[string]interface{}{"type": "string", "format": "binary"}
}
