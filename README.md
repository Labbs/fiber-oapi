# Fiber OpenAPI

A Go library that extends Fiber to add automatic OpenAPI documentation generation with built-in validation, authentication, and role-based access control.

## Features

- **Complete HTTP methods** (GET, POST, PUT, PATCH, DELETE, HEAD) with automatic validation
- **Group support** with OpenAPI methods available on both app and groups
- **Unified API** with interface-based approach for seamless app/group usage
- **Powerful validation** via `github.com/go-playground/validator/v10`
- **Multiple authentication schemes**: Bearer, Basic Auth, API Key, AWS SigV4
- **Declarative role-based access control** with OR/AND semantics
- **Custom error handlers** for validation and authentication errors
- **Per-route security overrides** and public routes
- **Type safety** with Go generics
- **OpenAPI 3.0 documentation** in JSON and YAML formats
- **Redoc documentation UI** for modern, responsive API documentation
- **OpenAPI extensions** (`x-required-roles`, `x-required-roles-mode`)
- **Conditional auth middleware** for flexible authentication strategies

## Installation

```bash
go get github.com/labbs/fiber-oapi/v3
```

> **What's new in v3.3**
> - [`AuthError.Err`](#authenticationauthorization-errors): the cause of an authentication failure (`errors.Is` / `errors.As`), and its status through `HTTPStatus()`.
> - [`TokenValidatorFunc` and `ContextTokenValidator`](#token-only-services-tokenvalidatorfunc): an auth service that only validates tokens.
> - [Problem details (RFC 9457)](#problem-details-rfc-9457): `Problem`, `ContentTyper`, `ProblemDetails`, `ValidationStatusCode`, `ProblemCodes`.
> - [Raw request bodies](#raw-request-body-binary): `body:"raw"` and `RequestContentType`.
> - [Responses written by the handler](#responses-written-by-the-handler-binary-streams-sse): `Responder`, `RawResponse`, `ResponseContentType`, Server-Sent Events.
> - [Success status](#success-status-201-204-and-extra-responses): `SuccessStatus` (201, 204 without body) and `Responses`.
> - [Route middlewares](#route-middlewares): `OpenAPIOptions.Middlewares`.
> - Protected routes document their 401, and 403 when they check roles, scopes or resources.
>
> Everything above is opt-in, except **three changes of behavior**:
> 1. **[Authentication runs first.](#authentication-runs-first)** A protected route checks the credentials before reading its input: a request that fails authentication gets its 401 without its body being decoded, even if that body is malformed or invalid (it used to get a 400 or 422).
> 2. **Path, query and header values now win over the body.** The body used to be decoded after the path and query, and the JSON decoder matches keys case-insensitively: a field tagged `uri:"id"` without `json:"-"` could be overwritten by `{"id": "..."}` in the body. The body is now decoded first.
> 3. **`uri` fields no longer appear in the documented request body.** They were already documented as path parameters.
>
> Also: `SmartAuthMiddleware` and `MultiSchemeAuthMiddleware` now go through `Config.AuthErrorHandler` when one is set.

> **Upgrading from v1.x?** v3 tracks Fiber v3 and requires Go 1.27+. Breaking changes:
> - Handlers now take `fiber.Ctx` (struct value) instead of `*fiber.Ctx`.
> - The path-parameter struct tag is now `uri:` instead of `path:` (Fiber v3 binder convention). Query and header tags are unchanged.
> - The default validation/parse error response uses a new `ErrorEnvelope` shape (one entry per failing field, `response_context.response_id` mirrored from `X-Request-Id`) and validation errors are returned as **422 Unprocessable Entity** instead of 400. See [Error responses](#error-responses).

## Quick Start

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    fiberoapi "github.com/labbs/fiber-oapi/v3"
)

func main() {
    app := fiber.New()
    oapi := fiberoapi.New(app)

    fiberoapi.Get(oapi, "/hello/:name",
        func(c fiber.Ctx, input struct {
            Name string `uri:"name" validate:"required,min=2"`
        }) (fiber.Map, *fiberoapi.ErrorResponse) {
            return fiber.Map{"message": "Hello " + input.Name}, nil
        },
        fiberoapi.OpenAPIOptions{
            Summary: "Say hello",
            Tags:    []string{"greeting"},
        })

    // Docs at /docs, spec at /openapi.json and /openapi.yaml
    app.Listen(":3000")
}
```

## Configuration

```go
type Config struct {
    EnableValidation       bool                      // Enable input validation (default: true)
    EnableOpenAPIDocs      bool                      // Enable automatic docs setup (default: true)
    EnableAuthorization    bool                      // Enable auth validation (default: false)
    OpenAPIDocsPath        string                    // Path for docs UI (default: "/docs")
    OpenAPIJSONPath        string                    // Path for JSON spec (default: "/openapi.json")
    OpenAPIYamlPath        string                    // Path for YAML spec (default: "/openapi.yaml")
    OpenAPITitle           string                    // Spec title (default: "Fiber OpenAPI")
    OpenAPIDescription     string                    // Spec description (default: "API documentation generated by fiber-oapi")
    OpenAPIVersion         string                    // Spec version (default: "1.0.0")
    AuthService            AuthorizationService      // Service for handling auth
    SecuritySchemes        map[string]SecurityScheme // OpenAPI security schemes
    DefaultSecurity        []map[string][]string     // Default security requirements
    ValidationErrorHandler ValidationErrorHandler    // Custom handler for validation errors
    AuthErrorHandler       AuthErrorHandler          // Custom handler for auth errors (401/403/5xx)
    NotFoundHandler        fiber.Handler             // Custom handler for unmatched routes (see UseNotFoundHandler)
    DefaultErrorShape      any                       // One shape for library errors (see below)
    IncludeInvalidValueInErrors bool                 // Echo the offending value in error entries (default: false)

    ProblemDetails         bool                      // Library errors as RFC 9457 problems (default: false)
    ValidationStatusCode   int                       // Status of validation errors (default: 422)
    ProblemCodes           map[string]string         // Replace the Code of a problem, per category
}
```

Default config when none is provided:
- Validation: **enabled**
- Documentation: **enabled**
- Authorization: **disabled**
- Docs path: `/docs`
- JSON spec path: `/openapi.json`
- YAML spec path: `/openapi.yaml`
- Spec title: `Fiber OpenAPI`
- Spec description: `API documentation generated by fiber-oapi`
- Spec version: `1.0.0`

Override the `info` block of the generated spec via `Config`:

```go
fiberoapi.New(app, fiberoapi.Config{
    OpenAPITitle:       "My Service",
    OpenAPIDescription: "Public REST API for the My Service platform",
    OpenAPIVersion:     "2024.10.1",
})
```

## HTTP Methods

All methods work with both the main app and groups:

```go
fiberoapi.Get(router, path, handler, options)
fiberoapi.Post(router, path, handler, options)
fiberoapi.Put(router, path, handler, options)
fiberoapi.Patch(router, path, handler, options)
fiberoapi.Delete(router, path, handler, options)
fiberoapi.Head(router, path, handler, options)
fiberoapi.Method(method, router, path, handler, options) // Custom HTTP method
```

## Parameter Types

```go
type MyInput struct {
    ID     string `uri:"id" validate:"required"`           // Path parameter
    Filter string `query:"filter" validate:"omitempty"`      // Query parameter
    Auth   string `header:"Authorization"`                   // Header parameter
    Title  string `json:"title" validate:"required,min=1"`   // JSON body field
}
```

The body is decoded first, then the path, query and header values are bound:
they always win over a body key with the same name.

Special tags:
- `body:"raw"` — Receive the request body as-is in a `[]byte` field (see [Raw request body](#raw-request-body-binary))
- `openapi:"-"` — Exclude a field from the OpenAPI schema (the field still works in the handler)
- `description:"text"` — Add a description to the field in the spec
- `resource:"document"` — Mark field as a resource identifier for dynamic authorization
- `action:"write"` — Specify the action for resource access checks

### Raw request body (binary)

Fiber's body binder only knows structured formats (JSON, XML, forms...). To
receive a body as-is — a file, an encrypted package — tag a `[]byte` field with
`body:"raw"`:

```go
type PutBlobInput struct {
    ID          string `uri:"id" validate:"required,uuid"`
    IfNoneMatch string `header:"If-None-Match" validate:"required,eq=*"`
    Body        []byte `body:"raw" validate:"required,max=10485760"` // the body, as-is
}

fiberoapi.Put(v1, "/blobs/:id", ctrl.PutBlob, fiberoapi.OpenAPIOptions{
    RequestContentType: "application/octet-stream", // default for a raw body
})
```

- The body is copied (Fiber reuses its buffers), whatever its `Content-Type`.
- An empty body leaves the field `nil`, so `required` rejects it; `min` / `max`
  apply to its length.
- A struct can have only one `body:"raw"` field, and it must be a `[]byte`:
  anything else panics when the route is registered.
- The spec documents the body as `{type: string, format: binary}` under
  `RequestContentType`. `uri`, `query` and `header` fields stay parameters.
  For a JSON input, `RequestContentType` sets the media type of the JSON body.

The whole body is read before the handler runs: `fiber.Config.BodyLimit`
(4 MiB by default) must be at least the largest body you accept. A route
middleware can enforce a lower limit on one route.

## Success status (201, 204) and extra responses

```go
fiberoapi.Post(v1, "/users", ctrl.CreateUser, fiberoapi.OpenAPIOptions{
    SuccessStatus: 201,
})

fiberoapi.Post(v1, "/auth/logout", ctrl.Logout, fiberoapi.OpenAPIOptions{
    SuccessStatus: 204, // sent without a body, documented without content
})

fiberoapi.Put(v1, "/blobs/:id", ctrl.PutBlob, fiberoapi.OpenAPIOptions{
    SuccessStatus: 201,
    Responses: map[string]any{ // OpenAPI response objects, merged as-is
        "200": map[string]any{"description": "Same content already present"},
    },
})
```

At runtime, the status of a successful response is, in order:

1. a status other than 200 set by the handler with `c.Status(...)`;
2. the `HTTPStatus() int` of the output, when it implements it — e.g. 201 for
   a new resource, 200 for an existing one;
3. `SuccessStatus`;
4. 200.

The spec documents the success response under `SuccessStatus`. `Responses`
is merged last and replaces any generated response with the same key.

## Route middlewares

`OpenAPIOptions.Middlewares` runs handlers on one route only, before its input
is read — after the group middlewares and, on a protected route, after
[authentication](#authentication-runs-first). A middleware can answer on its
own to stop the chain:

```go
fiberoapi.Post(v1, "/auth/login", ctrl.Login, fiberoapi.OpenAPIOptions{
    Security:    "disabled",
    Middlewares: []fiber.Handler{limiter.New(limiter.Config{Max: 5})}, // 429
})
```

## Groups

```go
app := fiber.New()
oapi := fiberoapi.New(app)

v1 := fiberoapi.Group(oapi, "/api/v1")
users := fiberoapi.Group(v1, "/users")

// OpenAPI methods on groups
fiberoapi.Get(users, "/:id", getUser, options)   // Registers as GET /api/v1/users/{id}
fiberoapi.Post(users, "/", createUser, options)

// Standard Fiber middleware still works
v1.Use(authMiddleware)
```

## Validation

Uses `validator/v10`. Common tags:

```go
type Input struct {
    Name   string `json:"name" validate:"required,min=3,max=50"`
    Email  string `json:"email" validate:"required,email"`
    Age    int    `json:"age" validate:"min=13,max=120"`
    Role   string `json:"role" validate:"oneof=admin user guest"`
    Tags   []string `json:"tags" validate:"dive,min=1"`
}
```

## Responses written by the handler (binary, streams, SSE)

A handler normally returns a value that the library encodes as JSON. When the
output implements `Responder`, the library calls its `Respond` method instead,
and the output writes its own status, headers and body. The success status of
the route (see [Success status](#success-status-201-204-and-extra-responses))
is set before `Respond` runs, and `Respond` may override it.

```go
type Responder interface {
    Respond(c fiber.Ctx) error
}
```

`RawResponse` covers the common case of a binary body:

```go
fiberoapi.Get(v1, "/blobs/:id",
    func(c fiber.Ctx, in GetBlobInput) (*fiberoapi.RawResponse, error) {
        data, err := store.Get(in.ID)
        if err != nil {
            return nil, err
        }
        return &fiberoapi.RawResponse{
            ContentType: "application/octet-stream", // default
            Body:        data,
            // Status: 0 keeps the success status of the route (200 by default)
        }, nil
    },
    fiberoapi.OpenAPIOptions{ResponseContentType: "application/octet-stream"},
)
```

In the spec, the success response of a `Responder` route is documented under
`ResponseContentType` (`application/octet-stream` by default) with
`{type: string, format: binary}`, or `{type: string}` for a `text/...` media
type. For a regular JSON output, `ResponseContentType` sets the media type the
JSON is sent and documented with (e.g. `application/vnd.api+json`).

### Server-Sent Events

A Server-Sent Events stream is a `Responder` written by the application:

```go
// EventStream sends the events of one subscription until the client goes
// away or the server ends the subscription.
type EventStream struct {
    events <-chan Event
    cancel func()
}

func (s *EventStream) Respond(c fiber.Ctx) error {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)

    // The writer runs after the handler has returned: do not use c inside it.
    return c.SendStreamWriter(func(w *bufio.Writer) {
        defer s.cancel()
        ping := time.NewTicker(15 * time.Second)
        defer ping.Stop()
        for {
            select {
            case event, ok := <-s.events:
                if !ok {
                    return // the server closes the stream
                }
                fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Data)
            case <-ping.C:
                fmt.Fprint(w, ": ping\n\n") // keeps idle connections open
            }
            if err := w.Flush(); err != nil {
                return // the client is gone
            }
        }
    })
}

fiberoapi.Get(v1, "/sync/events",
    func(c fiber.Ctx, _ struct{}) (*EventStream, error) {
        events, cancel := broker.Subscribe()
        return &EventStream{events: events, cancel: cancel}, nil
    },
    fiberoapi.OpenAPIOptions{
        Summary:             "Change feed",
        ResponseContentType: "text/event-stream",
    },
)
```

Two settings live outside the library and must be handled by the application:

- **Compression.** The `compress` middleware buffers the response, so events
  would never reach the client. Exclude the stream from it:
  `compress.New(compress.Config{Next: func(c fiber.Ctx) bool { return c.Path() == "/v1/sync/events" }})`.
- **`WriteTimeout`.** `fiber.Config.WriteTimeout` bounds the whole response,
  stream included. Leave it at 0 or above the longest stream you serve, or end
  streams on the server side periodically and let clients reconnect
  (`EventSource` does so automatically, sending `Last-Event-ID`).

## Error responses

When the default validation / parse handler runs (i.e. no custom `ValidationErrorHandler`
is configured), errors are returned as a structured envelope with one entry per
failing field:

```json
{
  "errors": [
    {
      "type": "validation_error",
      "code": 422,
      "loc": ["body", "workspaceId"],
      "field": "workspaceId",
      "msg": "field 'workspaceId' must be at least 11",
      "constraint": "min=11"
    },
    {
      "type": "validation_error",
      "code": 422,
      "loc": ["body", "nested", "slug"],
      "field": "slug",
      "msg": "field 'slug' must be at least 2",
      "constraint": "min=2"
    }
  ],
  "response_context": {
    "response_id": "bf0e9029-576b-42e8-84f9-ad0622972f50"
  }
}
```

Status codes used by the default handler:

| Code | Category | Entry `type` |
|------|----------|--------------|
| 422 | Failed validation rules (`Config.ValidationStatusCode` to change it) | `validation_error` |
| 400 | JSON parse / type mismatch | `type_error`, `parse_error` |
| 401 / 403 | Authentication / authorization | `authentication_error`, `authorization_error` |

`response_context.response_id` mirrors the `X-Request-Id` request header when
present (no UUID is generated by the lib — pair with a `requestid` middleware
if you want one). The `loc` array starts with the request source (`body`, `path`,
`query`, `header`) followed by the field path using JSON / URI / header tag names.

By default the offending value is omitted to avoid leaking secrets (e.g. a
password failing `min=8` validation). Opt in via `Config.IncludeInvalidValueInErrors: true`
if you want each entry to carry a `value` field.

The OpenAPI spec exposes `ErrorEnvelope` / `ValidationErrorEntry` / `ResponseContext`
under `components.schemas` and adds a 422 response with a realistic example to
every operation, plus a 400 example for body-carrying methods, a 401 for
protected routes, and a 403 for those that check roles, scopes or resources. Routes that
declare a non-empty `TError` and no per-status `OpenAPIOptions.Errors` keep their
domain shape under the catch-all `4XX` response — as soon as the route declares
explicit `Errors` entries the catch-all is suppressed and only the enumerated
status codes appear in the spec.

#### Hiding a route from the spec entirely

Some routes — internal admin endpoints, debug handlers, in-progress features —
serve traffic but shouldn't appear in the published spec. Set
`OpenAPIOptions.Hidden = true`:

```go
fiberoapi.Get(oapi, "/admin/debug/:id",
    controller.Debug,
    fiberoapi.OpenAPIOptions{
        OperationID: "internalDebug",
        Hidden:      true, // ← serves traffic, absent from /openapi.json
    },
)
```

The path entry is omitted and any type only referenced by hidden routes is
skipped from `components.schemas` (so reading the spec doesn't leak the shape
of internal endpoints). Types used by both hidden and visible routes still
surface — the visible route needs them.

#### Opting out of all default error responses

Some endpoints — health checks, readiness probes, anything with no body or no
error contract — don't need the framework's 400 / 422 / 404 / 4XX in the spec.
Pass an explicit empty `Errors` slice to suppress every default and document
only the 200 success path:

```go
fiberoapi.Get(oapi, "/health",
    func(c fiber.Ctx, _ struct{}) (HealthStatus, struct{}) {
        return HealthStatus{Status: "ok"}, struct{}{}
    },
    fiberoapi.OpenAPIOptions{
        Summary: "Liveness probe",
        Errors:  []any{}, // ← explicit "no errors for this route"
    },
)
```

The distinction matters: `Errors: nil` (the zero value) keeps the defaults,
`Errors: []any{}` (an explicitly empty slice) suppresses them. A slice that
contains only nil entries is treated as nil for back-compat.

If you need a different shape, set `Config.ValidationErrorHandler` / `Config.AuthErrorHandler`
— they receive the raw error (JSON type mismatches are wrapped so `err.Error()`
stays friendly, but `var ute *json.UnmarshalTypeError; errors.As(err, &ute)` still recovers
the original).

### Problem details (RFC 9457)

Set `Config.ProblemDetails` and every error produced by the library — parsing,
validation, authentication, 404/405, serialization — is a `Problem`, sent as
`application/problem+json` and documented as such in the spec:

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    EnableValidation:     true,
    ProblemDetails:       true,
    ValidationStatusCode: 400, // default 422, at runtime and in the spec
    ProblemCodes: map[string]string{ // default: the category itself
        "validation_error": "invalid_request",
        "parse_error":      "invalid_request",
        "type_error":       "invalid_request",
    },
})
```

```json
{
  "title": "Bad Request",
  "status": 400,
  "detail": "field 'name' must be at least 3",
  "code": "invalid_request",
  "errors": [
    {"type": "validation_error", "code": 400, "loc": ["body", "name"], "field": "name",
     "msg": "field 'name' must be at least 3", "constraint": "min=3"}
  ]
}
```

`code` and `errors` are extension members: a stable identifier, and the
per-field entries of validation and type errors. The categories are
`validation_error`, `parse_error`, `type_error`, `authentication_error`,
`authorization_error`, `not_found`, `method_not_allowed`,
`serialization_error` and `internal_error`.

`ProblemDetails` takes precedence over `DefaultErrorShape`.
`ValidationErrorHandler`, `AuthErrorHandler` and `NotFoundHandler` keep
priority; they can answer with a problem of their own:

```go
AuthErrorHandler: func(c fiber.Ctx, err *fiberoapi.AuthError) error {
    p := &fiberoapi.Problem{Title: "Unauthorized", Status: err.StatusCode, Code: "token_expired"}
    return c.Status(p.Status).JSON(p, p.ContentType())
},
```

Any error value — a `TError`, an entry of `OpenAPIOptions.Errors` — can choose
its `Content-Type` by implementing `ContentTyper`; `Problem` answers
`application/problem+json`. A handler can therefore return a `*Problem` as its
`TError`:

```go
type ContentTyper interface{ ContentType() string }
```

### Unifying all errors under one shape (`DefaultErrorShape`)

By default, the library emits its own `ErrorEnvelope` shape for framework-level
errors (parse, validation, auth, route-miss) while your custom errors use
whatever struct you declared. That mismatch shows up in tools like Redoc /
Stoplight as two distinct schemas per endpoint.

Set `Config.DefaultErrorShape` to a (zero) instance of your error type and the
library uses that shape everywhere — both runtime responses and spec entries:

```go
type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
    Type    string `json:"type"`
}

oapi := fiberoapi.New(app, fiberoapi.Config{
    DefaultErrorShape: &ErrorResponse{}, // empty template — fields are filled per error
})
```

For each library-emitted error, the matching fields on your shape are populated
via reflection (case-sensitive, applied only if present and settable):

| Field | Source |
|-------|--------|
| `StatusCode` or `Code` | HTTP status code (400, 401, 403, 404, 405) |
| `Message`, `Description`, or `Msg` | one-line human-readable summary |
| `Type` | discriminator (`type_error`, `parse_error`, `authentication_error`, `authorization_error`, `not_found`, `method_not_allowed`) |
| `Details` | joined extra context (allowed methods for 405, …) |

**One exception:** the 422 validation response keeps the rich `ErrorEnvelope`
shape (one entry per failing field with `loc` / `constraint` / `field` /
`value`). Collapsing a multi-field validation failure into a single flat struct
would lose the structured info that form-level UX needs. If you really want a
flat 422, declare your own entry at status 422 in `OpenAPIOptions.Errors` — the
per-route override still wins.

Result: in your spec every error response references `#/components/schemas/ErrorResponse`
**except 422**, which stays on `#/components/schemas/ErrorEnvelope`.

`DefaultErrorShape` is ignored when `ProblemDetails` is set.

Per-route entries declared via `OpenAPIOptions.Errors` still take precedence
for their status code — so you can selectively override the default shape on a
specific endpoint if you ever need to.

### Custom domain errors (declared per route, visible in the spec)

For handler-emitted errors (conflict, not-found, precondition-failed, …),
declare them via `OpenAPIOptions.Errors`. Each entry is an instance of any
struct describing one error case — the library inspects it to populate the
generated OpenAPI spec and the handler returns the same instance at runtime.

```go
// One shared type in your app — all custom errors funnel through it so the
// spec has a single component schema and clients see one consistent shape.
type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
    Type    string `json:"type"`
}

func (e *ErrorResponse) Error() string { return e.Message } // optional

func UserAlreadyExists(name string) *ErrorResponse {
    return &ErrorResponse{Code: 409, Message: fmt.Sprintf("user %q already exists", name), Type: "Conflict"}
}

func UserNotFound(name string) *ErrorResponse {
    return &ErrorResponse{Code: 404, Message: fmt.Sprintf("user %q not found", name), Type: "NotFound"}
}

fiberoapi.Post(oapi, "/users/:name",
    func(c fiber.Ctx, in CreateUserInput) (CreateUserOutput, error) {
        if in.Name == "admin" {
            return CreateUserOutput{}, UserAlreadyExists(in.Name)
        }
        return CreateUserOutput{Message: "ok"}, nil
    },
    fiberoapi.OpenAPIOptions{
        Errors: []any{
            UserAlreadyExists("admin"), // becomes the 409 example in the spec
            UserNotFound("ghost"),      // becomes the 404 example in the spec
        },
    },
)
```

How each entry maps to the spec:

| Source | Extracted via |
|--------|---------------|
| Status code | `HTTPStatus() int` method, else `StatusCode` / `Code` int field, else `500` |
| Description | `Description() string` method, else `Message` / `Description` / `Msg` string field, else HTTP reason phrase |
| Schema | `reflect.TypeOf(entry)` — named types use `$ref` to `components.schemas` so multiple entries with the same type stay deduplicated |
| Example | the entry value itself, JSON-marshalled |

The handler's return type can be `error` (when the entry implements `error`),
`*ErrorResponse`, or any other concrete type — the library uses reflection,
not a type assertion, to read the status code.

Multiple entries at the same status code: the last one wins (typical when
mixing `Errors` with the auto-emitted default envelopes). Use this to
deliberately override the default `404` (route-miss) envelope with your own
domain-404 shape for routes that report "resource not found".

### 404 Not Found

The same envelope is produced for unmatched routes when you opt in via
`oapi.UseNotFoundHandler()`. **Call it after registering every route** — under
the hood it installs a catch-all `app.Use(handler)` middleware in Fiber, which
is matched in registration order, so it must come last to avoid swallowing real
routes.

```go
app := fiber.New()
oapi := fiberoapi.New(app)

fiberoapi.Get(oapi, "/users/:id", getUser, opts)
fiberoapi.Post(oapi, "/users", createUser, opts)
// ... every other route ...

oapi.UseNotFoundHandler() // ← last
app.Listen(":3000")
```

Response shape:

```json
{
  "errors": [
    {
      "type": "not_found",
      "code": 404,
      "loc": ["path"],
      "field": "/users/42",
      "msg": "no route matches GET /users/42"
    }
  ],
  "response_context": { "response_id": "bf0e9029-..." }
}
```

The default handler does more than just emit the 404 envelope:

- **HEAD** requests get a bodyless 404 (HTTP-conformant).
- **OPTIONS** requests fall through (`c.Next()`) so downstream CORS middleware
  can answer preflights.
- When the path is registered under another HTTP method, the response is **405**
  with an `Allow` header listing the supported methods and the envelope's
  entry `type` set to `method_not_allowed`.
- The `X-Request-Id` header is sanitised before being echoed (max 128 bytes,
  charset `[A-Za-z0-9._\-:]`) — invalid values are dropped to neutralise
  log-injection vectors.
- The echoed path is capped at ~1 KiB and validated as UTF-8.

Calling `UseNotFoundHandler()` more than once on the same `OApiApp` is a no-op
after the first install. Once installed, the generated OpenAPI spec also lists
a 404 response on every operation (referencing `ErrorEnvelope`) so the contract
is documented for clients.

Override the handler entirely via `Config.NotFoundHandler`. Your handler
receives a raw `fiber.Ctx` and owns the full response — call
`fiberoapi.NotFoundEnvelope(c)` to reuse the library's shape from inside it.
For users managing their own `fiber.Config`, `fiberoapi.DefaultNotFoundHandler()`
returns a no-op-405 version of the catch-all you can install yourself.

## Authentication & Authorization

### Supported Security Schemes

| Scheme | Config | Validator Interface |
|--------|--------|-------------------|
| Bearer Token | `Type: "http", Scheme: "bearer"` | `AuthorizationService` (built-in) |
| HTTP Basic | `Type: "http", Scheme: "basic"` | `BasicAuthValidator` |
| API Key | `Type: "apiKey", In: "header"/"query"/"cookie"` | `APIKeyValidator` |
| AWS SigV4 | `Type: "http", Scheme: "AWS4-HMAC-SHA256"` | `AWSSignatureValidator` |

### Setup

Implement `AuthorizationService` (required) and any additional validator interfaces:

```go
type MyAuthService struct{}

// Required: AuthorizationService
func (s *MyAuthService) ValidateToken(token string) (*fiberoapi.AuthContext, error) { ... }
func (s *MyAuthService) HasRole(ctx *fiberoapi.AuthContext, role string) bool { ... }
func (s *MyAuthService) HasScope(ctx *fiberoapi.AuthContext, scope string) bool { ... }
func (s *MyAuthService) CanAccessResource(ctx *fiberoapi.AuthContext, resourceType, resourceID, action string) (bool, error) { ... }
func (s *MyAuthService) GetUserPermissions(ctx *fiberoapi.AuthContext, resourceType, resourceID string) (*fiberoapi.ResourcePermission, error) { ... }

// Optional: BasicAuthValidator
func (s *MyAuthService) ValidateBasicAuth(username, password string) (*fiberoapi.AuthContext, error) { ... }

// Optional: APIKeyValidator
func (s *MyAuthService) ValidateAPIKey(key, location, paramName string) (*fiberoapi.AuthContext, error) { ... }

// Optional: AWSSignatureValidator
func (s *MyAuthService) ValidateAWSSignature(params *fiberoapi.AWSSignatureParams) (*fiberoapi.AuthContext, error) { ... }
```

Configure multiple security schemes (OR semantics between them):

```go
config := fiberoapi.Config{
    EnableAuthorization: true,
    AuthService:         &MyAuthService{},
    SecuritySchemes: map[string]fiberoapi.SecurityScheme{
        "bearerAuth": {
            Type:         "http",
            Scheme:       "bearer",
            BearerFormat: "JWT",
            Description:  "JWT Bearer token",
        },
        "basicAuth": {
            Type:        "http",
            Scheme:      "basic",
            Description: "HTTP Basic authentication",
        },
        "apiKeyAuth": {
            Type:        "apiKey",
            In:          "header",
            Name:        "X-API-Key",
            Description: "API Key via header",
        },
    },
    // Any of these schemes can authenticate a request (OR semantics)
    DefaultSecurity: []map[string][]string{
        {"bearerAuth": {}},
        {"basicAuth": {}},
        {"apiKeyAuth": {}},
    },
}
oapi := fiberoapi.New(app, config)
```

### Token-only services (`TokenValidatorFunc`)

An application that only validates tokens does not need to implement the whole
`AuthorizationService`:

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    EnableAuthorization: true,
    AuthService: fiberoapi.TokenValidatorFunc(func(token string) (*fiberoapi.AuthContext, error) {
        return tokens.Validate(token) // tokens.ErrExpired, tokens.ErrDeviceRevoked...
    }),
})
```

`HasRole` and `HasScope` read the roles and scopes of the returned
`AuthContext`. Resource permissions are not supported: a route that needs them
answers 500 (`ErrResourceAuthorizationNotConfigured`), never an access granted
by default.

A service that needs the request to validate a Bearer token (client IP,
request context...) can implement `ContextTokenValidator`; it is then used
instead of `ValidateToken`:

```go
type ContextTokenValidator interface {
    ValidateTokenCtx(c fiber.Ctx, token string) (*fiberoapi.AuthContext, error)
}
```

### Authentication runs first

On a protected route, the credentials are checked before anything reads the
request, in a handler placed at the head of the route:

1. authentication (401 on failure);
2. the [route middlewares](#route-middlewares);
3. reading the input (path, query, body, headers) and validating it (400, 422);
4. roles and resource permissions (403);
5. the handler.

Group middlewares (`Group(prefix, handlers...)`, `Use`) run before step 1.

- A request without valid credentials gets its 401 before its body is
  decoded: no work is spent on a request that will be rejected, and an
  unauthenticated client cannot probe the validation rules of the route.
- [Route middlewares](#route-middlewares) run after authentication and can call
  `GetAuthContext(c)`.
- Roles and resource permissions are still checked after the input is parsed,
  since they need it.
- Routes with `Security: "disabled"` are not authenticated.

### Public vs Protected Routes

```go
// Public route — no authentication
fiberoapi.Get(oapi, "/health", handler,
    fiberoapi.OpenAPIOptions{
        Summary:  "Health check",
        Security: "disabled",
    })

// Protected route — uses default security
fiberoapi.Get(oapi, "/profile", handler,
    fiberoapi.OpenAPIOptions{
        Summary: "Get profile",
    })

// Per-route security override
fiberoapi.Get(oapi, "/admin", handler,
    fiberoapi.WithSecurity(
        fiberoapi.OpenAPIOptions{Summary: "Admin endpoint"},
        []map[string][]string{{"bearerAuth": {}}}, // Only bearer, not API key
    ))
```

### Declarative Role-Based Access Control

Roles are checked automatically before your handler runs. No manual checks needed.

```go
// OR semantics: user needs at least ONE of the listed roles
fiberoapi.Get(oapi, "/documents/:id", handler,
    fiberoapi.WithRoles(
        fiberoapi.OpenAPIOptions{Summary: "Get document", Tags: []string{"documents"}},
        "admin", "editor",  // admin OR editor can access
    ))

// AND semantics: user needs ALL of the listed roles
fiberoapi.Delete(oapi, "/documents/:id", handler,
    fiberoapi.WithAllRoles(
        fiberoapi.OpenAPIOptions{Summary: "Delete document", Tags: []string{"documents"}},
        "admin", "moderator",  // must be admin AND moderator
    ))

// Inline via OpenAPIOptions
fiberoapi.Get(oapi, "/settings", handler,
    fiberoapi.OpenAPIOptions{
        Summary:         "Settings",
        RequiredRoles:   []string{"admin", "superadmin"},
        RequireAllRoles: false,  // OR semantics (default)
    })
```

Roles appear in the OpenAPI spec as extensions:

```json
{
    "x-required-roles": ["admin", "editor"],
    "x-required-roles-mode": "any"
}
```

### Permissions and Resource Access

```go
// RequiredPermissions are documented in the OpenAPI spec description
fiberoapi.Put(oapi, "/documents/:id", handler,
    fiberoapi.OpenAPIOptions{
        Summary:             "Update document",
        RequiredRoles:       []string{"editor"},
        RequiredPermissions: []string{"document:write"},
    })

// Resource-based access via struct tags
type UpdateDocInput struct {
    DocumentID string `uri:"documentId" validate:"required" resource:"document" action:"write"`
    Title      string `json:"title" validate:"required"`
}

// Dynamic resource access check in handler
fiberoapi.RequireResourceAccess(c, authService, "document", docID, "delete")
```

### Authentication Context

Access the authenticated user in handlers:

```go
fiberoapi.Get(oapi, "/me", func(c fiber.Ctx, input struct{}) (fiber.Map, *fiberoapi.ErrorResponse) {
    authCtx, err := fiberoapi.GetAuthContext(c)
    if err != nil {
        return nil, &fiberoapi.ErrorResponse{Code: 401, Details: "Not authenticated"}
    }
    return fiber.Map{
        "user_id": authCtx.UserID,
        "roles":   authCtx.Roles,
        "scopes":  authCtx.Scopes,
        "claims":  authCtx.Claims,
    }, nil
}, fiberoapi.OpenAPIOptions{Summary: "Current user"})
```

## Custom Error Handlers

### Validation Errors

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    ValidationErrorHandler: func(c fiber.Ctx, err error) error {
        return c.Status(400).JSON(fiber.Map{
            "success": false,
            "error":   err.Error(),
        })
    },
})
```

### Authentication/Authorization Errors

```go
oapi := fiberoapi.New(app, fiberoapi.Config{
    EnableAuthorization: true,
    AuthService:         authService,
    AuthErrorHandler: func(c fiber.Ctx, err *fiberoapi.AuthError) error {
        // err.StatusCode: 401, 403, or 5xx
        // err.Message: human-readable error message
        return c.Status(err.StatusCode).JSON(fiber.Map{
            "error":   err.Message,
            "status":  err.StatusCode,
        })
    },
})
```

`AuthError` keeps the error returned by the validator (`ValidateToken`,
`ValidateAPIKey`...) as its cause, so the handler can tell failures apart
without comparing messages:

```go
AuthErrorHandler: func(c fiber.Ctx, err *fiberoapi.AuthError) error {
    code := "unauthorized"
    switch {
    case errors.Is(err, tokens.ErrExpired):
        code = "token_expired"
    case errors.Is(err, tokens.ErrDeviceRevoked):
        code = "device_revoked"
    }
    return c.Status(err.StatusCode).JSON(fiber.Map{"code": code})
},
```

The status is 401 (403 for a missing scope), unless the cause implements
`HTTPStatus() int`, in which case its status is used.

`SmartAuthMiddleware` and `MultiSchemeAuthMiddleware` go through
`AuthErrorHandler` too, and answer with a problem when `ProblemDetails` is set.

Without custom handlers, authentication and authorization failures use the
default [error envelope](#error-responses), with an entry of type
`authentication_error` (401) or `authorization_error` (403).

## Conditional Auth Middleware

Standalone middleware functions for use outside the declarative route system:

```go
// Smart middleware that auto-detects security schemes and excludes doc routes
app.Use(fiberoapi.SmartAuthMiddleware(authService, config))

// Skip auth for specific paths
app.Use(fiberoapi.ConditionalAuthMiddleware(
    fiberoapi.BearerTokenMiddleware(authService),
    "/health", "/docs", "/openapi.json",
))

// Individual scheme middleware
app.Use(fiberoapi.BearerTokenMiddleware(authService))
app.Use(fiberoapi.BasicAuthMiddleware(authService))
app.Use(fiberoapi.APIKeyMiddleware(authService, scheme))
app.Use(fiberoapi.AWSSignatureMiddleware(authService))

// Role guard middleware
app.Use(fiberoapi.RoleGuard(authService, "admin"))
```

## Security Helpers

```go
opts := fiberoapi.OpenAPIOptions{Summary: "My endpoint"}

// Security
opts = fiberoapi.WithSecurity(opts, []map[string][]string{{"bearerAuth": {}}})
opts = fiberoapi.WithSecurityDisabled(opts)

// Roles
opts = fiberoapi.WithRoles(opts, "admin", "editor")       // OR semantics
opts = fiberoapi.WithAllRoles(opts, "admin", "moderator")  // AND semantics

// Documentation
opts = fiberoapi.WithPermissions(opts, "document:read", "document:write")
opts = fiberoapi.WithResourceType(opts, "document")
```

## OpenAPI Spec Generation

The spec is available in both JSON and YAML:

```go
// Automatic endpoints
// GET /openapi.json
// GET /openapi.yaml
// GET /docs (Redoc UI)

// Programmatic access
spec := oapi.GenerateOpenAPISpec()           // map[string]interface{}
yamlSpec, err := oapi.GenerateOpenAPISpecYAML() // string
```

### Custom Documentation

```go
oapi.SetupDocs(fiberoapi.DocConfig{
    Title:       "My API",
    Description: "My API description",
    Version:     "2.0.0",
    DocsPath:    "/documentation",
    JSONPath:    "/api-spec.json",
})
```

## Testing

```bash
# Run all tests
go test -v ./...

# Run the auth example
go run _examples/auth/main.go
# Visit http://localhost:3002/docs
```

Testing with authentication:

```bash
# Bearer token
curl -H "Authorization: Bearer admin-token" http://localhost:3002/me

# Basic auth
curl --user admin:admin-pass http://localhost:3002/me

# API key
curl -H "X-API-Key: my-secret-api-key" http://localhost:3002/documents/doc-1

# Public endpoint
curl http://localhost:3002/health
```

## Complete Example

See `_examples/auth/main.go` for a full working example with:
- Multiple security schemes (Bearer, Basic, API Key, AWS SigV4)
- Declarative role-based access control
- Custom auth error handler
- Public and protected routes
- Resource-level authorization
- OpenAPI documentation with security schemes
