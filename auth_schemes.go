package fiberoapi

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// BasicAuthValidator is an optional interface for services that support
// HTTP Basic authentication. Implement this alongside AuthorizationService
// to enable Basic Auth validation.
type BasicAuthValidator interface {
	ValidateBasicAuth(username, password string) (*AuthContext, error)
}

// APIKeyValidator is an optional interface for services that support
// API Key authentication (in header, query, or cookie).
type APIKeyValidator interface {
	ValidateAPIKey(key string, location string, paramName string) (*AuthContext, error)
}

// AWSSignatureValidator is an optional interface for services that support
// AWS Signature V4 authentication. The library parses the Authorization header
// and passes structured data; the implementation handles the actual
// cryptographic verification.
type AWSSignatureValidator interface {
	ValidateAWSSignature(params *AWSSignatureParams) (*AuthContext, error)
}

// AWSSignatureParams contains the parsed components of an AWS SigV4 Authorization header.
type AWSSignatureParams struct {
	// Parsed from "Credential=AKID/date/region/service/aws4_request"
	AccessKeyID string
	Date        string
	Region      string
	Service     string

	// Parsed from "SignedHeaders=host;x-amz-date;..."
	SignedHeaders []string

	// The raw signature hex string
	Signature string

	// The raw Authorization header for custom verification
	RawHeader string

	// Request metadata needed for signature verification
	Method      string
	Path        string
	QueryString string
	Headers     map[string]string
	Body        []byte
}

// validateBearerToken validates a Bearer token from the Authorization header.
func validateBearerToken(c *fiber.Ctx, authService AuthorizationService) (*AuthContext, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("authentication required: Bearer token expected")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid authorization header: Bearer prefix expected")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	return authService.ValidateToken(token)
}

// validateBasicAuth validates Basic Auth credentials from the Authorization header.
func validateBasicAuth(c *fiber.Ctx, authService AuthorizationService) (*AuthContext, error) {
	basicValidator, ok := authService.(BasicAuthValidator)
	if !ok {
		return nil, fmt.Errorf("Basic Auth scheme configured but AuthService does not implement BasicAuthValidator")
	}

	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("authentication required: Basic auth expected")
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return nil, fmt.Errorf("invalid authorization header: Basic prefix expected")
	}

	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid Basic auth encoding: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Basic auth format: expected username:password")
	}

	return basicValidator.ValidateBasicAuth(parts[0], parts[1])
}

// validateAPIKey validates an API key from header, query, or cookie.
func validateAPIKey(c *fiber.Ctx, scheme SecurityScheme, authService AuthorizationService) (*AuthContext, error) {
	apiKeyValidator, ok := authService.(APIKeyValidator)
	if !ok {
		return nil, fmt.Errorf("API Key scheme configured but AuthService does not implement APIKeyValidator")
	}

	var key string
	switch scheme.In {
	case "header":
		key = c.Get(scheme.Name)
	case "query":
		key = c.Query(scheme.Name)
	case "cookie":
		key = c.Cookies(scheme.Name)
	default:
		return nil, fmt.Errorf("unsupported API Key location: %s", scheme.In)
	}

	if key == "" {
		return nil, fmt.Errorf("API key not found in %s parameter '%s'", scheme.In, scheme.Name)
	}

	return apiKeyValidator.ValidateAPIKey(key, scheme.In, scheme.Name)
}

// validateAWSSigV4 validates an AWS Signature V4 Authorization header.
func validateAWSSigV4(c *fiber.Ctx, authService AuthorizationService) (*AuthContext, error) {
	awsValidator, ok := authService.(AWSSignatureValidator)
	if !ok {
		return nil, fmt.Errorf("AWS SigV4 scheme configured but AuthService does not implement AWSSignatureValidator")
	}

	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("authentication required: AWS4-HMAC-SHA256 signature expected")
	}

	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256 ") {
		return nil, fmt.Errorf("invalid authorization header: AWS4-HMAC-SHA256 prefix expected")
	}

	params, err := parseAWSSigV4Header(authHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AWS SigV4 header: %w", err)
	}

	// Populate request metadata
	params.Method = c.Method()
	params.Path = c.Path()
	params.QueryString = string(c.Request().URI().QueryString())
	params.Body = c.Body()
	params.RawHeader = authHeader

	// Collect all headers that were signed
	params.Headers = make(map[string]string)
	for _, headerName := range params.SignedHeaders {
		params.Headers[headerName] = c.Get(headerName)
	}

	return awsValidator.ValidateAWSSignature(params)
}

