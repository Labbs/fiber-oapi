package fiberoapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// HTTPStatusError is an optional interface a custom error instance can
// implement to expose its status code directly, bypassing field reflection.
type HTTPStatusError interface {
	HTTPStatus() int
}

// HTTPDescriptionError is an optional interface that lets a custom error
// instance provide the OpenAPI description string explicitly, bypassing the
// field-name fallback (Message → Description → Msg → HTTP reason phrase).
type HTTPDescriptionError interface {
	Description() string
}

// extractErrorStatusCode resolves the HTTP status code carried by a declared
// error instance. Priority:
//  1. HTTPStatus() int method (opt-in, type-safe)
//  2. StatusCode or Code int field on the (dereferenced) struct
//  3. fallback: 500
func extractErrorStatusCode(v any) int {
	if v == nil {
		return http.StatusInternalServerError
	}
	if r, ok := v.(HTTPStatusError); ok {
		if c := r.HTTPStatus(); c > 0 {
			return c
		}
	}
	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return http.StatusInternalServerError
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return http.StatusInternalServerError
	}
	for _, name := range []string{"StatusCode", "Code"} {
		if f := val.FieldByName(name); f.IsValid() && f.CanInt() {
			if c := int(f.Int()); c > 0 {
				return c
			}
		}
	}
	return http.StatusInternalServerError
}

// extractErrorDescription returns the human-readable description used in the
// OpenAPI spec for a declared error. Priority:
//  1. Description() string method
//  2. Message / Description / Msg string field
//  3. fallback: HTTP reason phrase for the resolved status code
func extractErrorDescription(v any, code int) string {
	if v == nil {
		return http.StatusText(code)
	}
	if r, ok := v.(HTTPDescriptionError); ok {
		if d := r.Description(); d != "" {
			return d
		}
	}
	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return http.StatusText(code)
		}
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		for _, name := range []string{"Description", "Message", "Msg"} {
			if f := val.FieldByName(name); f.IsValid() && f.Kind() == reflect.String {
				if s := f.String(); s != "" {
					return s
				}
			}
		}
	}
	if text := http.StatusText(code); text != "" {
		return text
	}
	return "Error response"
}

// errorSchemaRef returns the schema reference for a declared error type. Named
// types are exposed via $ref so the spec deduplicates the schema; anonymous
// types fall back to an inline schema.
func errorSchemaRef(t reflect.Type) map[string]interface{} {
	if t == nil {
		return map[string]interface{}{"type": "object"}
	}
	t = dereferenceType(t)
	if shouldInlineOperationSchema(t) {
		return generateSchema(t)
	}
	if name := getTypeName(t); name != "" {
		return map[string]interface{}{"$ref": "#/components/schemas/" + name}
	}
	return generateSchema(t)
}

// buildErrorResponse turns a single declared error instance into an OpenAPI
// response object. The status code is returned alongside so the caller can
// place it under the right key.
func buildErrorResponse(errInst any) (statusCode int, response map[string]interface{}) {
	statusCode = extractErrorStatusCode(errInst)
	description := extractErrorDescription(errInst, statusCode)
	t := reflect.TypeOf(errInst)
	response = map[string]interface{}{
		"description": description,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema":  errorSchemaRef(t),
				"example": errInst,
			},
		},
	}
	return statusCode, response
}

// statusCodeKey formats a status code as the string key expected by the
// OpenAPI responses map. We use the concrete code (e.g. "404") rather than
// the "4XX" wildcard so each declared error has its own slot.
func statusCodeKey(code int) string {
	return strconv.Itoa(code)
}

// hasNonNilErrorEntry reports whether the slice contains at least one non-nil
// entry. A slice of only nils is treated as "no errors declared" since the
// spec-generation loop skips nil entries — counting them as a declaration
// would suppress the 4XX catch-all without emitting any replacement.
func hasNonNilErrorEntry(errors []any) bool {
	for _, e := range errors {
		if e != nil {
			return true
		}
	}
	return false
}

// errorCategory groups the inputs needed to materialise a user-defined error
// shape from a library-internal error. The category is what we know about the
// error before we know what shape the user wants.
type errorCategory struct {
	Code    int
	Type    string // entry-type discriminator: validation_error / type_error / parse_error / authentication_error / authorization_error / not_found / method_not_allowed
	Message string // human-readable message (single line)
	Details string // optional secondary context (joined field list, source error, ...)
}

