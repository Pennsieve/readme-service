package handler

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

const applicationJson = "application/json"

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// ReadmeServiceHandler is the Lambda entry point. Two routes are supported:
//
//	GET /docs/{slug}            — fetch a guide by slug (readme.io v1)
//	GET /search?query=&limit=   — search guides only (readme.io v2)
//
// Path matching tolerates BOTH the new prefix-stripped form (`/docs/...`,
// `/search`) and the legacy unstripped form (`/readme/docs/...`,
// `/readme/search`) — see comment on documentPathRegex. The new form is
// what arrives once this Lambda is fronted by its own dedicated API
// Gateway with api_mapping_key = "readme" (see terraform/gateway.tf).
// The legacy form is what arrives via pennsieve-go-api's gateway while
// that repo still defines the route. Both can be deployed simultaneously
// during the migration window; the cleanup in pennsieve-go-api removes
// the legacy path once the dedicated gateway is live.
func ReadmeServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.APIGatewayV2HTTPResponse, error) {
	logger = logger.With("requestID", request.RequestContext.RequestID)
	apiKey, errorResp := readmeApiKey()
	if errorResp != nil {
		return errorResp, nil
	}
	logger.Debug("request parameters",
		"routeKey", request.RouteKey,
		"pathParameters", request.PathParameters,
		"rawPath", request.RawPath,
		"requestContext.routeKey", request.RequestContext.RouteKey,
		"requestContext.http.path", request.RequestContext.HTTP.Path)
	path := request.RequestContext.HTTP.Path

	switch {
	case documentPathRegex.MatchString(path):
		return handleDocumentRequest(ctx, request, apiKey)
	case searchPathRegex.MatchString(path):
		return handleSearchRequest(ctx, request, apiKey)
	default:
		return NewReadmeErrorResponse(http.StatusNotFound, "resource not found: %s", path).AsAPIGatewayV2HTTPResponse(), nil
	}
}

// handleDocumentRequest fronts readme.io v1 docs-by-slug.
func handleDocumentRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest, apiKey string) (*events.APIGatewayV2HTTPResponse, error) {
	method := request.RequestContext.HTTP.Method
	if method != http.MethodGet {
		return NewReadmeErrorResponse(http.StatusMethodNotAllowed,
			"unsupported method for path %s: %s",
			request.RequestContext.HTTP.Path,
			method).AsAPIGatewayV2HTTPResponse(), nil
	}
	slug, ok := request.PathParameters["slug"]
	if !ok || len(slug) == 0 {
		return NewReadmeErrorResponse(http.StatusBadRequest, "no slug requested").AsAPIGatewayV2HTTPResponse(), nil
	}
	logger = logger.With("slug", slug)
	readmeResponse := GetDocument(ctx, apiKey, slug)
	return readmeResponse.AsAPIGatewayV2HTTPResponse(), nil
}

// handleSearchRequest fronts readme.io v2 guide-only search. `query` is
// required; `limit` is optional and clamped to [1, 25] with default 5.
func handleSearchRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest, apiKey string) (*events.APIGatewayV2HTTPResponse, error) {
	method := request.RequestContext.HTTP.Method
	if method != http.MethodGet {
		return NewReadmeErrorResponse(http.StatusMethodNotAllowed,
			"unsupported method for path %s: %s",
			request.RequestContext.HTTP.Path,
			method).AsAPIGatewayV2HTTPResponse(), nil
	}
	query := request.QueryStringParameters["query"]
	if query == "" {
		return NewReadmeErrorResponse(http.StatusBadRequest, "missing required query parameter: query").AsAPIGatewayV2HTTPResponse(), nil
	}
	limit := defaultSearchLimit
	if raw := request.QueryStringParameters["limit"]; raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	logger = logger.With("query", query, "limit", limit)
	readmeResponse := SearchGuides(ctx, apiKey, query, limit)
	return readmeResponse.AsAPIGatewayV2HTTPResponse(), nil
}

func readmeApiKey() (string, *events.APIGatewayV2HTTPResponse) {
	envKey := "README_API_KEY"
	if apiKey, ok := os.LookupEnv(envKey); !ok {
		return "", NewReadmeErrorResponse(http.StatusInternalServerError, "environment variable %s is not set", envKey).AsAPIGatewayV2HTTPResponse()
	} else if len(apiKey) == 0 {
		return "", NewReadmeErrorResponse(http.StatusInternalServerError, "environment variable %s is empty", envKey).AsAPIGatewayV2HTTPResponse()
	} else if apiKey == "dummy" {
		return "", NewReadmeErrorResponse(http.StatusInternalServerError, "environment variable %s is still set to it's initial temporary value", envKey).AsAPIGatewayV2HTTPResponse()
	} else {
		return apiKey, nil
	}
}

func response(body string, statusCode int) *events.APIGatewayV2HTTPResponse {
	resp := events.APIGatewayV2HTTPResponse{Body: body, StatusCode: statusCode, Headers: map[string]string{}}
	resp.Headers[http.CanonicalHeaderKey("content-type")] = applicationJson
	return &resp
}