// parseAWSSigV4Header parses an AWS SigV4 Authorization header into its components.
// Format: AWS4-HMAC-SHA256 Credential=AKID/20250101/us-east-1/s3/aws4_request,
//
//	SignedHeaders=host;x-amz-date, Signature=abcdef...
func parseAWSSigV4Header(header string) (*AWSSignatureParams, error) {
	params := &AWSSignatureParams{}
	content := strings.TrimPrefix(header, "AWS4-HMAC-SHA256 ")

	parts := strings.Split(content, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "Credential":
			credParts := strings.Split(kv[1], "/")
			if len(credParts) >= 5 {
				params.AccessKeyID = credParts[0]
				params.Date = credParts[1]
				params.Region = credParts[2]
				params.Service = credParts[3]
			}
		case "SignedHeaders":
			params.SignedHeaders = strings.Split(kv[1], ";")
		case "Signature":
			params.Signature = kv[1]
		}
	}

	if params.AccessKeyID == "" || params.Signature == "" {
		return nil, fmt.Errorf("incomplete AWS SigV4 header: missing Credential or Signature")
	}

	return params, nil
}

// validateWithScheme dispatches validation to the appropriate scheme handler.
func validateWithScheme(c *fiber.Ctx, scheme SecurityScheme, authService AuthorizationService) (*AuthContext, error) {
	switch {
	case scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "bearer"):
		return validateBearerToken(c, authService)
	case scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "basic"):
		return validateBasicAuth(c, authService)
	case scheme.Type == "apiKey":
		return validateAPIKey(c, scheme, authService)
	case scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "aws4-hmac-sha256"):
		return validateAWSSigV4(c, authService)
	default:
		return nil, fmt.Errorf("unsupported security scheme: type=%s scheme=%s", scheme.Type, scheme.Scheme)
	}
}

// AuthError represents an authentication or authorization failure with an HTTP status code.
type AuthError struct {
	StatusCode int
	Message    string
}

func (e *AuthError) Error() string {
	return e.Message
}

// ScopeError represents an authorization failure due to missing scopes (403, not 401).
type ScopeError struct {
	Scope string
}

func (e *ScopeError) Error() string {
	return fmt.Sprintf("missing required scope: %s", e.Scope)
}

// validateSecurityRequirement validates a single OpenAPI security requirement.
// A requirement is a map of scheme-name -> required-scopes.
// ALL schemes in a requirement must validate (AND semantics).
func validateSecurityRequirement(c *fiber.Ctx, requirement map[string][]string, schemes map[string]SecurityScheme, authService AuthorizationService) (*AuthContext, error) {
	if len(requirement) == 0 {
		return nil, fmt.Errorf("empty security requirement")
	}

	var lastAuthCtx *AuthContext

	for schemeName, requiredScopes := range requirement {
		scheme, exists := schemes[schemeName]
		if !exists {
			return nil, fmt.Errorf("unknown security scheme: %s", schemeName)
		}

		authCtx, err := validateWithScheme(c, scheme, authService)
		if err != nil {
			return nil, err
		}

		// Check required scopes
		for _, scope := range requiredScopes {
			if !authService.HasScope(authCtx, scope) {
				return nil, &ScopeError{Scope: scope}
			}
		}

		lastAuthCtx = authCtx
	}

	return lastAuthCtx, nil
}

// buildDefaultFromSchemes generates security requirements from configured schemes.
// Each scheme becomes a separate alternative (OR semantics).
// Schemes are sorted by name for deterministic ordering.
func buildDefaultFromSchemes(schemes map[string]SecurityScheme) []map[string][]string {
	names := make([]string, 0, len(schemes))
	for name := range schemes {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]map[string][]string, 0, len(names))
	for _, name := range names {
		result = append(result, map[string][]string{name: {}})
	}
	return result
}