// isValidationError reports whether err is a go-playground/validator error.
// Used to keep validation responses on the rich ErrorEnvelope shape even when
// the user opted into a flat DefaultErrorShape — per-field info (loc /
// constraint / field) only makes sense in the array-of-entries shape.
func isValidationError(err error) bool {
	var vErrs validator.ValidationErrors
	return errors.As(err, &vErrs)
}

// categorizeError extracts the (code, type, message, details) tuple from any
// internal error produced by parseInput. Used by the default error handler to
// build either an ErrorEnvelope or a user-supplied DefaultErrorShape instance.
func categorizeError(err error) errorCategory {
	if authErr, ok := errors.AsType[*AuthError](err); ok {
		t := errTypeAuthN
		if authErr.StatusCode == fiber.StatusForbidden {
			t = errTypeAuthZ
		}
		return errorCategory{
			Code:    authErr.StatusCode,
			Type:    t,
			Message: authErr.Message,
		}
	}
	if ute, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		field := ute.Field
		if i := strings.LastIndex(field, "."); i >= 0 {
			field = field[i+1:]
		}
		msg := fmt.Sprintf(typeMismatchMsgFmt, field, ute.Type.String(), ute.Value)
		if field == "" {
			msg = fmt.Sprintf("invalid JSON: expected %s but got %s", ute.Type.String(), ute.Value)
		}
		return errorCategory{
			Code:    statusParseError,
			Type:    errTypeTypeMismatch,
			Message: msg,
			Details: ute.Type.String(),
		}
	}
	var vErrs validator.ValidationErrors
	if errors.As(err, &vErrs) {
		msgs := make([]string, 0, len(vErrs))
		for _, fe := range vErrs {
			msgs = append(msgs, translateValidatorTag(fe.Field(), fe.Tag(), fe.Param()))
		}
		head := msgs[0]
		if len(msgs) > 1 {
			head = fmt.Sprintf("%s (and %d more)", msgs[0], len(msgs)-1)
		}
		return errorCategory{
			Code:    statusValidationError,
			Type:    errTypeValidation,
			Message: head,
			Details: strings.Join(msgs, "; "),
		}
	}
	return errorCategory{
		Code:    statusParseError,
		Type:    errTypeParse,
		Message: err.Error(),
	}
}

// materializeError builds a new instance of the user's DefaultErrorShape with
// reflection-populated fields. The shape parameter is a template (typically the
// empty value the user stored in Config.DefaultErrorShape).
//
// Returns:
//   - the new instance (same kind as shape — struct or pointer-to-struct) when
//     the shape's underlying type is a struct;
//   - the shape value unchanged when it is not a struct (no fields to populate);
//   - nil only when shape itself is nil.
//
// Field assignments (case-sensitive, applied if present and settable):
//   - StatusCode, Code  → cat.Code
//   - Message, Description, Msg → cat.Message
//   - Type → cat.Type
//   - Details → cat.Details
func materializeError(shape any, cat errorCategory) any {
	if shape == nil {
		return nil
	}
	t := reflect.TypeOf(shape)
	isPtr := t.Kind() == reflect.Ptr
	if isPtr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return shape
	}
	inst := reflect.New(t).Elem()
	setIntFieldIfPresent(inst, "StatusCode", int64(cat.Code))
	setIntFieldIfPresent(inst, "Code", int64(cat.Code))
	setStringFieldIfPresent(inst, "Message", cat.Message)
	setStringFieldIfPresent(inst, "Description", cat.Message)
	setStringFieldIfPresent(inst, "Msg", cat.Message)
	setStringFieldIfPresent(inst, "Type", cat.Type)
	setStringFieldIfPresent(inst, "Details", cat.Details)
	if isPtr {
		return inst.Addr().Interface()
	}
	return inst.Interface()
}

func setIntFieldIfPresent(v reflect.Value, name string, val int64) {
	f := v.FieldByName(name)
	if f.IsValid() && f.CanSet() && f.CanInt() {
		f.SetInt(val)
	}
}

func setStringFieldIfPresent(v reflect.Value, name string, val string) {
	f := v.FieldByName(name)
	if f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
		f.SetString(val)
	}
}
