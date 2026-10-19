package fiberoapi

import (
	"net/http"
	"reflect"

	"github.com/gofiber/fiber/v3"
)

// MIMEApplicationProblemJSON is the media type of RFC 9457 problem details.
const MIMEApplicationProblemJSON = "application/problem+json"

// Problem is an error response following RFC 9457 (problem details for HTTP
// APIs). Code is an extension member: a stable, machine-readable identifier.
// Errors is an extension member too, carrying one entry per failing field.
//
// With Config.ProblemDetails, every error produced by the library is a
// Problem, sent and documented as application/problem+json. A handler can
// also return one through its TError.
type Problem struct {
	Type     string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Title    string                 `json:"title" yaml:"title"`
	Status   int                    `json:"status" yaml:"status"`
	Detail   string                 `json:"detail,omitempty" yaml:"detail,omitempty"`
	Instance string                 `json:"instance,omitempty" yaml:"instance,omitempty"`
	Code     string                 `json:"code,omitempty" yaml:"code,omitempty"`
	Errors   []ValidationErrorEntry `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// The methods of Problem have value receivers, so that a Problem works the
// same whether it is passed by value or by pointer.

// HTTPStatus returns the status of the problem.
func (p Problem) HTTPStatus() int { return p.Status }

// ContentType returns application/problem+json.
func (p Problem) ContentType() string { return MIMEApplicationProblemJSON }

// Error returns the title of the problem.
func (p Problem) Error() string { return p.Title }

// ContentTyper is an optional interface an error value (TError, an entry of
// OpenAPIOptions.Errors) can implement to choose the Content-Type it is sent
// and documented with. Without it, errors are application/json.
type ContentTyper interface {
	ContentType() string
}

var contentTyperType = reflect.TypeFor[ContentTyper]()

// Error categories that are not entry types of the ErrorEnvelope.
const (
	errTypeMethodNotAllowed = "method_not_allowed"
	errTypeSerialization    = "serialization_error"
	errTypeInternal         = "internal_error"
)

// problemFor builds the Problem for an error produced by the library. The
// Code is the category, unless Config.ProblemCodes maps it to another value.
func (cfg *Config) problemFor(cat errorCategory, entries []ValidationErrorEntry) *Problem {
	code := cat.Type
	if custom, ok := cfg.ProblemCodes[cat.Type]; ok && custom != "" {
		code = custom
	}
	title := http.StatusText(cat.Code)
	if title == "" {
		title = "Error"
	}
	return &Problem{
		Title:  title,
		Status: cat.Code,
		Detail: cat.Message,
		Code:   code,
		Errors: entries,
	}
}

// writeProblem sends p with its status and Content-Type.
func writeProblem(c fiber.Ctx, p *Problem) error {
	return c.Status(p.Status).JSON(p, p.ContentType())
}

// validationStatus returns the status of validation errors: ValidationStatusCode,
// or 422 when it is not set.
func (cfg *Config) validationStatus() int {
	if cfg.ValidationStatusCode > 0 {
		return cfg.ValidationStatusCode
	}
	return statusValidationError
}

// contentTypeOf returns the media type an error value is sent with.
func contentTypeOf(v any) string {
	if ct, ok := v.(ContentTyper); ok {
		if mediaType := ct.ContentType(); mediaType != "" {
			return mediaType
		}
	}
	return fiber.MIMEApplicationJSON
}

// contentTypeOfType returns the media type values of type t are sent with,
// for the spec of a TError that is only known by its type.
func contentTypeOfType(t reflect.Type) string {
	if t == nil || t.Kind() == reflect.Interface || !t.Implements(contentTyperType) {
		return fiber.MIMEApplicationJSON
	}
	if t.Kind() == reflect.Ptr {
		return contentTypeOf(reflect.New(t.Elem()).Interface())
	}
	return contentTypeOf(reflect.Zero(t).Interface())
}
