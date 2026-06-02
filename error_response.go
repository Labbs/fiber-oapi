package fiberoapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

const (
	requestIDHeader  = "X-Request-Id"
	maxRequestIDLen  = 128  // bytes — request-id is opaque, anything longer is abuse
	maxNotFoundPath  = 1024 // bytes — truncate echoed path to avoid log/UI blow-up
)

// requestIDPattern accepts characters typical for trace IDs (UUID, ULID, hex,
// dotted notation). Anything else is dropped to neutralise CRLF / log-injection
// vectors when the value is echoed into a JSON body that later ends up in logs.
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._\-:]+$`)

// sanitizeRequestID returns the header value if it is short enough and only
// contains safe characters; otherwise it returns an empty string.
func sanitizeRequestID(s string) string {
	if s == "" || len(s) > maxRequestIDLen {
		return ""
	}
	if !requestIDPattern.MatchString(s) {
		return ""
	}
	return s
}

// sanitizePath bounds and validates the path string echoed back to the client.
// Non-UTF-8 sequences are replaced and the result is truncated to maxNotFoundPath.
func sanitizePath(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	if len(s) > maxNotFoundPath {
		// Truncate on a rune boundary so we do not produce invalid UTF-8.
		s = s[:maxNotFoundPath]
		for !utf8.ValidString(s) && len(s) > 0 {
			s = s[:len(s)-1]
		}
		s += "…"
	}
	return s
}

// Status codes used by the default handlers. 422 follows the convention used by
// FastAPI/Pydantic and DRF: 400 for "I could not parse what you sent" and 422
// for "I parsed it but the content failed validation rules".
const (
	statusParseError      = fiber.StatusBadRequest          // 400
	statusValidationError = fiber.StatusUnprocessableEntity // 422
)

// Entry type constants — kept stable so clients can branch on them.
const (
	errTypeValidation   = "validation_error"
	errTypeTypeMismatch = "type_error"
	errTypeParse        = "parse_error"
	errTypeAuthN        = "authentication_error"
	errTypeAuthZ        = "authorization_error"
	errTypeNotFound     = "not_found"
)

// locResolverCache memoises per-type loc resolvers so error formatting does not
// pay the reflection cost on every request.
var locResolverCache sync.Map // map[reflect.Type]*locResolver

type locResolver struct {
	root reflect.Type
}

func resolverFor(t reflect.Type) *locResolver {
	if t == nil {
		return &locResolver{}
	}
	if cached, ok := locResolverCache.Load(t); ok {
		return cached.(*locResolver)
	}
	r := &locResolver{root: t}
	actual, _ := locResolverCache.LoadOrStore(t, r)
	return actual.(*locResolver)
}

// resolve takes the validator namespace (Go struct names, dot-separated) and
// produces the JSON-flavoured loc array plus the leaf field name. The first
// element of loc is the source (body / path / query / header), the remaining
// elements are field names in the source's naming convention.
func (r *locResolver) resolve(namespace string) (loc []any, leaf string) {
	if r.root == nil {
		return []any{"body"}, ""
	}
	segs := strings.Split(namespace, ".")
	// Drop the root struct name if validator prefixed it.
	if len(segs) > 0 && segs[0] == r.root.Name() {
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return []any{"body"}, ""
	}

	t := dereferenceType(r.root)
	if t.Kind() != reflect.Struct {
		return []any{"body"}, ""
	}

	loc = make([]any, 0, len(segs)+1)
	for i, seg := range segs {
		field, ok := t.FieldByName(seg)
		if !ok {
			break
		}
		if i == 0 {
			if tag := field.Tag.Get("uri"); tag != "" {
				loc = append(loc, "path", tag)
			} else if tag := field.Tag.Get("query"); tag != "" {
				loc = append(loc, "query", tag)
			} else if tag := field.Tag.Get("header"); tag != "" {
				loc = append(loc, "header", tag)
			} else {
				loc = append(loc, "body", jsonFieldName(field))
			}
		} else {
			loc = append(loc, jsonFieldName(field))
		}
		t = dereferenceType(field.Type)
		if t.Kind() != reflect.Struct {
			// Cannot descend further; remaining segments would not be valid struct fields.
			break
		}
	}

	if len(loc) > 0 {
		if s, ok := loc[len(loc)-1].(string); ok {
			leaf = s
		}
	}
	return loc, leaf
}

// jsonFieldName returns the JSON name of a struct field, falling back to the Go
// name when the field has no json tag or the tag explicitly hides the field.
func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return field.Name
	}
	return strings.Split(tag, ",")[0]
}

// friendlyJSONError wraps a *json.UnmarshalTypeError so the user-facing message
// (Error()) is the friendly "invalid type for field 'X'..." form while the
// original error is still recoverable via errors.As / errors.AsType. Used by
// parseInput so custom ValidationErrorHandlers that call err.Error() keep
// getting a nice message.
type friendlyJSONError struct {
	msg string
	ute *json.UnmarshalTypeError
}

func (e *friendlyJSONError) Error() string { return e.msg }
func (e *friendlyJSONError) Unwrap() error { return e.ute }

// wrapJSONTypeError builds a friendlyJSONError if err carries a JSON type
// mismatch. Returns nil if err is not a *json.UnmarshalTypeError.
func wrapJSONTypeError(err error) error {
	ute, ok := errors.AsType[*json.UnmarshalTypeError](err)
	if !ok {
		return nil
	}
	field := ute.Field
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	msg := fmt.Sprintf("invalid type for field '%s': expected %s but got %s", field, ute.Type.String(), ute.Value)
	if field == "" {
		msg = fmt.Sprintf("invalid JSON: expected %s but got %s", ute.Type.String(), ute.Value)
	}
	return &friendlyJSONError{msg: msg, ute: ute}
}

// translateValidatorTag turns a validator tag + parameter into a human-readable
// message. Covers the most common tags from go-playground/validator; anything
// unknown falls back to a generic message that still exposes the tag name so the
// client side stays informative.
func translateValidatorTag(field, tag, param string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("field '%s' is required", field)
	case "min":
		return fmt.Sprintf("field '%s' must be at least %s characters long", field, param)
	case "max":
		return fmt.Sprintf("field '%s' must be at most %s characters long", field, param)
	case "len":
		return fmt.Sprintf("field '%s' must be exactly %s characters long", field, param)
	case "email":
		return fmt.Sprintf("field '%s' must be a valid email address", field)
	case "url":
		return fmt.Sprintf("field '%s' must be a valid URL", field)
	case "uuid", "uuid4":
		return fmt.Sprintf("field '%s' must be a valid UUID", field)
	case "alphanum":
		return fmt.Sprintf("field '%s' must contain only alphanumeric characters", field)
	case "alpha":
		return fmt.Sprintf("field '%s' must contain only alphabetic characters", field)
	case "numeric":
		return fmt.Sprintf("field '%s' must be numeric", field)
	case "oneof":
		return fmt.Sprintf("field '%s' must be one of: %s", field, param)
	case "gte":
		return fmt.Sprintf("field '%s' must be greater than or equal to %s", field, param)
	case "lte":
		return fmt.Sprintf("field '%s' must be less than or equal to %s", field, param)
	case "gt":
		return fmt.Sprintf("field '%s' must be greater than %s", field, param)
	case "lt":
		return fmt.Sprintf("field '%s' must be less than %s", field, param)
	default:
		if param != "" {
			return fmt.Sprintf("field '%s' failed validation: %s=%s", field, tag, param)
		}
		return fmt.Sprintf("field '%s' failed validation: %s", field, tag)
	}
}

// buildEnvelope produces an ErrorEnvelope from any error returned by parseInput.
// The status code carried in the envelope entries (and intended to be set on the
// response) is returned alongside so the handler can call c.Status() once.
func buildEnvelope(c fiber.Ctx, cfg Config, inputType reflect.Type, err error) (ErrorEnvelope, int) {
	resolver := resolverFor(inputType)
	ctx := ResponseContext{ResponseID: sanitizeRequestID(c.Get(requestIDHeader))}

	// AuthError — single entry, status from the error itself.
	if authErr, ok := errors.AsType[*AuthError](err); ok {
		status := authErr.StatusCode
		entryType := errTypeAuthN
		if status == fiber.StatusForbidden {
			entryType = errTypeAuthZ
		}
		return ErrorEnvelope{
			Errors: []ValidationErrorEntry{{
				Type: entryType,
				Code: status,
				Loc:  []any{"header", "Authorization"},
				Msg:  authErr.Message,
			}},
			ResponseContext: ctx,
		}, status
	}

	// JSON type mismatch — single entry, 400 Bad Request.
	if ute, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		fieldName := ute.Field
		if i := strings.LastIndex(fieldName, "."); i >= 0 {
			fieldName = fieldName[i+1:]
		}
		loc, _ := resolver.resolve(strings.ReplaceAll(ute.Field, ".", "."))
		// resolver works on Go names; UnmarshalTypeError gives JSON names with dots,
		// so when no match is found it just produces ["body"]. Append the leaf for clarity.
		if len(loc) == 1 && fieldName != "" {
			loc = append(loc, fieldName)
		}
		msg := fmt.Sprintf("invalid type for field '%s': expected %s but got %s", fieldName, ute.Type.String(), ute.Value)
		if fieldName == "" {
			msg = fmt.Sprintf("invalid JSON: expected %s but got %s", ute.Type.String(), ute.Value)
		}
		entry := ValidationErrorEntry{
			Type:       errTypeTypeMismatch,
			Code:       statusParseError,
			Loc:        loc,
			Field:      fieldName,
			Msg:        msg,
			Constraint: ute.Type.String(),
		}
		if cfg.IncludeInvalidValueInErrors {
			entry.Value = ute.Value
		}
		return ErrorEnvelope{Errors: []ValidationErrorEntry{entry}, ResponseContext: ctx}, statusParseError
	}

	// validator.ValidationErrors — one entry per failing field, 422.
	var vErrs validator.ValidationErrors
	if errors.As(err, &vErrs) {
		entries := make([]ValidationErrorEntry, 0, len(vErrs))
		for _, fe := range vErrs {
			loc, leaf := resolver.resolve(fe.StructNamespace())
			if leaf == "" {
				leaf = fe.Field()
			}
			entry := ValidationErrorEntry{
				Type:       errTypeValidation,
				Code:       statusValidationError,
				Loc:        loc,
				Field:      leaf,
				Msg:        translateValidatorTag(leaf, fe.Tag(), fe.Param()),
				Constraint: constraintString(fe.Tag(), fe.Param()),
			}
			if cfg.IncludeInvalidValueInErrors {
				entry.Value = fe.Value()
			}
			entries = append(entries, entry)
		}
		return ErrorEnvelope{Errors: entries, ResponseContext: ctx}, statusValidationError
	}

	// Anything else — generic parse error.
	return ErrorEnvelope{
		Errors: []ValidationErrorEntry{{
			Type: errTypeParse,
			Code: statusParseError,
			Loc:  []any{"body"},
			Msg:  err.Error(),
		}},
		ResponseContext: ctx,
	}, statusParseError
}

func constraintString(tag, param string) string {
	if param == "" {
		return tag
	}
	return tag + "=" + param
}

// exampleEnvelope returns a representative ErrorEnvelope used as the OpenAPI
// example for the 422 response. It is deliberately compact but realistic enough
// to show the shape to consumers reading the spec.
func exampleValidationEnvelope() ErrorEnvelope {
	return ErrorEnvelope{
		Errors: []ValidationErrorEntry{{
			Type:       errTypeValidation,
			Code:       statusValidationError,
			Loc:        []any{"body", "workspaceId"},
			Field:      "workspaceId",
			Msg:        "field 'workspaceId' must be at least 11 characters long",
			Constraint: "min=11",
		}},
		ResponseContext: ResponseContext{ResponseID: "bf0e9029-576b-42e8-84f9-ad0622972f50"},
	}
}

// NotFoundEnvelope is the public counterpart to the internal builder: it
// produces the default 404 ErrorEnvelope for a request. Custom NotFoundHandler
// implementations can call it to reuse the library's shape while overriding
// only the response status or body.
func NotFoundEnvelope(c fiber.Ctx) ErrorEnvelope {
	method := c.Method()
	path := sanitizePath(c.Path())
	return ErrorEnvelope{
		Errors: []ValidationErrorEntry{{
			Type:  errTypeNotFound,
			Code:  fiber.StatusNotFound,
			Loc:   []any{"path"},
			Field: path,
			Msg:   fmt.Sprintf("no route matches %s %s", method, path),
		}},
		ResponseContext: ResponseContext{ResponseID: sanitizeRequestID(c.Get(requestIDHeader))},
	}
}

// methodNotAllowedEnvelope is emitted when the path exists on other HTTP
// methods. It mirrors the 405 status code Fiber would otherwise emit, but in
// our envelope shape so clients only have to parse one structure.
func methodNotAllowedEnvelope(c fiber.Ctx, allowed []string) ErrorEnvelope {
	method := c.Method()
	path := sanitizePath(c.Path())
	return ErrorEnvelope{
		Errors: []ValidationErrorEntry{{
			Type:       "method_not_allowed",
			Code:       fiber.StatusMethodNotAllowed,
			Loc:        []any{"method"},
			Field:      method,
			Msg:        fmt.Sprintf("method %s not allowed on %s; allowed: %s", method, path, strings.Join(allowed, ", ")),
			Constraint: strings.Join(allowed, ","),
		}},
		ResponseContext: ResponseContext{ResponseID: sanitizeRequestID(c.Get(requestIDHeader))},
	}
}

// defaultNotFoundHandler builds the closure installed when no user-supplied
// Config.NotFoundHandler is configured. The closure captures o.operations so it
// can emit 405 with an Allow header when the path exists on another method.
func (o *OApiApp) defaultNotFoundHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		// HEAD: HTTP forbids a body — emit just the status.
		if c.Method() == fiber.MethodHead {
			return c.SendStatus(fiber.StatusNotFound)
		}
		// OPTIONS: pass through so downstream CORS middleware can produce the
		// preflight response. If nothing else handles it, Fiber's stack returns
		// 404 naturally and that is the right outcome (the route does not exist).
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		shape := o.config.DefaultErrorShape
		// 405: the path exists on another method.
		if allowed := o.allowedMethodsFor(c.Path()); len(allowed) > 0 {
			c.Set(fiber.HeaderAllow, strings.Join(allowed, ", "))
			if shape != nil {
				cat := errorCategory{
					Code:    fiber.StatusMethodNotAllowed,
					Type:    "method_not_allowed",
					Message: fmt.Sprintf("method %s not allowed on %s", c.Method(), sanitizePath(c.Path())),
					Details: strings.Join(allowed, ", "),
				}
				return c.Status(fiber.StatusMethodNotAllowed).JSON(materializeError(shape, cat))
			}
			return c.Status(fiber.StatusMethodNotAllowed).JSON(methodNotAllowedEnvelope(c, allowed))
		}
		if shape != nil {
			cat := errorCategory{
				Code:    fiber.StatusNotFound,
				Type:    errTypeNotFound,
				Message: fmt.Sprintf("no route matches %s %s", c.Method(), sanitizePath(c.Path())),
			}
			return c.Status(fiber.StatusNotFound).JSON(materializeError(shape, cat))
		}
		return c.Status(fiber.StatusNotFound).JSON(NotFoundEnvelope(c))
	}
}

// allowedMethodsFor walks the registered operations and returns the HTTP
// methods that match the requested path (Fiber-style :param patterns supported).
func (o *OApiApp) allowedMethodsFor(path string) []string {
	seen := map[string]struct{}{}
	var allowed []string
	for _, op := range o.operations {
		if op.Method == "" {
			continue
		}
		if !matchFiberPath(op.Path, path) {
			continue
		}
		if _, dup := seen[op.Method]; dup {
			continue
		}
		seen[op.Method] = struct{}{}
		allowed = append(allowed, op.Method)
	}
	return allowed
}

// pathParamRegex captures Fiber's :name and {name} placeholders so we can turn
// a route pattern into a regex that matches one path segment per placeholder.
var pathParamRegex = regexp.MustCompile(`:[A-Za-z_][A-Za-z0-9_]*|\{[A-Za-z_][A-Za-z0-9_]*\}`)

// matchFiberPath returns true when the concrete `path` matches the route
// `pattern`. Only the common Fiber placeholder forms are supported (`:name`,
// `{name}`) — wildcards / regex constraints fall back to literal matching.
func matchFiberPath(pattern, path string) bool {
	// Fast path: literal equality.
	if pattern == path {
		return true
	}
	if !strings.ContainsAny(pattern, ":{") {
		return false
	}
	expr := "^" + pathParamRegex.ReplaceAllStringFunc(regexp.QuoteMeta(pattern), func(string) string {
		return "[^/]+"
	}) + "$"
	// The replacement runs on the QuoteMeta'd pattern, where `:` becomes `:`
	// (unchanged) and `{`/`}` become `\{`/`\}`. Strip those backslashes so the
	// regex sees the original delimiters.
	expr = strings.ReplaceAll(expr, `\{`, `{`)
	expr = strings.ReplaceAll(expr, `\}`, `}`)
	re, err := regexp.Compile(expr)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// UseNotFoundHandler installs a catch-all middleware that responds with the
// fiber-oapi ErrorEnvelope when no other route matches the request.
//
// Call this AFTER registering every route. The catch-all is installed via
// fiber.App.Use, which Fiber matches in registration order — install it
// before any route and that route will be unreachable. Calling the method more
// than once on the same OApiApp is a no-op after the first install.
//
// The default handler does three things beyond emitting the 404 envelope:
//   - HEAD requests get a bodyless 404 (HTTP-conformant).
//   - OPTIONS requests fall through to the next handler so downstream CORS
//     middleware can answer preflights.
//   - When the requested path is registered under another HTTP method, the
//     response is 405 with an Allow header listing the supported methods.
//
// To customise the response, set Config.NotFoundHandler. The handler runs in
// place of the default and receives a raw fiber.Ctx — it owns the entire
// response (status code and body). Call NotFoundEnvelope(c) to reuse the
// library's envelope shape from inside a custom handler.
func (o *OApiApp) UseNotFoundHandler() {
	if o.notFoundInstalled {
		return
	}
	handler := o.config.NotFoundHandler
	if handler == nil {
		handler = o.defaultNotFoundHandler()
	}
	o.f.Use(handler)
	o.notFoundInstalled = true
}

// DefaultNotFoundHandler returns the default envelope-producing fiber.Handler
// without any operation-aware 405 detection. Useful for users who manage their
// own fiber.Config and want to install the catch-all outside of fiber-oapi.
func DefaultNotFoundHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodHead {
			return c.SendStatus(fiber.StatusNotFound)
		}
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		return c.Status(fiber.StatusNotFound).JSON(NotFoundEnvelope(c))
	}
}

func exampleNotFoundEnvelope() ErrorEnvelope {
	return ErrorEnvelope{
		Errors: []ValidationErrorEntry{{
			Type:  errTypeNotFound,
			Code:  fiber.StatusNotFound,
			Loc:   []any{"path"},
			Field: "/users/42",
			Msg:   "no route matches GET /users/42",
		}},
		ResponseContext: ResponseContext{ResponseID: "bf0e9029-576b-42e8-84f9-ad0622972f50"},
	}
}

func exampleParseEnvelope() ErrorEnvelope {
	return ErrorEnvelope{
		Errors: []ValidationErrorEntry{{
			Type:       errTypeTypeMismatch,
			Code:       statusParseError,
			Loc:        []any{"body", "age"},
			Field:      "age",
			Msg:        "expected int but got string",
			Constraint: "int",
		}},
		ResponseContext: ResponseContext{ResponseID: "bf0e9029-576b-42e8-84f9-ad0622972f50"},
	}
}
